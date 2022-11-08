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

package dummy

import (
	"net/http"

	"masm/v3/api"
	"masm/v3/jobs"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type storedDummyJob struct {
	jobInfo   dummyJobInfo
	jobUpdate chan jobs.GeneralJobInfo
}

// Actions contains all the server HTTP REST actions
type Actions struct {
	dummyJobs  map[string]*storedDummyJob
	jobActions *jobs.Actions
}

// GetCorpusInfo provides some basic information about stored data
func (a *Actions) CreateDummyJob(w http.ResponseWriter, req *http.Request) {
	jobID, err := uuid.NewUUID()
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionError("Failed to create dummy"), http.StatusUnauthorized)
		return
	}

	jobInfo := dummyJobInfo{
		ID:       jobID.String(),
		Type:     "dummy-job",
		Start:    jobs.CurrentDatetime(),
		CorpusID: "dummy",
	}
	jobChannel := a.jobActions.AddJobInfo(&jobInfo)
	a.dummyJobs[jobID.String()] = &storedDummyJob{
		jobInfo:   jobInfo,
		jobUpdate: jobChannel,
	}
	api.WriteJSONResponse(w, jobInfo)
}

func (a *Actions) FinishDummyJob(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	storedJob, ok := a.dummyJobs[vars["jobId"]]
	if ok {
		delete(a.dummyJobs, vars["jobId"])
		defer close(storedJob.jobUpdate)

		storedJob.jobInfo.SetFinished()
		storedJob.jobInfo.Result = &dummyResult{Payload: "Job Done!"}
		storedJob.jobUpdate <- &storedJob.jobInfo
		api.WriteJSONResponse(w, storedJob.jobInfo)

	} else {
		api.WriteJSONErrorResponse(w, api.NewActionError("job not found"), http.StatusNotFound)
	}
}

// NewActions is the default factory
func NewActions(jobActions *jobs.Actions) *Actions {
	return &Actions{
		dummyJobs:  make(map[string]*storedDummyJob),
		jobActions: jobActions,
	}
}
