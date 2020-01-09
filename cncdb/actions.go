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
	"masm/cnf"
	"masm/corpus"
	"net/http"

	"masm/api"

	"github.com/gorilla/mux"
)

// DataHandler describes functions expected from
// CNC information system database as needed by KonText
// (and possibly other apps).
type DataHandler interface {
	UpdateSize(corpus string, size int64) error
}

type updateSizeResp struct {
	OK bool `json:"ok"`
}

// Actions contains all the server HTTP REST actions
type Actions struct {
	conf *cnf.Conf
	db   DataHandler
}

// NewActions is the default factory
func NewActions(conf *cnf.Conf, db DataHandler) *Actions {
	return &Actions{
		conf: conf,
		db:   db,
	}
}

func (a *Actions) UpdateCorpusInfo(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	corpusID := vars["corpusId"]
	corpusInfo, err := corpus.GetCorpusInfo(corpusID, "", a.conf.CorporaSetup)
	if !corpusInfo.IndexedData.Path.FileExists {
		api.WriteJSONErrorResponse(w, api.NewActionError("Corpus %s not found", corpusID), http.StatusNotFound)
		return
	}
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(err), http.StatusInternalServerError)
		return
	}
	err = a.db.UpdateSize(corpusID, corpusInfo.IndexedData.Size)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(err), http.StatusInternalServerError)
		return
	}
	api.WriteJSONResponse(w, updateSizeResp{OK: true})
}