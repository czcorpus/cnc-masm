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
// Actions contains all the server HTTP REST actions

package root

import (
	"encoding/json"
	"masm/v3/api"
	"masm/v3/general"
	"net/http"
)

type Actions struct {
	Version general.VersionInfo
}

func (a *Actions) OnExit() {}

// RootAction is just an information action about the service
func (a *Actions) RootAction(w http.ResponseWriter, req *http.Request) {
	ans := make(map[string]interface{})
	ans["message"] = "MASM - Manatee Assets, Services and Metadata"
	ans["data"] = a.Version
	resp, err := json.Marshal(ans)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(err), http.StatusInternalServerError)
	}
	w.Write(resp)
}
