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
	"encoding/json"
	"fmt"
	"io"
	"masm/v3/api"
	"masm/v3/liveattrs/db"
	"masm/v3/liveattrs/request/biblio"
	"masm/v3/liveattrs/request/query"
	"net/http"
	"regexp"
	"strconv"

	"github.com/gorilla/mux"
)

var (
	attrValidRegex = regexp.MustCompile(`^[a-zA-Z0-9_\.]+$`)
)

func (a *Actions) GetBibliography(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	corpusID := vars["corpusId"]
	baseErrTpl := fmt.Sprintf("failed to get bibliography from corpus %s", corpusID)

	var qry biblio.Payload
	err := json.NewDecoder(req.Body).Decode(&qry)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusBadRequest)
		return
	}
	corpInfo, err := a.cncDB.LoadInfo(corpusID)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusInternalServerError)
		return
	}
	laConf, err := a.laConfCache.Get(corpInfo.Name)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusInternalServerError)
		return
	}
	ans, err := db.GetBibliography(a.laDB, corpInfo, laConf, qry)
	if err == db.ErrorEmptyResult {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusNotFound)
		return

	} else if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusInternalServerError)
		return
	}
	api.WriteJSONResponse(w, &ans)
}

func (a *Actions) FindBibTitles(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	corpusID := vars["corpusId"]
	baseErrTpl := fmt.Sprintf("failed to find bibliography titles in corpus %s", corpusID)

	var qry biblio.PayloadList
	err := json.NewDecoder(req.Body).Decode(&qry)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusBadRequest)
		return
	}
	corpInfo, err := a.cncDB.LoadInfo(corpusID)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusInternalServerError)
		return
	}
	laConf, err := a.laConfCache.Get(corpInfo.Name)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusInternalServerError)
		return
	}
	ans, err := db.FindBibTitles(a.laDB, corpInfo, laConf, qry)
	if err == db.ErrorEmptyResult {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusNotFound)
		return

	} else if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusInternalServerError)
		return
	}
	api.WriteJSONResponse(w, &ans)
}

func isValidAttr(a string) bool {
	return attrValidRegex.MatchString(a)
}

func (a *Actions) DocumentList(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	corpusID := vars["corpusId"]
	baseErrTpl := fmt.Sprintf("failed to download document list from %s", corpusID)
	corpInfo, err := a.cncDB.LoadInfo(corpusID)
	if err != nil {
		api.WriteJSONErrorResponse(
			w,
			api.NewActionErrorFrom(baseErrTpl, err),
			http.StatusInternalServerError,
		)
		return
	}
	if corpInfo.BibIDAttr == "" {
		api.WriteJSONErrorResponse(
			w,
			api.NewActionErrorFrom(baseErrTpl, fmt.Errorf("bib. ID not defined for %s", corpusID)),
			http.StatusNotFound,
		)
		return
	}
	spage := req.URL.Query().Get("page")
	if spage == "" {
		spage = "1"
	}
	page, err := strconv.Atoi(spage)
	if err != nil {
		if err != nil {
			api.WriteJSONErrorResponse(
				w,
				api.NewActionErrorFrom(baseErrTpl, err),
				http.StatusBadRequest,
			)
			return
		}
	}
	spageSize := req.URL.Query().Get("pageSize")
	if spageSize == "" {
		spageSize = "0"
	}
	pageSize, err := strconv.Atoi(spageSize)
	if err != nil {
		if err != nil {
			api.WriteJSONErrorResponse(
				w,
				api.NewActionErrorFrom(baseErrTpl, err),
				http.StatusBadRequest,
			)
			return
		}
	}
	if pageSize == 0 && page != 1 || pageSize < 0 || page < 0 {
		api.WriteJSONErrorResponse(
			w,
			api.NewActionErrorFrom(
				baseErrTpl,
				fmt.Errorf("page or pageSize argument incorrect (got: %d and %d)", page, pageSize)),
			http.StatusUnprocessableEntity,
		)
		return
	}

	pginfo := db.PageInfo{Page: page, PageSize: pageSize}

	var ans []*db.DocumentRow
	for _, v := range req.URL.Query()["attr"] {
		if !isValidAttr(v) {
			api.WriteJSONErrorResponse(
				w,
				api.NewActionErrorFrom(baseErrTpl, fmt.Errorf("incorrect attribute %s", v)),
				http.StatusUnprocessableEntity,
			)
			return
		}
	}

	var qry query.Payload
	err = json.NewDecoder(req.Body).Decode(&qry)
	if err != nil && err != io.EOF {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusBadRequest)
		return
	}

	ans, err = db.GetDocuments(
		a.laDB,
		corpInfo,
		req.URL.Query()["attr"],
		qry.Aligned,
		qry.Attrs,
		pginfo,
	)
	if err != nil {
		api.WriteJSONErrorResponse(
			w,
			api.NewActionErrorFrom(baseErrTpl, err),
			http.StatusInternalServerError,
		)
		return
	}
	api.WriteJSONResponse(w, ans)
}

func (a *Actions) NumMatchingDocuments(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	corpusID := vars["corpusId"]
	baseErrTpl := fmt.Sprintf("failed to count numberf of matching documents in %s", corpusID)
	corpInfo, err := a.cncDB.LoadInfo(corpusID)
	if err != nil {
		api.WriteJSONErrorResponse(
			w,
			api.NewActionErrorFrom(baseErrTpl, err),
			http.StatusInternalServerError,
		)
		return
	}
	if corpInfo.BibIDAttr == "" {
		api.WriteJSONErrorResponse(
			w,
			api.NewActionErrorFrom(baseErrTpl, fmt.Errorf("bib. ID not defined for %s", corpusID)),
			http.StatusNotFound,
		)
		return
	}

	var qry query.Payload
	err = json.NewDecoder(req.Body).Decode(&qry)
	if err != nil && err != io.EOF {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusBadRequest)
		return
	}

	ans, err := db.GetNumOfDocuments(
		a.laDB,
		corpInfo,
		qry.Aligned,
		qry.Attrs,
	)
	if err != nil {
		api.WriteJSONErrorResponse(
			w,
			api.NewActionErrorFrom(baseErrTpl, err),
			http.StatusInternalServerError,
		)
		return
	}
	api.WriteJSONResponse(w, ans)
}
