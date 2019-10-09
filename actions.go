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

package main

import (
	"encoding/json"
	"fmt"
	"masm/mango"
	"masm/ttdb"
	"net/http"
	"path/filepath"

	"github.com/gorilla/mux"
)

type ActionError struct {
	error
}

func (me ActionError) MarshalJSON() ([]byte, error) {
	return json.Marshal(me.Error())
}

type CorpusInfo struct {
	ID    string       `json:"id"`
	Size  int          `json:"size"`
	Error *ActionError `json:"error"`
}

func writeJSONResponse(w http.ResponseWriter, value interface{}) {
	jsonAns, err := json.Marshal(value)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	w.Write(jsonAns)
}

type Actions struct {
	conf *Conf
}

func (a *Actions) rootAction(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "{\"message\": \"Manatee Administration Setup Middleware\"}")
}

func (a *Actions) getCorpusInfo(w http.ResponseWriter, req *http.Request) {
	var err error
	vars := mux.Vars(req)
	corpusId := vars["corpusId"]

	ans := CorpusInfo{ID: corpusId}
	regPath := filepath.Join(a.conf.RegistryDirPath, corpusId)
	corp, err := mango.OpenCorpus(regPath)
	if err != nil {
		ans.Error = &ActionError{err}
		writeJSONResponse(w, ans)
		return
	}
	ans.Size, err = mango.GetCorpusSize(corp)
	if err != nil {
		ans.Error = &ActionError{err}
		writeJSONResponse(w, ans)
		return
	}
	defer mango.CloseCorpus(corp)
	writeJSONResponse(w, ans)
}

func (a *Actions) getTextTypeDbInfo(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	dbName := vars["dbName"]
	absPath := filepath.Join(a.conf.TextTypesDbDirPath, dbName+".db")
	if ttdb.IsFile(absPath) {
		ans := ttdb.TTDBRecord{Path: absPath, LastModified: ttdb.GetFileMtime(absPath)}
		writeJSONResponse(w, ans)
	} else {
		http.Error(w, "{\"message\": \"File not found\"}", http.StatusNotFound)
	}
}

func NewActions(conf *Conf) *Actions {
	return &Actions{conf: conf}
}
