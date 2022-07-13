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
	"masm/v3/api"
	"masm/v3/fsops"
	"net/http"
	"os"
	"reflect"
	"sort"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/gorilla/mux"
)

const (
	tableActionUpdateJob = iota
	tableActionFinishJob
	tableActionClearOldJobs
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
	itemID string
	data   GeneralJobInfo
}

// Actions contains async job-related actions
type Actions struct {
	conf         *Conf
	syncJobs     map[string]GeneralJobInfo
	detachedJobs map[string]GeneralJobInfo
	jobStop      chan<- string

	// tableUpdate is the only way syncJobs are actually
	// updated
	tableUpdate chan TableUpdate
}

// AddJobInfo add a new job to the job table and provides
// a channel to update its status
func (a *Actions) AddJobInfo(j GeneralJobInfo) chan GeneralJobInfo {
	_, ok := a.detachedJobs[j.GetID()]
	if ok {
		log.Info().Msgf("Registering again detached job %s", j.GetID())
		delete(a.detachedJobs, j.GetID())
	}
	a.syncJobs[j.GetID()] = j
	syncUpdates := make(chan GeneralJobInfo, 10)
	go func() {
		for item := range syncUpdates {
			a.tableUpdate <- TableUpdate{
				action: tableActionUpdateJob,
				itemID: j.GetID(),
				data:   item,
			}
		}
		a.tableUpdate <- TableUpdate{
			action: tableActionFinishJob,
			itemID: j.GetID(),
		}
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

// JobInfo gives an information about a specific data sync job
func (a *Actions) JobInfo(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	job := FindJob(a.syncJobs, vars["jobId"])
	if job != nil {
		api.WriteJSONResponse(w, job)

	} else {
		api.WriteJSONErrorResponse(w, api.NewActionError("job not found"), http.StatusNotFound)
	}
}

func (a *Actions) Delete(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	job := FindJob(a.syncJobs, vars["jobId"])
	if job != nil {
		a.jobStop <- job.GetID()
		api.WriteJSONResponse(w, job)

	} else {
		api.WriteJSONErrorResponse(w, api.NewActionError("job not found"), http.StatusNotFound)
	}
}

func (a *Actions) OnExit() {
	if a.conf.StatusDataPath != "" {
		log.Info().Msgf("saving state to %s", a.conf.StatusDataPath)
		jobList := a.createJobList()
		err := jobList.Serialize(a.conf.StatusDataPath)
		if err != nil {
			log.Error().Err(err)
		}

	} else {
		log.Warn().Msg("no status file specified, discarding job list")
	}
}

func (a *Actions) GetDetachedJobs() []GeneralJobInfo {
	ans := make([]GeneralJobInfo, len(a.detachedJobs))
	i := 0
	for _, v := range a.detachedJobs {
		ans[i] = v
		i++
	}
	return ans
}

func (a *Actions) ClearDetachedJob(jobID string) bool {
	_, ok := a.detachedJobs[jobID]
	delete(a.detachedJobs, jobID)
	return ok
}

func (a *Actions) LastUnfinishedJobOfType(corpusID string, jobType string) (GeneralJobInfo, bool) {
	var tmp GeneralJobInfo
	for _, v := range a.syncJobs {
		if v.GetCorpus() == corpusID && v.GetType() == jobType && !v.IsFinished() &&
			(v == nil || reflect.ValueOf(v).IsNil() || v.GetStartDT().Before(tmp.GetStartDT())) {
			tmp = v
		}
	}
	return tmp, tmp != nil && !reflect.ValueOf(tmp).IsNil()
}

func (a *Actions) GetJob(jobID string) (GeneralJobInfo, bool) {
	v, ok := a.syncJobs[jobID]
	return v, ok
}

// NewActions is the default factory
func NewActions(
	conf *Conf,
	exitEvent <-chan os.Signal,
	jobStop chan<- string,
) *Actions {
	ans := &Actions{
		conf:         conf,
		syncJobs:     make(map[string]GeneralJobInfo),
		detachedJobs: make(map[string]GeneralJobInfo),
		tableUpdate:  make(chan TableUpdate),
		jobStop:      jobStop,
	}
	if fsops.IsFile(conf.StatusDataPath) {
		log.Info().Msgf("found status data in %s - loading...", conf.StatusDataPath)
		jobs, err := LoadJobList(conf.StatusDataPath)
		if err != nil {
			log.Error().Err(err).Msg("failed to load status data")
		}
		for _, job := range jobs {
			if job != nil {
				ans.detachedJobs[job.GetID()] = job
				log.Info().Msgf("added detached job %s", job.GetID())
			}
		}
	}

	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		for {
			select {
			case <-ticker.C:
				ans.tableUpdate <- TableUpdate{
					action: tableActionClearOldJobs,
				}
			case <-exitEvent:
				ticker.Stop()
				return
			}
		}
	}()

	go func() {
		for upd := range ans.tableUpdate {
			switch upd.action {
			case tableActionUpdateJob:
				ans.syncJobs[upd.itemID] = upd.data
			case tableActionFinishJob:
				ans.syncJobs[upd.itemID].SetFinished()
			case tableActionClearOldJobs:
				clearOldJobs(ans.syncJobs)
			}

		}
	}()
	return ans
}
