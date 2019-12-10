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
	"regexp"
	"time"

	"github.com/shirou/gopsutil/process"
)

var (
	gunicProcSrch = regexp.MustCompile("gunicorn:\\s*master\\s+\\[([^\\]]+)\\]")
)

type autoDetectResult struct {
	Errors   []string
	ProcList []*processInfo
}

func (adr *autoDetectResult) ContainsPID(pid int) bool {
	for _, p := range adr.ProcList {
		if int(p.Process.Pid) == pid {
			return true
		}
	}
	return false
}

//
// gunicorn: master [kontext_production]
func importProcess(proc *process.Process) (*processInfo, error) {
	cmdLine, err := proc.Cmdline()
	if err != nil {
		return nil, err
	}
	srch := gunicProcSrch.FindStringSubmatch(cmdLine)
	if len(srch) > 0 {
		return &processInfo{
			Process:      proc,
			InstanceName: srch[1],
			Registered:   time.Now().Unix(),
			LastError:    nil,
		}, nil
	}
	return nil, nil
}

func autoDetectProcesses() (*autoDetectResult, error) {
	procList, err := process.Processes()
	if err != nil {
		return nil, err
	}
	ans := &autoDetectResult{
		Errors:   make([]string, 0, len(procList)),
		ProcList: make([]*processInfo, 0, len(procList)),
	}

	for _, proc := range procList {
		procInfo, err := importProcess(proc)
		if err != nil {
			ans.Errors = append(ans.Errors, err.Error())

		} else if procInfo != nil {
			ans.ProcList = append(ans.ProcList, procInfo)
		}
	}
	return ans, nil
}
