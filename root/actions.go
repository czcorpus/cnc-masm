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
	"masm/v3/cnf"
	"masm/v3/general"
	"net/http"
	"os"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

type Actions struct {
	Version general.VersionInfo
	Conf    *cnf.Conf
}

func (a *Actions) OnExit() {}

// RootAction is just an information action about the service
func (a *Actions) RootAction(ctx *gin.Context) {
	host, err := os.Hostname()
	if err != nil {
		host = "#failed_to_obtain"
	}
	ans := struct {
		Name     string              `json:"name"`
		Version  general.VersionInfo `json:"version"`
		Host     string              `json:"host"`
		ConfPath string              `json:"confPath"`
	}{
		Name:     "FRODO - Frequency Registry Of Dictionary Objects",
		Version:  a.Version,
		Host:     host,
		ConfPath: a.Conf.GetSourcePath(),
	}

	resp, err := json.Marshal(ans)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError("failed to run the root action: %w", err),
			http.StatusInternalServerError,
		)
	}
	ctx.Writer.Write(resp)
}
