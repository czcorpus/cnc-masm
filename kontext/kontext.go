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
	"log"
	"masm/api"
	"masm/cnf"
	"masm/mail"
	"net/http"
	"strconv"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/shirou/gopsutil/process"
)

type processInfo struct {
	Registered   int64
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
		PID:          p.GetPID(),
		InstanceName: p.InstanceName,
		Registered:   p.Registered,
		LastError:    lastError,
	})
}

type processInfoResponse struct {
	Registered   int64   `json:"registered"`
	InstanceName string  `json:"instanceName"`
	PID          int     `json:"pid"`
	LastError    *string `json:"lastError"`
}

type monitoredInstance struct {
	Name       string
	NumErrors  int
	AlarmToken *uuid.UUID
	Viewed     bool
}

func (m *monitoredInstance) MatchesToken(tk string) bool {
	return m.AlarmToken != nil && m.AlarmToken.String() == tk
}

// Actions contains all the server HTTP REST actions
type Actions struct {
	conf               *cnf.Conf
	version            string
	processes          map[int]*processInfo
	ticker             *time.Ticker
	monitoredInstances map[string]*monitoredInstance
}

func (a *Actions) findProcessInfo(pid int) *processInfo {
	return a.processes[pid]
}

func (a *Actions) findProcessInfoByInstanceName(name string) *processInfo {
	for _, v := range a.processes {
		if v.InstanceName == name {
			return v
		}
	}
	return nil
}

func (a *Actions) processesAsList() []*processInfo {
	ans := make([]*processInfo, len(a.processes))
	i := 0
	for _, p := range a.processes {
		ans[i] = p
		i++
	}
	return ans
}

// SoftReset resets either a specified PID process or all the registered
// processes (if no 'pid' URL argument is provided)
func (a *Actions) SoftReset(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	pidStr, ok := vars["pid"]
	var procList map[int]*processInfo
	if ok {
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			api.WriteJSONErrorResponse(w, api.NewActionError(err), http.StatusBadRequest)
			return
		}
		pinfo := a.findProcessInfo(pid)
		if pinfo == nil {
			err := fmt.Errorf("Process %d not registered", pid)
			log.Print(err)
			api.WriteJSONErrorResponse(w, api.NewActionError(err), http.StatusBadRequest)

		} else {
			procList = make(map[int]*processInfo)
			procList[pid] = pinfo
		}

	} else {
		procList = a.processes
	}
	for _, pinfo := range procList {
		err := pinfo.Process.SendSignal(syscall.SIGUSR1)
		if err != nil {
			pinfo.LastError = err
			continue
		}
	}
	api.WriteJSONResponse(w, struct {
		Processes []*processInfo `json:"processes"`
	}{
		Processes: a.processesAsList(),
	})
}

func (a *Actions) addProcInfo(pinfo *processInfo) error {
	if !a.conf.KonTextMonitoring.ContainsInstance(pinfo.InstanceName) {
		return fmt.Errorf("Process instance \"%s\" not configured", pinfo.InstanceName)
	}
	a.processes[pinfo.GetPID()] = pinfo
	log.Printf("INFO: added new process {pid: %d, instance: %s}", pinfo.GetPID(), pinfo.InstanceName)
	return nil
}

// RegisterProcess adds a new PID to be monitored and handled.
// The PID must point to a process with a name defined in the configuration
// (e.g. 'kontext_api', 'kontext_dev',...)
func (a *Actions) RegisterProcess(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	pid, err := strconv.Atoi(vars["pid"])
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionError(err), http.StatusBadRequest)
		return
	}
	for _, pinfo := range a.processes {
		if pid == pinfo.GetPID() {
			err := fmt.Errorf("PID already registered: %d", pid)
			api.WriteJSONErrorResponse(w, api.NewActionError(err), http.StatusBadRequest)
			return
		}
	}
	newProc, err := process.NewProcess(int32(pid))
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionError(err), http.StatusBadRequest)
		return
	}

	newProcInfo, err := importProcess(newProc)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionError(err), http.StatusBadRequest)
		return
	}
	if newProcInfo == nil {
		err = fmt.Errorf("PID %d does not look like a Gunicorn master process", newProc.Pid)
		api.WriteJSONErrorResponse(w, api.NewActionError(err), http.StatusBadRequest)
		return
	}
	err = a.addProcInfo(newProcInfo)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionError(err), http.StatusBadRequest)
		return
	}
	api.WriteJSONResponse(w, newProcInfo)
}

