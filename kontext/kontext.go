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
	"log"
	"masm/cnf"
	"time"

	"github.com/google/uuid"
	"github.com/shirou/gopsutil/process"
)

type processInfo struct {
	Registered   *string
	InstanceName string
	Process      *process.Process
	LastError    error
}

func (p *processInfo) GetPID() int {
	return int(p.Process.Pid)
}

func (p *processInfo) MarshalJSON() ([]byte, error) {
	var lastError *string
	if p.LastError != nil {
		err := p.LastError.Error()
		lastError = &err
	}
	return json.Marshal(processInfoResponse{
		PID:            p.GetPID(),
		InstanceName:   p.InstanceName,
		RegisteredTime: p.Registered,
		LastError:      lastError,
	})
}

type processInfoResponse struct {
	RegisteredTime *string `json:"registeredTime"`
	InstanceName   string  `json:"instanceName"`
	PID            int     `json:"pid"`
	LastError      *string `json:"lastError"`
}

type monitoredInstance struct {
	Name       string       `json:"name"`
	NumErrors  int          `json:"numErrors"`
	AlarmToken *uuid.UUID   `json:"alarmToken"`
	ViewedTime *string      `json:"viewedTime"`
	ProcInfo   *processInfo `json:"procInfo"`
}

func (m *monitoredInstance) MatchesToken(tk string) bool {
	return m.AlarmToken != nil && m.AlarmToken.String() == tk
}

// NewActions is the default factory
func NewActions(conf *cnf.Conf, version string) *Actions {
	var ticker *time.Ticker

	if conf.KonTextMonitoring != nil && conf.KonTextMonitoring.CheckIntervalSecs > 0 {
		ticker = time.NewTicker(time.Duration(conf.KonTextMonitoring.CheckIntervalSecs) * time.Second)

	} else {
		log.Print("WARNING: KonText service monitoring not set - skipping")
	}

	ans := &Actions{
		conf:               conf,
		version:            version,
		ticker:             ticker,
		monitoredInstances: make(map[string]*monitoredInstance),
	}

	if conf.KonTextMonitoring != nil {
		for _, v := range conf.KonTextMonitoring.Instances {
			ans.monitoredInstances[v] = &monitoredInstance{
				Name:       v,
				NumErrors:  0,
				AlarmToken: nil,
				ViewedTime: nil,
			}
		}
	}

	if ticker != nil {
		go func() {
			ans.refreshProcesses()
			for {
				select {
				case <-ans.ticker.C:
					ans.refreshProcesses()
				}
			}
		}()
	}
	return ans
}
