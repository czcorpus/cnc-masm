// Copyright 2023 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2023 Institute of the Czech National Corpus,
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

package conc

import (
	"masm/v3/corpus"
	"masm/v3/mango"
	"net/http"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

type Actions struct {
	conf *corpus.Conf
}

func (a *Actions) CreateConcordance(ctx *gin.Context) {
	q := ctx.Request.URL.Query().Get("q")
	corp, err := corpus.OpenCorpus(ctx.Param("corpusId"), a.conf.CorporaSetup)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError, // TODO the status should be based on err type
		)
		return
	}
	conc, err := mango.CreateConcordance(corp, q)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError, // TODO the status should be based on err type
		)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, map[string]any{"concSize": conc.Size()})
}

func NewActions(conf *corpus.Conf) *Actions {
	return &Actions{
		conf: conf,
	}
}
