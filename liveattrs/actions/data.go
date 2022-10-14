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

package actions

import (
	"masm/v3/api"
	"masm/v3/jobs"
	"masm/v3/liveattrs"
	"masm/v3/liveattrs/db"
	"net/http"

	vteCnf "github.com/czcorpus/vert-tagextract/v2/cnf"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// Create starts a process of creating fresh liveattrs data for a a specified corpus.
// URL args:
// * atomStructure - a minimal structure masm will be able to search for (typically 'doc', 'text')
// * noCache - if '1' then masm regenerates data extraction configuration based on actual corpus
//   registry file
// * bibIdAttr - if defined then masm will create bibliography entries with IDs matching values from
//   from referred bibIdAttr values
// * maxNumErrors - limit number of parsing errors for processed vertical file(s)
// * skipNgrams - if '1' then n-grams won't be generated even if they are (pre)configured
//   (either via previous PUT /liveAttributes/{corpusId}/conf or by passing JSON args with n-gram
//   configuration). In case the setting cannot have an effect (= n-grams are not configured),
//   the setting is silently ignored.
func (a *Actions) Create(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	corpusID := vars["corpusId"]
	noCache := false
	if req.URL.Query().Get("noCache") == "1" {
		noCache = true
	}

	var err error
	var conf *vteCnf.VTEConf
	if !noCache {
		conf, err = a.laConfCache.Get(corpusID)
	}

	var jsonArgs *liveattrsJsonArgs
	if conf == nil {
		var newConf *vteCnf.VTEConf
		var err error
		newConf, jsonArgs, err = a.createConf(corpusID, req, false)
		if err != nil {
			api.WriteJSONErrorResponse(w, api.NewActionErrorFromMsg("LiveAttrs generator failed: '%s'", err), http.StatusBadRequest)
			return
		}

		err = a.laConfCache.Save(newConf)
		if err != nil {
			api.WriteJSONErrorResponse(w, api.NewActionErrorFromMsg("LiveAttrs generator failed: '%s'", err), http.StatusBadRequest)
			return
		}

		conf, err = a.laConfCache.Get(corpusID)
		if err != nil {
			api.WriteJSONErrorResponse(w, api.NewActionErrorFromMsg("LiveAttrs generator failed: '%s'", err), http.StatusBadRequest)
			return
		}

	} else {
		jsonArgs, err = a.getJsonArgs(req)
		if err != nil {
			api.WriteJSONErrorResponse(w, api.NewActionErrorFromMsg("LiveAttrs generator failed: '%s'", err), http.StatusBadRequest)
			return
		}
	}

	runtimeConf := *conf
	if len(jsonArgs.VerticalFiles) > 0 {
		runtimeConf.VerticalFile = ""
		runtimeConf.VerticalFiles = jsonArgs.VerticalFiles
	}
	if jsonArgs.Ngrams.NgramSize > 0 && req.URL.Query().Get("skipNgrams") == "1" {
		runtimeConf.Ngrams = vteCnf.NgramConf{}
	}

	// TODO search collisions only in liveattrs type jobs
	jobID, err := uuid.NewUUID()
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFromMsg("Failed to start liveattrs job for '%s'", corpusID), http.StatusUnauthorized)
		return
	}

	if prevRunning, ok := a.jobActions.LastUnfinishedJobOfType(corpusID, liveattrs.JobType); ok {
		api.WriteJSONErrorResponse(
			w,
			api.NewActionErrorFromMsg(
				"LiveAttrs generator failed - the previous job '%s' have not finished yet",
				prevRunning.GetID(),
			),
			http.StatusConflict,
		)
		return
	}

	append := req.URL.Query().Get("append")
	noCorpusUpdate := req.URL.Query().Get("noCorpusUpdate")
	status := &liveattrs.LiveAttrsJobInfo{
		ID:       jobID.String(),
		CorpusID: corpusID,
		Start:    jobs.CurrentDatetime(),
		Args: liveattrs.JobInfoArgs{
			VteConf:        runtimeConf,
			Append:         append == "1",
			NoCorpusUpdate: noCorpusUpdate == "1",
		},
	}
	err = a.createDataFromJobStatus(status)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(err), http.StatusInternalServerError)
		return
	}
	api.WriteJSONResponseWithStatus(w, http.StatusCreated, status.FullInfo())
}

// Delete removes all the live attributes data for a corpus
func (a *Actions) Delete(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	corpusID := vars["corpusId"]
	corpusDBInfo, err := a.cncDB.LoadInfo(corpusID)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(err), http.StatusInternalServerError)
		return
	}
	tx0, err := a.laDB.Begin()
	err = db.DeleteTable(
		tx0,
		corpusDBInfo.GroupedName(),
		corpusID,
	)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(err), http.StatusInternalServerError)
		tx0.Rollback()
		return
	}
	tx1, err := a.cncDB.StartTx()
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(err), http.StatusInternalServerError)
		return
	}
	err = a.cncDB.UnsetLiveAttrs(tx1, corpusID)
	if err != nil {
		tx1.Rollback()
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(err), http.StatusInternalServerError)
		return
	}
	// Now we commit tx0 and tx1 deliberately before soft reset below as a failed operation of
	// cache reset does no permanent damage.
	// Also please note that the two independent transactions tx0, tx1 here cannot provide
	// behavior of a single transaction which means the operation may end up in a
	// non-consistent state (if tx0 commits and tx1 fails).
	err = tx0.Commit()
	if err != nil {
		tx1.Rollback()
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(err), http.StatusInternalServerError)
		return
	}
	err = tx1.Commit() // in case this fails we're screwed as tx0 is already commited
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(err), http.StatusInternalServerError)
		return
	}
	err = a.setSoftResetToKontext()
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(err), http.StatusInternalServerError)
		return
	}
	api.WriteJSONResponse(w, map[string]any{"ok": true})
}
