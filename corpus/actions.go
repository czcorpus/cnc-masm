// Copyright 2019 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2019 Institute of the Czech National Corpus,
//                Faculty of Arts, Charles University
//   This file is part of CNC-MASM.
//
//  CNC-MASM is free software: you can redistribute it and/or modify
//  it under the terms of the GNU General Public License as published by
//  the Free Software Foundation, either version 3 of the License, or
//  (at your option) any later version.
//
//  CNC-MASM is distributed in the hope that it will be useful,
//  but WITHOUT ANY WARRANTY; without even the implied warranty of
//  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//  GNU General Public License for more details.
//
//  You should have received a copy of the GNU General Public License
//  along with CNC-MASM.  If not, see <https://www.gnu.org/licenses/>.

package corpus

import (
	"encoding/json"
	"log"
	"masm/api"
	"masm/cnf"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// Actions contains all the server HTTP REST actions
type Actions struct {
	conf        *cnf.Conf
	version     cnf.VersionInfo
	syncJobs    map[string]*JobInfo
	syncUpdates chan *JobInfo
}

// RootAction is just an information action about the service
func (a *Actions) RootAction(w http.ResponseWriter, req *http.Request) {
	ans := make(map[string]interface{})
	ans["message"] = "MASM - Manatee Data And KonText Service Management"
	ans["data"] = a.version
	resp, err := json.Marshal(ans)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(err), http.StatusInternalServerError)
	}
	w.Write(resp)
}

// GetCorpusInfo provides some basic information about stored data
func (a *Actions) GetCorpusInfo(w http.ResponseWriter, req *http.Request) {
	var err error
	vars := mux.Vars(req)
	corpusID := vars["corpusId"]
	subdir := vars["subdir"]
	if subdir != "" {
		corpusID = filepath.Join(subdir, corpusID)
	}
	wsattr := req.URL.Query().Get("wsattr")
	if wsattr == "" {
		wsattr = "lemma"
	}
	log.Printf("INFO: request[corpusID: %s, wsattr: %s]", corpusID, wsattr)
	ans, err := GetCorpusInfo(corpusID, wsattr, a.conf.CorporaSetup)
	switch err.(type) {
	case NotFound:
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(err), http.StatusNotFound)
		log.Printf("ERROR: %s", err)
	case InfoError:
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(err), http.StatusInternalServerError)
		log.Printf("ERROR: %s", err)
	case nil:
		api.WriteJSONResponse(w, ans)
	}
}

// SynchronizeCorpusData synchronizes data between CNC corpora data and KonText data
// for a specified corpus (the corpus must be explicitly allowed in the configuration).
func (a *Actions) SynchronizeCorpusData(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	corpusID := vars["corpusId"]
	subdir := vars["subdir"]
	if subdir != "" {
		corpusID = filepath.Join(subdir, corpusID)
	}
	if !a.conf.CorporaSetup.AllowsSyncForCorpus(corpusID) {
		api.WriteJSONErrorResponse(w, api.NewActionError("Corpus synchronization forbidden for '%s'", corpusID), http.StatusUnauthorized)
		return
	}
	jobID, err := uuid.NewUUID()
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionError("Failed to start synchronization job for '%s'", corpusID), http.StatusUnauthorized)
		return
	}

	if prevRunning := getUnfinishedJobForCorpus(a.syncJobs, corpusID); prevRunning != "" {
		api.WriteJSONErrorResponse(w, api.NewActionError("Cannot run synchronization - the previous job '%s' have not finished yet", prevRunning), http.StatusConflict)
		return
	}

	jobKey := jobID.String()
	jobRec := &JobInfo{
		ID:       jobKey,
		CorpusID: corpusID,
		Start:    time.Now().Format(time.RFC3339),
		Finish:   "",
	}
	clearOldJobs(a.syncJobs)
	a.syncJobs[jobKey] = jobRec

	go func(jobRec JobInfo) {
		resp, err := synchronizeCorpusData(&a.conf.CorporaSetup.CorpusDataPath, corpusID)
		if err != nil {
			jobRec.Error = err.Error()
		}
		jobRec.Result = &resp
		jobRec.Finish = time.Now().Format(time.RFC3339)
		a.syncUpdates <- &jobRec
	}(*jobRec)

	api.WriteJSONResponse(w, a.syncJobs[jobKey])
}

// SyncJobsList returns a list of corpus data synchronization jobs
// (i.e. syncing between /cnk/run/manatee/data and /cnk/local/ssd/run/manatee/data)
func (a *Actions) SyncJobsList(w http.ResponseWriter, req *http.Request) {
	args := req.URL.Query()
	compStr, ok := args["compact"]
	if !ok {
		compStr = []string{"0"}
	}
	compInt, err := strconv.Atoi(compStr[0])
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionError("compact argument must be either 0 or 1"), http.StatusBadRequest)
		return
	}

	if compInt == 1 {
		ans := make(JobInfoListCompact, 0, len(a.syncJobs))
		for _, v := range a.syncJobs {
			item := JobInfoCompact{
				ID:       v.ID,
				CorpusID: v.CorpusID,
				Start:    v.Start,
				Finish:   v.Finish,
				OK:       true,
			}
			if v.Error != "" || (v.Result != nil && !v.Result.OK) {
				item.OK = false
			}
			ans = append(ans, &item)
		}
		sort.Sort(sort.Reverse(ans))
		api.WriteJSONResponse(w, ans)

	} else {
		ans := make(JobInfoList, 0, len(a.syncJobs))
		for _, v := range a.syncJobs {
			ans = append(ans, v)
		}
		sort.Sort(sort.Reverse(ans))
		api.WriteJSONResponse(w, ans)
	}
}

// SyncJobInfo gives an information about a specific data sync job
func (a *Actions) SyncJobInfo(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	ans, ok := a.syncJobs[vars["jobId"]]
	if ok {
		api.WriteJSONResponse(w, ans)

	} else {
		api.WriteJSONErrorResponse(w, api.NewActionError("Synchronization job not found"), http.StatusNotFound)
	}
}

// NewActions is the default factory
func NewActions(conf *cnf.Conf, version cnf.VersionInfo) *Actions {
	ans := &Actions{
		conf:        conf,
		version:     version,
		syncJobs:    make(map[string]*JobInfo),
		syncUpdates: make(chan *JobInfo),
	}
	go func() {
		for item := range ans.syncUpdates {
			ans.syncJobs[item.ID] = item
		}
	}()
	return ans
}
