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
	"masm/v3/liveattrs/db"
	"masm/v3/liveattrs/request/biblio"
	"masm/v3/liveattrs/request/query"
	"net/http"
	"regexp"
	"strconv"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

var (
	attrValidRegex = regexp.MustCompile(`^[a-zA-Z0-9_\.]+$`)
)

func (a *Actions) GetBibliography(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	baseErrTpl := "failed to get bibliography from corpus %s: %w"

	var qry biblio.Payload
	err := json.NewDecoder(ctx.Request.Body).Decode(&qry)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusBadRequest)
		return
	}
	corpInfo, err := a.cncDB.LoadInfo(corpusID)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		return
	}
	laConf, err := a.laConfCache.Get(corpInfo.Name)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		return
	}
	ans, err := db.GetBibliography(a.laDB, corpInfo, laConf, qry)
	if err == db.ErrorEmptyResult {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusNotFound)
		return

	} else if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, &ans)
}

func (a *Actions) FindBibTitles(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	baseErrTpl := "failed to find bibliography titles in corpus %s: %w"

	var qry biblio.PayloadList
	err := json.NewDecoder(ctx.Request.Body).Decode(&qry)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusBadRequest)
		return
	}
	corpInfo, err := a.cncDB.LoadInfo(corpusID)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		return
	}
	laConf, err := a.laConfCache.Get(corpInfo.Name)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		return
	}
	ans, err := db.FindBibTitles(a.laDB, corpInfo, laConf, qry)
	if err == db.ErrorEmptyResult {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusNotFound)
		return

	} else if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, &ans)
}

func isValidAttr(a string) bool {
	return attrValidRegex.MatchString(a)
}

func (a *Actions) DocumentList(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	baseErrTpl := "failed to download document list from %s: %w"
	corpInfo, err := a.cncDB.LoadInfo(corpusID)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError(baseErrTpl, corpusID, err),
			http.StatusInternalServerError,
		)
		return
	}
	if corpInfo.BibIDAttr == "" {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError(baseErrTpl, corpusID, fmt.Errorf("bib. ID not defined for %s", corpusID)),
			http.StatusNotFound,
		)
		return
	}
	spage := ctx.Request.URL.Query().Get("page")
	if spage == "" {
		spage = "1"
	}
	page, err := strconv.Atoi(spage)
	if err != nil {
		if err != nil {
			uniresp.WriteJSONErrorResponse(
				ctx.Writer,
				uniresp.NewActionError(baseErrTpl, corpusID, err),
				http.StatusBadRequest,
			)
			return
		}
	}
	spageSize := ctx.Request.URL.Query().Get("pageSize")
	if spageSize == "" {
		spageSize = "0"
	}
	pageSize, err := strconv.Atoi(spageSize)
	if err != nil {
		if err != nil {
			uniresp.WriteJSONErrorResponse(
				ctx.Writer,
				uniresp.NewActionError(baseErrTpl, corpusID, err),
				http.StatusBadRequest,
			)
			return
		}
	}
	if pageSize == 0 && page != 1 || pageSize < 0 || page < 0 {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError(
				baseErrTpl,
				corpusID,
				fmt.Errorf("page or pageSize argument incorrect (got: %d and %d)", page, pageSize)),
			http.StatusUnprocessableEntity,
		)
		return
	}

	pginfo := db.PageInfo{Page: page, PageSize: pageSize}

	for _, v := range ctx.Request.URL.Query()["attr"] {
		if !isValidAttr(v) {
			uniresp.WriteJSONErrorResponse(
				ctx.Writer,
				uniresp.NewActionError(baseErrTpl, corpusID, fmt.Errorf("incorrect attribute %s", v)),
				http.StatusUnprocessableEntity,
			)
			return
		}
	}

	var qry query.Payload
	err = json.NewDecoder(ctx.Request.Body).Decode(&qry)
	if err != nil && err != io.EOF {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusBadRequest)
		return

	}

	var ans []*db.DocumentRow
	ans, err = db.GetDocuments(
		a.laDB,
		corpInfo,
		ctx.Request.URL.Query()["attr"],
		qry.Aligned,
		qry.Attrs,
		pginfo,
	)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError(baseErrTpl, corpusID, err),
			http.StatusInternalServerError,
		)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, ans)
}

func (a *Actions) NumMatchingDocuments(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	baseErrTpl := "failed to count number of matching documents in %s: %w"
	corpInfo, err := a.cncDB.LoadInfo(corpusID)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError(baseErrTpl, corpusID, err),
			http.StatusInternalServerError,
		)
		return
	}
	if corpInfo.BibIDAttr == "" {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError(baseErrTpl, corpusID, fmt.Errorf("bib. ID not defined for %s", corpusID)),
			http.StatusNotFound,
		)
		return
	}

	var qry query.Payload
	err = json.NewDecoder(ctx.Request.Body).Decode(&qry)
	if err != nil && err != io.EOF {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusBadRequest)
		return
	}

	ans, err := db.GetNumOfDocuments(
		a.laDB,
		corpInfo,
		qry.Aligned,
		qry.Attrs,
	)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError(baseErrTpl, corpusID, err),
			http.StatusInternalServerError,
		)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, ans)
}
