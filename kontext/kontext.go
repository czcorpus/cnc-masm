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

package kontext

import (
	"encoding/json"
	"fmt"
	"masm/api"
	"masm/cnf"
	"net/http"
	"os"
	"strconv"
	"syscall"
	"time"

	"github.com/gorilla/mux"
)

type processInfo struct {
	Registered int64
	PID        int
	LastError  error
}

func (p *processInfo) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Registered int64  `json:"registered"`
		PID        int    `json:"pid"`
		LastError  string `json:"lastError"`
	}{
		Registered: p.Registered,
		PID:        p.PID,
		LastError:  p.LastError.Error(),
	})
}

type processInfoResponse struct {
	Registered int64  `json:"registered"`
	PID        int    `json:"pid"`
	LastError  string `json:"lastError"`
}

// Actions contains all the server HTTP REST actions
type Actions struct {
	conf      *cnf.Conf
	version   string
	processes []*processInfo
}

func (a *Actions) SoftReset(w http.ResponseWriter, req *http.Request) {
	for _, pinfo := range a.processes {
		proc, err := os.FindProcess(pinfo.PID)
		if err != nil {
			pinfo.LastError = err
			continue
		}
		err = proc.Signal(syscall.SIGUSR1)
		if err != nil {
			pinfo.LastError = err
			continue
		}
	}
	api.WriteJSONResponse(w, struct {
		Processes []*processInfo `json:"processes"`
	}{
		Processes: a.processes,
	})
}

func (a *Actions) RegisterProcess(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	pid, err := strconv.Atoi(vars["pid"])
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionError(err), http.StatusBadRequest)
		return
	}
	for _, pinfo := range a.processes {
		if pid == pinfo.PID {
			err := fmt.Errorf("PID already registered: %d", pid)
			api.WriteJSONErrorResponse(w, api.NewActionError(err), http.StatusBadRequest)
			return
		}
	}
	newProc := processInfo{
		PID:        pid,
		Registered: time.Now().Unix(),
		LastError:  nil,
	}
	a.processes = append(a.processes, &newProc)
	api.WriteJSONResponse(w, newProc)
}

// NewActions is the default factory
func NewActions(conf *cnf.Conf, version string) *Actions {
	return &Actions{
		conf:      conf,
		version:   version,
		processes: make([]*processInfo, 0, 500),
	}
}
