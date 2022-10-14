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

package cncdb

import (
	"database/sql"
	"fmt"
	"masm/v3/corpus"
	"net/http"

	"github.com/rs/zerolog/log"

	"masm/v3/api"

	"github.com/gorilla/mux"
)

// DataHandler describes functions expected from
// CNC information system database as needed by KonText
// (and possibly other apps).
type DataHandler interface {
	UpdateSize(transact *sql.Tx, corpus string, size int64) error
	UpdateDescription(transact *sql.Tx, corpus, descCs, descEn string) error
	StartTx() (*sql.Tx, error)
	CommitTx(transact *sql.Tx) error
	RollbackTx(transact *sql.Tx) error
}

type updateSizeResp struct {
	OK bool `json:"ok"`
}

// Actions contains all the server HTTP REST actions
type Actions struct {
	conf *corpus.Conf
	db   DataHandler
}

// NewActions is the default factory
func NewActions(conf *corpus.Conf, db DataHandler) *Actions {
	return &Actions{
		conf: conf,
		db:   db,
	}
}

func (a *Actions) UpdateCorpusInfo(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	corpusID := vars["corpusId"]
	baseErrTpl := fmt.Sprintf("failed to update info for corpus %s", corpusID)
	corpusInfo, err := corpus.GetCorpusInfo(corpusID, "", a.conf.CorporaSetup)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusInternalServerError)
		return
	}
	if !corpusInfo.IndexedData.Path.FileExists {
		err := fmt.Errorf("data not found for corpus %s", corpusID)
		api.WriteJSONErrorResponse(
			w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusNotFound)
		return
	}
	transact, err := a.db.StartTx()
	err = a.db.UpdateSize(transact, corpusID, corpusInfo.IndexedData.Size)
	if err != nil {
		err2 := a.db.RollbackTx(transact)
		if err2 != nil {
			log.Error().Err(err2).Msg("failed to rollback transaction")
		}
		api.WriteJSONErrorResponse(
			w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusInternalServerError)
		return
	}

	err = a.db.UpdateDescription(transact, corpusID, req.FormValue("description_cs"),
		req.FormValue("description_en"))
	if err != nil {
		err2 := a.db.RollbackTx(transact)
		if err2 != nil {
			log.Error().Err(err2).Msg("failed to rollback transaction")
		}
	}

	err = a.db.CommitTx(transact)
	if err != nil {
		api.WriteJSONErrorResponse(
			w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusInternalServerError)
	}
	api.WriteJSONResponse(w, updateSizeResp{OK: true})
}
