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
	"fmt"
	"masm/v3/api"
	"masm/v3/fsops"
	"masm/v3/mail"
	"net/http"
	"os"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/gorilla/mux"
)

const (
	tableActionUpdateJob = iota
	tableActionFinishJob
	tableActionClearOldJobs
)

// TableUpdate is a job table queue element specifying
// required operation on the table
type TableUpdate struct {
	action int
	itemID string
	data   GeneralJobInfo
}

// Actions contains async job-related actions
type Actions struct {
	conf             *Conf
	jobList          map[string]GeneralJobInfo
	jobListLock      sync.Mutex
	detachedJobs     map[string]GeneralJobInfo
	detachedJobsLock sync.Mutex
	jobStop          chan<- string

	// tableUpdate is the only way jobList is actually
	// updated
	tableUpdate chan TableUpdate

	notificationRecipients map[string][]string
}

func (a *Actions) createJobList(unfinishedOnly bool) JobInfoList {
	ans := make(JobInfoList, 0, len(a.jobList))
	for _, v := range a.jobList {
		if !unfinishedOnly || !v.IsFinished() {
			ans = append(ans, v)
		}
	}
	return ans
}

// AddJobInfo add a new job to the job table and provides
// a channel to update its status
func (a *Actions) AddJobInfo(j GeneralJobInfo) chan GeneralJobInfo {
	_, ok := a.detachedJobs[j.GetID()]
	if ok {
		log.Info().Msgf("Registering again detached job %s", j.GetID())
		a.detachedJobsLock.Lock()
		delete(a.detachedJobs, j.GetID())
		a.detachedJobsLock.Unlock()
	}
	a.jobListLock.Lock()
	a.jobList[j.GetID()] = j
	a.jobListLock.Unlock()
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

// JobList returns a list of corpus data synchronization jobs
// (i.e. syncing between /cnk/run/manatee/data and /cnk/local/ssd/run/manatee/data)
func (a *Actions) JobList(w http.ResponseWriter, req *http.Request) {
	unOnly := req.URL.Query().Get("unfinishedOnly") == "1"
	if req.URL.Query().Get("compact") == "1" {
		ans := make(JobInfoListCompact, 0, len(a.jobList))
		for _, v := range a.jobList {
			if !unOnly || !v.IsFinished() {
				item := v.CompactVersion()
				ans = append(ans, &item)
			}
		}
		sort.Sort(sort.Reverse(ans))
		api.WriteJSONResponse(w, ans)

	} else {
		tmp := a.createJobList(unOnly)
		sort.Sort(sort.Reverse(tmp))
		ans := make([]any, len(tmp))
		for i, item := range tmp {
			ans[i] = item.FullInfo()
		}
		api.WriteJSONResponse(w, ans)
	}
}

// JobInfo gives an information about a specific data sync job
func (a *Actions) JobInfo(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	job := FindJob(a.jobList, vars["jobId"])
	if job != nil {
		api.WriteJSONResponse(w, job)

	} else {
		api.WriteJSONErrorResponse(w, api.NewActionError("job not found"), http.StatusNotFound)
	}
}

func (a *Actions) Delete(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	job := FindJob(a.jobList, vars["jobId"])
	if job != nil {
		a.jobStop <- job.GetID()
		api.WriteJSONResponse(w, job)

	} else {
		api.WriteJSONErrorResponse(w, api.NewActionError("job not found"), http.StatusNotFound)
	}
}

func (a *Actions) ClearIfFinished(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	job, removed := ClearFinishedJob(a.jobList, vars["jobId"])
	if job != nil {
		api.WriteJSONResponse(w, map[string]any{"removed": removed, "jobInfo": job})

	} else {
		api.WriteJSONErrorResponse(w, api.NewActionError("job does not exist or did not finish yet"), http.StatusNotFound)
	}
}

func (a *Actions) OnExit() {
	if a.conf.StatusDataPath != "" {
		log.Info().Msgf("saving state to %s", a.conf.StatusDataPath)
		jobList := a.createJobList(true)
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
	a.detachedJobsLock.Lock()
	defer a.detachedJobsLock.Unlock()
	_, ok := a.detachedJobs[jobID]
	delete(a.detachedJobs, jobID)
	return ok
}

func (a *Actions) LastUnfinishedJobOfType(corpusID string, jobType string) (GeneralJobInfo, bool) {
	var tmp GeneralJobInfo
	for _, v := range a.jobList {
		if v.GetCorpus() == corpusID && v.GetType() == jobType && !v.IsFinished() &&
			(tmp == nil || reflect.ValueOf(tmp).IsNil() || v.GetStartDT().Before(tmp.GetStartDT())) {
			tmp = v
		}
	}
	return tmp, tmp != nil && !reflect.ValueOf(tmp).IsNil()
}

func (a *Actions) GetJob(jobID string) (GeneralJobInfo, bool) {
	v, ok := a.jobList[jobID]
	return v, ok
}

func (a *Actions) AddNotification(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	jobID := vars["jobId"]
	job := FindJob(a.jobList, jobID)
	if job != nil {
		recipients, ok := a.notificationRecipients[jobID]
		if !ok {
			recipients = make([]string, 1)
			recipients[0] = vars["address"]
		} else {
			hasValue := false
			for _, addr := range recipients {
				if addr == vars["address"] {
					hasValue = true
				}
			}
			if !hasValue {
				recipients = append(recipients, vars["address"])
			}
		}
		a.notificationRecipients[jobID] = recipients
		resp := struct {
			Registered bool `json:"registered"`
		}{
			Registered: true,
		}
		api.WriteJSONResponse(w, resp)

	} else {
		api.WriteJSONErrorResponse(w, api.NewActionError("job not found"), http.StatusNotFound)
	}
}

func (a *Actions) GetNotifications(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	jobID := vars["jobId"]
	job := FindJob(a.jobList, jobID)
	if job != nil {
		recipients, ok := a.notificationRecipients[job.GetID()]
		resp := struct {
			Recipients []string `json:"recipients"`
		}{
			Recipients: []string{},
		}
		if ok {
			resp.Recipients = recipients
		}
		api.WriteJSONResponse(w, resp)

	} else {
		api.WriteJSONErrorResponse(w, api.NewActionError("job not found"), http.StatusNotFound)
	}
}

func (a *Actions) CheckNotification(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	jobID := vars["jobId"]
	job := FindJob(a.jobList, jobID)
	if job != nil {
		registered := false
		recipients, ok := a.notificationRecipients[jobID]
		if ok {
			for _, addr := range recipients {
				if addr == vars["address"] {
					registered = true
					break
				}
			}
		}

		resp := struct {
			Registered bool `json:"registered"`
		}{
			Registered: registered,
		}

		if registered {
			api.WriteJSONResponse(w, resp)
		} else {
			api.WriteJSONResponseWithStatus(w, http.StatusNotFound, resp)
		}

	} else {
		api.WriteJSONErrorResponse(w, api.NewActionError("job not found"), http.StatusNotFound)
	}
}

func (a *Actions) RemoveNotification(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	jobID := vars["jobId"]
	job := FindJob(a.jobList, jobID)
	if job != nil {
		recipients, ok := a.notificationRecipients[jobID]
		if ok {
			for i, addr := range recipients {
				if addr == vars["address"] {
					recipients = append(recipients[:i], recipients[i+1:]...)
					break
				}
			}
			a.notificationRecipients[jobID] = recipients
		}

		resp := struct {
			Registered bool `json:"registered"`
		}{
			Registered: false,
		}
		api.WriteJSONResponse(w, resp)

	} else {
		api.WriteJSONErrorResponse(w, api.NewActionError("job not found"), http.StatusNotFound)
	}
}

// NewActions is the default factory
func NewActions(
	conf *Conf,
	exitEvent <-chan os.Signal,
	jobStop chan<- string,
) *Actions {
	ans := &Actions{
		conf:                   conf,
		jobList:                make(map[string]GeneralJobInfo),
		detachedJobs:           make(map[string]GeneralJobInfo),
		tableUpdate:            make(chan TableUpdate),
		jobStop:                jobStop,
		notificationRecipients: make(map[string][]string),
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
				ans.jobListLock.Lock()
				currErr := ans.jobList[upd.itemID].GetError()
				// make sure we keep the current error even if new status
				// comes without one
				if currErr != nil && upd.data.GetError() == nil {
					ans.jobList[upd.itemID] = upd.data.CloneWithError(currErr)

				} else {
					ans.jobList[upd.itemID] = upd.data
				}
				ans.jobListLock.Unlock()
			case tableActionFinishJob:
				ans.jobListLock.Lock()
				ans.jobList[upd.itemID].SetFinished()
				ans.jobListLock.Unlock()
			case tableActionClearOldJobs:
				ans.jobListLock.Lock()
				clearOldJobs(ans.jobList)
				ans.jobListLock.Unlock()
			}

		}
	}()

	go func() {
		for upd := range ans.tableUpdate {
			switch upd.action {
			case tableActionFinishJob:
				recipients, ok := ans.notificationRecipients[upd.itemID]
				if ok {
					message := fmt.Sprintf("Job `%s` finished", upd.itemID)
					mail.SendNotification(&conf.EmailNotification, recipients, message, message)
				}
			}
		}
	}()

	return ans
}
