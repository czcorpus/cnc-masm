// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Institute of the Czech National Corpus,
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
	"masm/v3/liveattrs/qs"
	"net/http"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

func (a *Actions) CreateQuerySuggestions(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	baseErrTpl := "failed to generate query suggestions for %s: %w"
	multiValuesEnabled := ctx.Request.URL.Query().Get("multiValuesEnabled") == "1"

	corpusDBInfo, err := a.cncDB.LoadInfo(corpusID)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		return
	}
	exporter := qs.NewExporter(
		a.conf.Ngram,
		a.laDB,
		corpusDBInfo.GroupedName(),
		multiValuesEnabled,
		a.jobActions,
	)
	jobInfo, err := exporter.EnqueueExportJob(ctx.Request.URL.Query().Get("parentJobId"))
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, jobInfo)
}
