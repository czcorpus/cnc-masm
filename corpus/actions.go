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
	"net/http"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"masm/v3/jobs"
)

const (
	jobTypeSyncCNK = "sync-cnk"
)

// Actions contains all the server HTTP REST actions
type Actions struct {
	conf       *Conf
	osSignal   chan os.Signal
	jobActions *jobs.Actions
}

func (a *Actions) OnExit() {}

// GetCorpusInfo provides some basic information about stored data
func (a *Actions) GetCorpusInfo(w http.ResponseWriter, req *http.Request) {
	var err error
	vars := mux.Vars(req)
	corpusID := vars["corpusId"]
	subdir := vars["subdir"]
	if subdir != "" {
		corpusID = filepath.Join(subdir, corpusID)
	}
	baseErrTpl := "failed to get corpus info for %s: %w"
	wsattr := req.URL.Query().Get("wsattr")
	if wsattr == "" {
		wsattr = "lemma"
	}
	log.Info().Msgf("request[corpusID: %s, wsattr: %s]", corpusID, wsattr)
	ans, err := GetCorpusInfo(corpusID, wsattr, a.conf.CorporaSetup)
	switch err.(type) {
	case NotFound:
		uniresp.WriteJSONErrorResponse(
			w, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusNotFound)
		log.Error().Err(err)
	case InfoError:
		uniresp.WriteJSONErrorResponse(
			w, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		log.Error().Err(err)
	case nil:
		uniresp.WriteJSONResponse(w, ans)
	}
}

func (a *Actions) RestartJob(jinfo *JobInfo) error {
	if jinfo.NumRestarts >= a.conf.Jobs.MaxNumRestarts {
		return fmt.Errorf("cannot restart job %s - max. num. of restarts reached", jinfo.ID)
	}
	jinfo.Start = jobs.CurrentDatetime()
	jinfo.NumRestarts++
	jinfo.Update = jobs.CurrentDatetime()

	fn := func(updateJobChan chan<- jobs.GeneralJobInfo) {
		defer close(updateJobChan)
		resp, err := synchronizeCorpusData(&a.conf.CorporaSetup.CorpusDataPath, jinfo.CorpusID)
		if err != nil {
			updateJobChan <- jinfo.WithError(err)

		} else {
			newJinfo := *jinfo
			newJinfo.Result = &resp

			updateJobChan <- newJinfo.AsFinished()
		}
	}
	a.jobActions.EnqueueJob(&fn, jinfo)
	log.Info().Msgf("Restarted corpus job %s", jinfo.ID)
	return nil
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
		uniresp.WriteJSONErrorResponse(w, uniresp.NewActionError("Corpus synchronization forbidden for '%s'", corpusID), http.StatusUnauthorized)
		return
	}

	jobID, err := uuid.NewUUID()
	if err != nil {
		uniresp.WriteJSONErrorResponse(w, uniresp.NewActionError("Failed to start synchronization job for '%s'", corpusID), http.StatusUnauthorized)
		return
	}

	if prevRunning, ok := a.jobActions.LastUnfinishedJobOfType(corpusID, jobTypeSyncCNK); ok {
		uniresp.WriteJSONErrorResponse(w, uniresp.NewActionError("Cannot run synchronization - the previous job '%s' have not finished yet", prevRunning), http.StatusConflict)
		return
	}

	jobKey := jobID.String()
	jobRec := &JobInfo{
		ID:       jobKey,
		Type:     jobTypeSyncCNK,
		CorpusID: corpusID,
		Start:    jobs.CurrentDatetime(),
	}

	// now let's define and enqueue the actual synchronization
	fn := func(updateJobChan chan<- jobs.GeneralJobInfo) {
		defer close(updateJobChan)
		resp, err := synchronizeCorpusData(&a.conf.CorporaSetup.CorpusDataPath, corpusID)
		if err != nil {
			jobRec.Error = err
		}
		jobRec.Result = &resp
		updateJobChan <- jobRec.AsFinished()
	}
	a.jobActions.EnqueueJob(&fn, jobRec)

	uniresp.WriteJSONResponse(w, jobRec.FullInfo())
}

// NewActions is the default factory
func NewActions(conf *Conf, jobActions *jobs.Actions) *Actions {
	return &Actions{
		conf:       conf,
		jobActions: jobActions,
	}
}
