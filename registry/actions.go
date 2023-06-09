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

package registry

import (
	"masm/v3/corpus"
	"net/http"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

// Actions wraps liveattrs-related actions
type Actions struct {
	conf *corpus.Conf
}

// DynamicFunctions provides a list of Manatee internal + our configured functions
// for generating dynamic attributes
func (a *Actions) DynamicFunctions(ctx *gin.Context) {
	fullList := dynFnList[:]
	fullList = append(fullList, DynFn{
		Name:        "geteachncharbysep",
		Args:        []string{"str", "n"},
		Description: "Separate a string by \"|\" and return all the pos-th elements from respective items",
		Dynlib:      a.conf.CorporaSetup.ManateeDynlibPath,
	})
	uniresp.WriteCacheableJSONResponse(ctx.Writer, ctx.Request, fullList)
}

func (a *Actions) PosSets(ctx *gin.Context) {
	ans := make([]Pos, len(posList))
	for i, v := range posList {
		ans[i] = v
	}
	uniresp.WriteCacheableJSONResponse(ctx.Writer, ctx.Request, ans)
}

func (a *Actions) GetPosSetInfo(ctx *gin.Context) {
	posID := ctx.Param("posId")
	var srch Pos
	for _, v := range posList {
		if v.ID == posID {
			srch = v
		}
	}
	if srch.ID == "" {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError("Tagset %s not found", posID), http.StatusInternalServerError)

	} else {
		uniresp.WriteJSONResponse(ctx.Writer, srch)
	}
}

func (a *Actions) GetAttrMultivalueDefaults(ctx *gin.Context) {
	uniresp.WriteJSONResponse(ctx.Writer, availBoolValues)
}

func (a *Actions) GetAttrMultisepDefaults(ctx *gin.Context) {
	ans := []multisep{
		{Value: "|", Description: "A default value used within the CNC"},
	}
	uniresp.WriteJSONResponse(ctx.Writer, ans)
}

func (a *Actions) GetAttrDynlibDefaults(ctx *gin.Context) {
	ans := []dynlibItem{
		{Value: "internal", Description: "Functions provided by Manatee"},
		{Value: a.conf.CorporaSetup.ManateeDynlibPath, Description: "Custom functions provided by the CNC"},
	}
	uniresp.WriteJSONResponse(ctx.Writer, ans)
}

func (a *Actions) GetAttrTransqueryDefaults(ctx *gin.Context) {
	uniresp.WriteJSONResponse(ctx.Writer, availBoolValues)
}

func (a *Actions) GetStructMultivalueDefaults(ctx *gin.Context) {
	uniresp.WriteJSONResponse(ctx.Writer, availBoolValues)
}

func (a *Actions) GetStructMultisepDefaults(ctx *gin.Context) {
	ans := []multisep{
		{Value: "|", Description: "A default value used within the CNC"},
	}
	uniresp.WriteJSONResponse(ctx.Writer, ans)
}

// NewActions is the default factory for Actions
func NewActions(
	conf *corpus.Conf,
) *Actions {
	return &Actions{
		conf: conf,
	}
}
