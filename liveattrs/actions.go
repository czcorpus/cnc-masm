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

package liveattrs

import (
	"fmt"
	"masm/api"
	"masm/cnf"
	"masm/jobs"
	"net/http"
	"path/filepath"

	vteCnf "github.com/czcorpus/vert-tagextract/cnf"
	vteLib "github.com/czcorpus/vert-tagextract/library"
	"github.com/czcorpus/vert-tagextract/proc"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

const (
	jobType = "liveattrs"
)

func loadConf(basePath, corpname string) (*vteCnf.VTEConf, error) {
	return vteCnf.LoadConf(filepath.Join(basePath, fmt.Sprintf("%s.json", corpname)))
}

// Actions wraps liveattrs-related actions
type Actions struct {
	exitEvent  chan struct{}
	conf       *cnf.Conf
	jobActions *jobs.Actions
}

// Create handles creating of liveattrs data for a specific corpus
func (a *Actions) Create(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	corpusID := vars["corpusId"]

	// TODO search collisions only in liveattrs type jobs

	jobID, err := uuid.NewUUID()
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionError("Failed to start liveattrs job for '%s'", corpusID), http.StatusUnauthorized)
		return
	}

	if prevRunning := a.jobActions.GetUnfinishedJob(corpusID, jobType); prevRunning != nil {
		api.WriteJSONErrorResponse(
			w,
			api.NewActionError("Cannot run liveattrs generator - the previous job '%s' have not finished yet", prevRunning.GetID()),
			http.StatusConflict,
		)
		return
	}

	conf, err := loadConf(a.conf.CorporaSetup.LiveAttrsConfPath, corpusID)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionError("Cannot run liveattrs generator - config loading error", err), http.StatusNotFound)
		return
	}
	procStatusChan := make(chan proc.Status, 10)
	status := &JobInfo{
		ID:       jobID.String(),
		CorpusID: corpusID,
		Start:    jobs.CurrentDatetime(),
	}
	updateJobChan := a.jobActions.AddJobInfo(status)
	go func() {
		vteLib.ExtractData(conf, false, a.exitEvent, procStatusChan)
	}()
	go func() {
		for upd := range procStatusChan {
			errStr := ""
			if upd.Error != nil {
				errStr = upd.Error.Error()
			}
			updateJobChan <- &JobInfo{
				ID:             status.ID,
				Type:           jobType,
				CorpusID:       status.CorpusID,
				Start:          status.Start,
				Error:          errStr,
				ProcessedAtoms: upd.ProcessedAtoms,
				ProcessedLines: upd.ProcessedLines,
			}
		}
		close(updateJobChan)
	}()
	api.WriteJSONResponse(w, status)

}

// NewActions is the default factory for Actions
func NewActions(conf *cnf.Conf, exitEvent chan struct{}, jobActions *jobs.Actions, version cnf.VersionInfo) *Actions {
	return &Actions{
		exitEvent:  exitEvent,
		conf:       conf,
		jobActions: jobActions,
	}
}
