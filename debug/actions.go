// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Institute of the Czech National Corpus,
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

package debug

import (
	"fmt"
	"net/http"

	"masm/v3/jobs"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type storedDummyJob struct {
	jobInfo   jobs.DummyJobInfo
	jobUpdate chan jobs.GeneralJobInfo
}

// Actions contains all the server HTTP REST actions
type Actions struct {
	finishSignals map[string]chan<- bool
	jobActions    *jobs.Actions
}

// GetCorpusInfo provides some basic information about stored data
func (a *Actions) CreateDummyJob(ctx *gin.Context) {
	jobID, err := uuid.NewUUID()
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError("failed to create dummy job"), http.StatusUnauthorized)
		return
	}

	jobInfo := &jobs.DummyJobInfo{
		ID:       jobID.String(),
		Type:     "dummy-job",
		Start:    jobs.CurrentDatetime(),
		CorpusID: "dummy",
	}
	if ctx.Request.URL.Query().Get("error") == "1" {
		jobInfo.Error = fmt.Errorf("dummy error")
	}
	finishSignal := make(chan bool)
	fn := func(upds chan<- jobs.GeneralJobInfo) {
		defer close(upds)
		<-finishSignal
		jobInfo.Result = &jobs.DummyJobResult{Payload: "Job Done!"}
		upds <- jobInfo.AsFinished()
	}
	a.jobActions.EnqueueJob(&fn, jobInfo)
	a.finishSignals[jobID.String()] = finishSignal
	uniresp.WriteJSONResponse(ctx.Writer, jobInfo)
}

func (a *Actions) FinishDummyJob(ctx *gin.Context) {
	finish, ok := a.finishSignals[ctx.Param("jobId")]
	if ok {
		delete(a.finishSignals, ctx.Param("jobId"))
		defer close(finish)
		finish <- true
		if storedJob, ok := a.jobActions.GetJob(ctx.Param("jobId")); ok {
			// TODO please note that here we typically won't see the
			// final storedJob value (updated elsewhere in a different
			// goroutine). So it may be a bit confusing.
			uniresp.WriteJSONResponse(ctx.Writer, storedJob.FullInfo())

		} else {
			uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError("job not found"), http.StatusNotFound)
		}

	} else {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError("job not found"), http.StatusNotFound)
	}
}

// NewActions is the default factory
func NewActions(jobActions *jobs.Actions) *Actions {
	return &Actions{
		finishSignals: make(map[string]chan<- bool),
		jobActions:    jobActions,
	}
}
