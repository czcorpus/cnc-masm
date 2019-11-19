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
	"log"
	"masm/corpus"
	"net/http"

	"github.com/gorilla/mux"
)

// ActionError represents a basic user action error (e.g. a wrong parameter,
// non-existing record etc.)
type ActionError struct {
	error
}

// MarshalJSON serializes the error to JSON
func (me ActionError) MarshalJSON() ([]byte, error) {
	return json.Marshal(me.Error())
}

// ErrorResponse describes a wrapping object for all error HTTP responses
type ErrorResponse struct {
	Error *ActionError `json:"error"`
}

// writeJSONResponse writes 'value' to an HTTP response encoded as JSON
func writeJSONResponse(w http.ResponseWriter, value interface{}) {
	jsonAns, err := json.Marshal(value)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	w.Write(jsonAns)
}

// writeJSONErrorResponse writes 'aerr' to an HTTP error response  as JSON
func writeJSONErrorResponse(w http.ResponseWriter, aerr ActionError, status int) {
	ans := &ErrorResponse{Error: &aerr}
	jsonAns, err := json.Marshal(ans)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	w.WriteHeader(status)
	w.Write(jsonAns)
}

// Actions contains all the server HTTP REST actions
type Actions struct {
	conf    *Conf
	version string
}

func (a *Actions) rootAction(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "{\"message\": \"Manatee Administration Setup Middleware v%s\"}", a.version)
}

func (a *Actions) getCorpusInfo(w http.ResponseWriter, req *http.Request) {
	var err error
	vars := mux.Vars(req)
	corpusID := vars["corpusId"]
	wsattr := req.URL.Query().Get("wsattr")
	if wsattr == "" {
		wsattr = "lemma"
	}
	log.Printf("INFO: request[corpusID: %s, wsattr: %s]", corpusID, wsattr)
	ans, err := corpus.GetCorpusInfo(corpusID, wsattr, &a.conf.CorporaSetup)
	switch err.(type) {
	case corpus.NotFound:
		writeJSONErrorResponse(w, ActionError{err}, http.StatusNotFound)
		log.Printf("ERROR: %s", err)
	case corpus.InfoError:
		writeJSONErrorResponse(w, ActionError{err}, http.StatusInternalServerError)
		log.Printf("ERROR: %s", err)
	case nil:
		writeJSONResponse(w, ans)
	}
}

// NewActions is the default factory
func NewActions(conf *Conf, version string) *Actions {
	return &Actions{conf: conf, version: version}
}
