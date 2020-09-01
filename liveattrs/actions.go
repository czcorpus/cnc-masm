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
	"io"
	"log"
	"masm/api"
	"masm/cnf"
	"masm/corpus"
	"masm/fsops"
	"masm/jobs"
	"masm/kontext"
	"net/http"
	"os"
	"path/filepath"

	vteCnf "github.com/czcorpus/vert-tagextract/cnf"
	vteLib "github.com/czcorpus/vert-tagextract/library"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

const (
	jobType = "liveattrs"
)

func loadConf(basePath, corpname string) (*vteCnf.VTEConf, error) {
	return vteCnf.LoadConf(filepath.Join(basePath, fmt.Sprintf("%s.json", corpname)))
}

func installDatabase(corpusID, tmpPath, textTypesDbDirPath string) error {
	dbFileName := corpus.GenCorpusGroupName(corpusID) + ".db"
	absPath := filepath.Join(textTypesDbDirPath, dbFileName)
	srcFile, err := os.Open(tmpPath)
	if err != nil {
		return err
	}
	dstFile, err := os.Create(absPath)
	if err != nil {
		return err
	}
	_, err = io.Copy(dstFile, srcFile)
	return err
}

// Actions wraps liveattrs-related actions
type Actions struct {
	exitEvent      <-chan os.Signal
	conf           *cnf.Conf
	jobActions     *jobs.Actions
	kontextActions *kontext.Actions
}

func (a *Actions) OnExit() {}

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
		api.WriteJSONErrorResponse(w, api.NewActionError("Cannot run liveattrs generator - config loading error:", err), http.StatusInternalServerError)
		return
	}

	if fsops.IsFile(conf.DBFile) {
		log.Printf("INFO: MASM found an existing workspace database for %s - deleting", conf.Corpus)
		err := os.Remove(conf.DBFile)
		if err != nil {
			api.WriteJSONErrorResponse(w, api.NewActionError("Cannot run liveattrs generator - failed to remove db in workspace:  ", err), http.StatusInternalServerError)
			return
		}
	}

	status := &JobInfo{
		ID:       jobID.String(),
		CorpusID: corpusID,
		Start:    jobs.CurrentDatetime(),
	}
	procStatus, err := vteLib.ExtractData(conf, false, a.exitEvent)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionError("Cannot run liveattrs generator:", err), http.StatusNotFound)
		return
	}
	go func() {
		updateJobChan := a.jobActions.AddJobInfo(status)
		defer close(updateJobChan)
		var lastErr error
		for upd := range procStatus {
			if upd.Error != nil {
				lastErr = upd.Error
			}
			updateJobChan <- &JobInfo{
				ID:             status.ID,
				Type:           jobType,
				CorpusID:       status.CorpusID,
				Start:          status.Start,
				Update:         jobs.CurrentDatetime(),
				Error:          jobs.NewJSONError(upd.Error),
				ProcessedAtoms: upd.ProcessedAtoms,
				ProcessedLines: upd.ProcessedLines,
			}
		}

		if lastErr == nil {
			err := installDatabase(conf.Corpus, conf.DBFile, a.conf.CorporaSetup.TextTypesDbDirPath)
			if err != nil {
				updateJobChan <- &JobInfo{
					ID:       status.ID,
					Type:     jobType,
					CorpusID: status.CorpusID,
					Start:    status.Start,
					Error:    jobs.NewJSONError(err),
				}

			} else {
				err = a.kontextActions.SoftResetAll()
				if err != nil {
					updateJobChan <- &JobInfo{
						ID:       status.ID,
						Type:     jobType,
						CorpusID: status.CorpusID,
						Start:    status.Start,
						Error:    jobs.NewJSONError(err),
					}
				}
			}
		}
	}()
	api.WriteJSONResponse(w, status)
}

// NewActions is the default factory for Actions
func NewActions(
	conf *cnf.Conf,
	exitEvent <-chan os.Signal,
	jobActions *jobs.Actions,
	kontextActions *kontext.Actions,
	version cnf.VersionInfo,
) *Actions {
	return &Actions{
		exitEvent:      exitEvent,
		conf:           conf,
		jobActions:     jobActions,
		kontextActions: kontextActions,
	}
}