// UnregisterProcess stops monitoring and handling of a specified process
// (identified by URL argument 'pid'). If no such process is found, Bad Request
// is created as a response.
func (a *Actions) UnregisterProcess(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	pid, err := strconv.Atoi(vars["pid"])
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionError(err), http.StatusBadRequest)
		return
	}
	proc, ok := a.processes[pid]
	if !ok {
		err := fmt.Errorf("No such PID registered: %d", pid)
		api.WriteJSONErrorResponse(w, api.NewActionError(err), http.StatusBadRequest)
		return
	}
	delete(a.processes, pid)
	api.WriteJSONResponse(w, proc)
}

// NewActions is the default factory
func NewActions(conf *cnf.Conf, version string) *Actions {
	ticker := time.NewTicker(time.Duration(conf.KonTextMonitoring.CheckIntervalSecs) * time.Second)

	ans := &Actions{
		conf:               conf,
		version:            version,
		processes:          make(map[int]*processInfo),
		ticker:             ticker,
		monitoredInstances: make(map[string]*monitoredInstance),
	}
	for _, v := range conf.KonTextMonitoring.Instances {
		ans.monitoredInstances[v] = &monitoredInstance{
			Name:       v,
			NumErrors:  0,
			AlarmToken: nil,
		}
	}
	go func() {
		for {
			select {
			case <-ans.ticker.C:
				ans.refreshProcesses()
			}
		}
	}()
	return ans
}

func (a *Actions) refreshProcesses() (*autoDetectResult, error) {
	ans, err := autoDetectProcesses()
	if err != nil {
		return nil, err
	}
	for pid, pinfo := range a.processes {
		if !ans.ContainsPID(pid) {
			delete(a.processes, pid)
			log.Printf("WARNING: removed inactive process {pid: %d, instance: %s}", pinfo.GetPID(), pinfo.InstanceName)
			mail.SendNotification(a.conf.KonTextMonitoring,
				"KonText monitoring - unregistered OS process",
				fmt.Sprintf("KonText instance [%s] process %d removed as it is not present any more.",
					pinfo.InstanceName, pinfo.GetPID()),
				nil)
		}
	}
	for _, pinfo := range ans.ProcList {
		_, ok := a.processes[pinfo.GetPID()]
		if !ok {
			// we ignore the possible error here intentionally (autodetect processes => just use what matches config)
			a.addProcInfo(pinfo)
		}
	}
	for name, mon := range a.monitoredInstances {
		srch := a.findProcessInfoByInstanceName(name)
		if srch == nil {
			mon.NumErrors++
			log.Printf("ERROR: cannot find monitored instance %s (repeated: %d)", name, mon.NumErrors)

		} else {
			mon.NumErrors = 0
			mon.AlarmToken = nil
			mon.Viewed = false
		}
		if mon.NumErrors > 0 && mon.NumErrors%a.conf.KonTextMonitoring.AlarmNumErrors == 0 {
			if mon.Viewed == false {
				if mon.AlarmToken == nil {
					tok := uuid.New()
					mon.AlarmToken = &tok
				}
				go func() {
					err = mail.SendNotification(a.conf.KonTextMonitoring,
						"KonText monitoring ALARM - process down",
						fmt.Sprintf("============= ALARM ===============\n\rKonText instance [%s] is down.", mon.Name),
						mon.AlarmToken)
					log.Print("INFO: sent an e-mail notification")
					if err != nil {
						log.Print("ERROR: ", err)
					}
				}()

			} else {
				log.Print("INFO: alarm already turned off")
			}
		}
	}
	return ans, nil
}

// AutoDetectProcesses refreshes an internal list of running KonText processes.
// The process auto-refresh is performed regularly by MASM so the method should
// be considered as a fallback solution when something goes wrong.
func (a *Actions) AutoDetectProcesses(w http.ResponseWriter, req *http.Request) {
	ans, err := a.refreshProcesses()
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionError(err), http.StatusBadRequest)
		return
	}
	api.WriteJSONResponse(w, ans)
}

// ResetAlarm is an action providing a confirmation that the addressee
// has been informed about a problem. This is typically confirmed via
// a link included in a respective alarm e-mail.
func (a *Actions) ResetAlarm(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	token := vars["token"]
	for _, mon := range a.monitoredInstances {
		if mon.MatchesToken(token) {
			mon.Viewed = true
			api.WriteJSONResponse(w, mon)
			return
		}
	}
	err := fmt.Errorf("Token %s not found", token)
	api.WriteJSONErrorResponse(w, api.NewActionError(err), http.StatusBadRequest)
}
