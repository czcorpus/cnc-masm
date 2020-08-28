// Copyright 2020 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2020 Institute of the Czech National Corpus,
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

package jobs

import (
	"log"
	"masm/api"
	"masm/cnf"
	"masm/fsops"
	"net/http"
	"sort"
	"strconv"

	"github.com/gorilla/mux"
)

const (
	tableActionUpdateJob    = 0
	tableActionClearOldJobs = 1
)

func (a *Actions) createJobList() JobInfoList {
	ans := make(JobInfoList, 0, len(a.syncJobs))
	for _, v := range a.syncJobs {
		ans = append(ans, v)
	}
	return ans
}

// TableUpdate is a job table queue element specifying
// required operation on the table
type TableUpdate struct {
	action int
	data   GeneralJobInfo
}

// Actions contains async job-related actions
type Actions struct {
	conf     *cnf.Conf
	version  cnf.VersionInfo
	syncJobs map[string]GeneralJobInfo

	// tableUpdate is the only way syncJobs are actually
	// updated
	tableUpdate chan TableUpdate
}

// ClearOldJobs clears the job table by removing
// too old jobs
func (a *Actions) ClearOldJobs() {
	a.tableUpdate <- TableUpdate{
		action: tableActionClearOldJobs,
	}
}

// GetUnfinishedJob returns a first matching unfinished job
// for the same corpus and job type
func (a *Actions) GetUnfinishedJob(corpusID, jobType string) GeneralJobInfo {
	return FindUnfinishedJobOfType(a.syncJobs, corpusID, jobType)
}

// AddJobInfo add a new job to the job table and provides
// a channel to update its status
func (a *Actions) AddJobInfo(j GeneralJobInfo) chan GeneralJobInfo {
	a.syncJobs[j.GetID()] = j
	syncUpdates := make(chan GeneralJobInfo, 10)
	go func() {
		for item := range syncUpdates {
			a.tableUpdate <- TableUpdate{
				action: tableActionUpdateJob,
				data:   item,
			}
		}
		a.syncJobs[j.GetID()].SetFinished()
	}()
	return syncUpdates
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
			item := v.CompactVersion()
			ans = append(ans, &item)
		}
		sort.Sort(sort.Reverse(ans))
		api.WriteJSONResponse(w, ans)

	} else {
		ans := a.createJobList()
		sort.Sort(sort.Reverse(ans))
		api.WriteJSONResponse(w, ans)
	}
}

// SyncJobInfo gives an information about a specific data sync job
func (a *Actions) SyncJobInfo(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	job := FindJob(a.syncJobs, vars["jobId"])
	if job != nil {
		api.WriteJSONResponse(w, job)

	} else {
		api.WriteJSONErrorResponse(w, api.NewActionError("Synchronization job not found"), http.StatusNotFound)
	}
}

func (a *Actions) OnExit() {
	if fsops.IsFile(a.conf.StatusDataPath) {
		log.Printf("INFO: saving state to %s", a.conf.StatusDataPath)
		jobList := a.createJobList()
		err := jobList.Serialize(a.conf.StatusDataPath)
		if err != nil {
			log.Print("ERROR: ", err)
		}

	} else {
		log.Print("WARNING: no status file specified, discarding job list")
	}
}

// NewActions is the default factory
func NewActions(conf *cnf.Conf, version cnf.VersionInfo) *Actions {
	ans := &Actions{
		conf:        conf,
		version:     version,
		syncJobs:    make(map[string]GeneralJobInfo),
		tableUpdate: make(chan TableUpdate),
	}
	if fsops.IsFile(conf.StatusDataPath) {
		log.Printf("INFO: found status data in %s - loading...", conf.StatusDataPath)
		jobs, err := LoadJobList(conf.StatusDataPath)
		if err != nil {
			log.Print("ERROR: failed to load status data - ", err)
		}
		log.Printf("INFO: loaded %d job(s)", len(jobs))
		for _, job := range jobs {
			ans.syncJobs[job.GetID()] = job
		}
	}
	go func() {
		for upd := range ans.tableUpdate {
			switch upd.action {
			case tableActionUpdateJob:
				ans.syncJobs[upd.data.GetID()] = upd.data
			case tableActionClearOldJobs:
				clearOldJobs(ans.syncJobs)
			}

		}
	}()
	return ans
}
