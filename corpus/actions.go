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
	"database/sql"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/google/uuid"

	"masm/v3/jobs"
)

const (
	jobTypeSyncCNK = "sync-cnk"
)

type CorpusInfoProvider interface {
	LoadInfo(corpusID string) (*DBInfo, error)
}

// Actions contains all the server HTTP REST actions
type Actions struct {
	conf         *CorporaSetup
	osSignal     chan os.Signal
	jobsConf     *jobs.Conf
	jobActions   *jobs.Actions
	infoProvider CorpusInfoProvider
}

// GetCorpusInfo provides some basic information about stored data
func (a *Actions) GetCorpusInfo(ctx *gin.Context) {
	var err error
	corpusID := ctx.Param("corpusId")
	baseErrTpl := "failed to get corpus info for %s: %w"
	dbInfo, err := a.infoProvider.LoadInfo(corpusID)

	if err == sql.ErrNoRows {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusNotFound)
		log.Error().Err(err)
		return

	} else if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		log.Error().Err(err)
		return
	}
	ans, err := GetCorpusInfo(corpusID, a.conf, dbInfo.HasLimitedVariant)
	if err == CorpusNotFound {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusNotFound)
		log.Error().Err(err)
		return

	} else if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		log.Error().Err(err)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, ans)
}

func (a *Actions) RestartJob(jinfo *JobInfo) error {
	err := a.jobActions.TestAllowsJobRestart(jinfo)
	if err != nil {
		return err
	}
	jinfo.Start = jobs.CurrentDatetime()
	jinfo.NumRestarts++
	jinfo.Update = jobs.CurrentDatetime()

	fn := func(updateJobChan chan<- jobs.GeneralJobInfo) {
		defer close(updateJobChan)
		resp, err := synchronizeCorpusData(&a.conf.CorpusDataPath, jinfo.CorpusID)
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
func (a *Actions) SynchronizeCorpusData(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	subdir := ctx.Param("subdir")
	if subdir != "" {
		corpusID = filepath.Join(subdir, corpusID)
	}
	if !a.conf.AllowsSyncForCorpus(corpusID) {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError("Corpus synchronization forbidden for '%s'", corpusID), http.StatusUnauthorized)
		return
	}

	jobID, err := uuid.NewUUID()
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError("Failed to start synchronization job for '%s'", corpusID), http.StatusUnauthorized)
		return
	}

	if prevRunning, ok := a.jobActions.LastUnfinishedJobOfType(corpusID, jobTypeSyncCNK); ok {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError("Cannot run synchronization - the previous job '%s' have not finished yet", prevRunning), http.StatusConflict)
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
		resp, err := synchronizeCorpusData(&a.conf.CorpusDataPath, corpusID)
		if err != nil {
			jobRec.Error = err
		}
		jobRec.Result = &resp
		updateJobChan <- jobRec.AsFinished()
	}
	a.jobActions.EnqueueJob(&fn, jobRec)

	uniresp.WriteJSONResponse(ctx.Writer, jobRec.FullInfo())
}

// NewActions is the default factory
func NewActions(
	conf *CorporaSetup,
	jobsConf *jobs.Conf,
	jobActions *jobs.Actions,
	infoProvider CorpusInfoProvider,
) *Actions {
	return &Actions{
		conf:         conf,
		jobsConf:     jobsConf,
		jobActions:   jobActions,
		infoProvider: infoProvider,
	}
}
