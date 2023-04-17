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
	"net/http"
	"os"
	"reflect"
	"sort"
	"sync"
	"time"

	cncmail "github.com/czcorpus/cnc-gokit/mail"
	"github.com/rs/zerolog/log"
	"golang.org/x/text/message"

	"github.com/czcorpus/cnc-gokit/fs"
	"github.com/czcorpus/cnc-gokit/uniresp"
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
	jobQueue         *JobQueue
	jobQueueLock     sync.Mutex
	jobDeps          JobsDeps
	jobStop          chan<- string
	msgPrinter       *message.Printer

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

func (a *Actions) EnqueueJob(fn *QueuedFunc, initialStatus GeneralJobInfo) {
	a.jobQueueLock.Lock()
	a.jobQueue.Enqueue(fn, initialStatus)
	a.jobQueueLock.Unlock()
	log.Info().Msgf("Enqueued job %s", initialStatus.GetID())
}

func (a *Actions) EqueueJobAfter(fn *QueuedFunc, initialStatus GeneralJobInfo, parentJobID string) {
	a.jobQueueLock.Lock()
	a.jobQueue.Enqueue(fn, initialStatus)
	a.jobQueueLock.Unlock()
	a.jobDeps.Add(initialStatus.GetID(), parentJobID)
	log.Info().Msgf("Enqueued job %s with parent %s", initialStatus.GetID(), parentJobID)
}

func (a *Actions) dequeueAndRunJob() {
	fn, initState, err := a.jobQueue.Dequeue()
	if err == nil {
		log.Info().
			Float32(
				"utilization",
				float32(a.numOfUnfinishedJobs())/float32(a.conf.MaxNumConcurrentJobs),
			).
			Str("jobId", initState.GetID()).
			Str("jobType", initState.GetType()).
			Str("corpus", initState.GetCorpus()).
			Msgf("Dequeued a new job")
		updateJobChan := a.addJobInfo(initState)
		go func() {
			(*fn)(updateJobChan)
		}()
	}
}

// dequeueJobAsFailed can be used in case we know we cannot
// run a job e.g. because of a failed dependency (= other job).
// But we still need to respect basic workflow so we dequeue
// the job, set the status and send it via a respective channel.
func (a *Actions) dequeueJobAsFailed(err error) {
	_, initState, _ := a.jobQueue.Dequeue()
	finalState := initState.WithError(err)
	updateJobChan := a.addJobInfo(finalState)
	updateJobChan <- finalState.AsFinished()
	log.Error().Err(err).Send()
}

// addJobInfo add a new job to the job table and provides
// a channel to update its status
func (a *Actions) addJobInfo(j GeneralJobInfo) chan GeneralJobInfo {
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
		var item GeneralJobInfo
		for item = range syncUpdates {
			a.tableUpdate <- TableUpdate{
				action: tableActionUpdateJob,
				itemID: j.GetID(),
				data:   item,
			}
		}
		a.tableUpdate <- TableUpdate{
			action: tableActionFinishJob,
			itemID: j.GetID(),
			data:   item,
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
		uniresp.WriteJSONResponse(w, ans)

	} else {
		tmp := a.createJobList(unOnly)
		sort.Sort(sort.Reverse(tmp))
		ans := make([]any, len(tmp))
		for i, item := range tmp {
			ans[i] = item.FullInfo()
		}
		uniresp.WriteJSONResponse(w, ans)
	}
}

// JobInfo gives an information about a specific data sync job
func (a *Actions) JobInfo(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	job := FindJob(a.jobList, vars["jobId"])
	if job != nil {
		if req.URL.Query().Get("compact") == "1" {
			uniresp.WriteJSONResponse(w, job.CompactVersion())

		} else {
			uniresp.WriteJSONResponse(w, job.FullInfo())
		}

	} else {
		uniresp.WriteJSONErrorResponse(w, uniresp.NewActionError("job not found"), http.StatusNotFound)
	}
}

func (a *Actions) Delete(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	job := FindJob(a.jobList, vars["jobId"])
	if job != nil {
		a.jobStop <- job.GetID()
		uniresp.WriteJSONResponse(w, job)

	} else {
		uniresp.WriteJSONErrorResponse(w, uniresp.NewActionError("job not found"), http.StatusNotFound)
	}
}

func (a *Actions) ClearIfFinished(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	job, removed := ClearFinishedJob(a.jobList, vars["jobId"])
	if job != nil {
		uniresp.WriteJSONResponse(w, map[string]any{"removed": removed, "jobInfo": job})

	} else {
		uniresp.WriteJSONErrorResponse(w, uniresp.NewActionError("job does not exist or did not finish yet"), http.StatusNotFound)
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

func (a *Actions) numOfUnfinishedJobs() int {
	ans := 0
	a.jobListLock.Lock()
	for _, v := range a.jobList {
		if !v.IsFinished() {
			ans++
		}
	}
	a.jobListLock.Unlock()
	return ans
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
		uniresp.WriteJSONResponse(w, resp)

	} else {
		uniresp.WriteJSONErrorResponse(w, uniresp.NewActionError("job not found"), http.StatusNotFound)
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
		uniresp.WriteJSONResponse(w, resp)

	} else {
		uniresp.WriteJSONErrorResponse(w, uniresp.NewActionError("job not found"), http.StatusNotFound)
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
			uniresp.WriteJSONResponse(w, resp)
		} else {
			uniresp.WriteJSONResponseWithStatus(w, http.StatusNotFound, resp)
		}

	} else {
		uniresp.WriteJSONErrorResponse(w, uniresp.NewActionError("job not found"), http.StatusNotFound)
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
		uniresp.WriteJSONResponse(w, resp)

	} else {
		uniresp.WriteJSONErrorResponse(w, uniresp.NewActionError("job not found"), http.StatusNotFound)
	}
}

