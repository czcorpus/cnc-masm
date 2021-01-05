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
	"fmt"
	"masm/api"
	"masm/cnf"
	"net/http"

	"github.com/gorilla/mux"
)

// Actions wraps liveattrs-related actions
type Actions struct {
	conf *cnf.Conf
}

// DynamicFunctions provides a list of Manatee internal + our configured functions
// for generating dynamic attributes
func (a *Actions) DynamicFunctions(w http.ResponseWriter, req *http.Request) {
	fullList := dynFnList[:]
	fullList = append(fullList, DynFn{
		Name:        "geteachncharbysep",
		Args:        []string{"n"},
		Description: "Separate a string by \"|\" and return all the pos-th elements from respective items",
		Dynlib:      a.conf.CorporaSetup.ManateeDynlibPath,
	})
	api.WriteCacheableJSONResponse(w, req, fullList)
}

func (a *Actions) PosSets(w http.ResponseWriter, req *http.Request) {
	ans := make([]PosSimple, len(posList))
	for i, v := range posList {
		ans[i] = PosSimple{ID: v.ID, Name: v.Name}
	}
	api.WriteCacheableJSONResponse(w, req, ans)
}

func (a *Actions) GetPosSetInfo(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	posID := vars["posId"]
	var srch Pos
	for _, v := range posList {
		if v.ID == posID {
			srch = v
		}
	}
	if srch.ID == "" {
		api.WriteJSONErrorResponse(w, api.NewActionError(fmt.Sprintf("Tagset %s not found", posID)), http.StatusInternalServerError)

	} else {
		api.WriteJSONResponse(w, srch)
	}
}

func (a *Actions) GetAttrMultivalueDefaults(w http.ResponseWriter, req *http.Request) {
	api.WriteJSONResponse(w, availBoolValues)
}

func (a *Actions) GetAttrMultisepDefaults(w http.ResponseWriter, req *http.Request) {
	ans := []multisep{
		{Value: "|", Description: "A default value used within the CNC"},
	}
	api.WriteJSONResponse(w, ans)
}

func (a *Actions) GetAttrDynlibDefaults(w http.ResponseWriter, req *http.Request) {
	ans := []dynlibItem{
		{Value: "internal", Description: "Functions provided by Manatee"},
		{Value: a.conf.CorporaSetup.ManateeDynlibPath, Description: "Custom functions provided by the CNC"},
	}
	api.WriteJSONResponse(w, ans)
}

func (a *Actions) GetAttrTransqueryDefaults(w http.ResponseWriter, req *http.Request) {
	api.WriteJSONResponse(w, availBoolValues)
}

// NewActions is the default factory for Actions
func NewActions(
	conf *cnf.Conf,
) *Actions {
	return &Actions{
		conf: conf,
	}
}
