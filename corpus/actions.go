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
	"fmt"
	"log"
	"masm/api"
	"masm/cnf"
	"net/http"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type JobInfo struct {
	ID       string `json:"id"`
	CorpusID string `json:"corpusId"`
	Start    string `json:"start"`
	Finish   string `json:"finish"`
	Error    error  `json:"error"`
}

func clearOldJobs(data map[string]*JobInfo) {
	curr := time.Now()
	for k, v := range data {
		t, err := time.Parse(time.RFC3339, v.Start)
		if err != nil {
			log.Print("WARNING: job datetime info malformed: ", err)
		}
		if curr.Sub(t) > time.Duration(168)*time.Hour {
			delete(data, k)
		}
	}
}

// Actions contains all the server HTTP REST actions
type Actions struct {
	conf     *cnf.Conf
	version  string
	syncJobs map[string]*JobInfo
}

func (a *Actions) RootAction(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "{\"message\": \"MASM - Manatee and KonText Middleware v%s\"}", a.version)
}

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
		_, err := synchronizeCorpusData(&a.conf.CorporaSetup.CorpusDataPath, corpusID)
		if err != nil {
			jobRec.Error = err
		}
		jobRec.Finish = time.Now().Format(time.RFC3339)
		a.syncJobs[jobRec.ID] = &jobRec
	}(*jobRec)

	api.WriteJSONResponse(w, a.syncJobs[jobKey])
}

// SyncJobsList returns a list of corpus data synchronization jobs
// (i.e. syncing between /cnk/run/manatee/data and /cnk/local/ssd/run/manatee/data)
func (a *Actions) SyncJobsList(w http.ResponseWriter, req *http.Request) {
	ans := make([]*JobInfo, 0, len(a.syncJobs))
	for _, v := range a.syncJobs {
		ans = append(ans, v)
	}
	api.WriteJSONResponse(w, ans)
}

// NewActions is the default factory
func NewActions(conf *cnf.Conf, version string) *Actions {
	return &Actions{conf: conf, version: version, syncJobs: make(map[string]*JobInfo)}
}