func (a *Actions) Utilization(w http.ResponseWriter, req *http.Request) {
	numUnfinished := a.numOfUnfinishedJobs()
	ans := map[string]any{
		"maxNumConcurrentJobs": a.conf.MaxNumConcurrentJobs,
		"currentRunningJobs":   numUnfinished,
		"utilization":          float32(numUnfinished) / float32(a.conf.MaxNumConcurrentJobs),
		"jobQueueLength":       a.jobQueue.Size(),
	}
	uniresp.WriteJSONResponse(w, ans)
}

// NewActions is the default factory
func NewActions(
	conf *Conf,
	lang string,
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
		msgPrinter:             message.NewPrinter(message.MatchLanguage(lang)),
		jobQueue:               &JobQueue{},
		jobDeps:                make(JobsDeps),
	}
	isFile, _ := fs.IsFile(conf.StatusDataPath)
	if isFile {
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

	// here we listen for exit events and clean finished
	// jobs info regularly
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

	ticker2 := time.NewTicker(1 * time.Second)
	go func() {
		for {
			select {
			case <-ticker2.C:
				ans.jobQueueLock.Lock()
				numUnfinished := ans.numOfUnfinishedJobs()
				// Now calling again the numOfUnfinishedJobs() may return
				// different value but it can be only a value smaller than
				// numUnfinished as the change can be only caused by another
				// job being finished (adding of jobs for execution happens
				// only here and is not concurrent).
				if ans.conf.MaxNumConcurrentJobs > numUnfinished {
					// first, let's check whether the current job depends
					// on other job(s) (= aka 'parents') and delay it in case
					// parents are not ready yet
					nextJobID, err := ans.jobQueue.PeekID()
					if err != nil {
						// empty queue
					} else if _, ok := ans.jobDeps[nextJobID]; ok { // job with dependencies

						mustWait, err := ans.jobDeps.MustWait(nextJobID)
						if err != nil {
							err := fmt.Errorf("failed to obtain waiting status for job %s: %w", nextJobID, err)
							ans.dequeueJobAsFailed(err)

						} else if mustWait {
							ans.jobQueue.DelayNext()

						} else {
							hasFailedParent, err := ans.jobDeps.HasFailedParent(nextJobID)
							if err != nil {
								err := fmt.Errorf("failed to check parents of job %s: %w", nextJobID, err)
								ans.dequeueJobAsFailed(err)

							} else if hasFailedParent {
								err := fmt.Errorf("failed to run job %s due to failed parent(s): %w", nextJobID, err)
								ans.dequeueJobAsFailed(err)

							} else {
								ans.dequeueAndRunJob()
							}
						}

					} else { // job without deps
						ans.dequeueAndRunJob()
					}
					ans.jobQueueLock.Unlock()
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
					ans.jobList[upd.itemID] = upd.data.WithError(currErr)

				} else {
					ans.jobList[upd.itemID] = upd.data
				}
				ans.jobListLock.Unlock()
			case tableActionFinishJob:
				ans.jobListLock.Lock()
				ans.jobList[upd.itemID] = ans.jobList[upd.itemID].AsFinished()
				ans.jobListLock.Unlock()
				ans.jobDeps.SetParentFinished(upd.itemID, upd.data.GetError() != nil)
				recipients, ok := ans.notificationRecipients[upd.itemID]
				if ok {
					jdesc := extractJobDescription(ans.msgPrinter, upd.data)
					subject := ans.msgPrinter.Sprintf("Job of type \"%s\" finished", jdesc)
					var sign string
					if conf.EmailNotification.HasSignature() {
						var err error
						sign, err = conf.EmailNotification.LocalizedSignature(lang)
						if err != nil {
							log.Error().Err(err).Send()
						}

					} else {
						sign = conf.EmailNotification.DefaultSignature(lang)
					}

					notificationConf := conf.EmailNotification.WithRecipients(recipients...)
					err := cncmail.SendNotification(
						&notificationConf,
						time.Now().Location(),
						cncmail.Notification{
							Subject: subject,
							Paragraphs: []string{
								subject,
								ans.msgPrinter.Sprintf("Job ID: %s", upd.itemID),
								localizedStatus(ans.msgPrinter, upd.data),
								"",
								"",
								sign,
							},
						},
					)
					if err != nil {
						log.Error().Err(err).
							Str("mailSubject", subject).
							Strs("mailBody", []string{subject, jdesc}).
							Msg("Failed to send finished job notification")
					}
				}
			case tableActionClearOldJobs:
				ans.jobListLock.Lock()
				clearOldJobs(ans.jobList)
				ans.jobListLock.Unlock()
			}

		}
	}()

	return ans
}
