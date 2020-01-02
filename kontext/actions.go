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

// Actions contains all the server HTTP REST actions
type Actions struct {
	conf               *cnf.Conf
	version            string
	ticker             *time.Ticker
	monitoredInstances map[string]*monitoredInstance
}

func (a *Actions) processesAsList() []*processInfo {
	ans := make([]*processInfo, len(a.monitoredInstances))
	i := 0
	for _, p := range a.monitoredInstances {
		ans[i] = p.ProcInfo
		i++
	}
	return ans
}

func (a *Actions) findMonitoredInstanceByPID(pid int) *monitoredInstance {
	for _, mon := range a.monitoredInstances {
		if mon.ProcInfo != nil && mon.ProcInfo.GetPID() == pid {
			return mon
		}
	}
	return nil
}

func (a *Actions) addProcInfo(pinfo *processInfo) error {
	_, ok := a.monitoredInstances[pinfo.InstanceName]
	if !ok {
		return fmt.Errorf("Process instance \"%s\" not configured", pinfo.InstanceName)
	}
	a.monitoredInstances[pinfo.InstanceName].ProcInfo = pinfo
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
	for _, mon := range a.monitoredInstances {
		if pid == mon.ProcInfo.GetPID() {
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
	mon := a.findMonitoredInstanceByPID(pid)
	if mon == nil {
		err := fmt.Errorf("No such PID registered: %d", pid)
		api.WriteJSONErrorResponse(w, api.NewActionError(err), http.StatusBadRequest)
		return
	}
	proc := mon.ProcInfo
	mon.ProcInfo = nil
	api.WriteJSONResponse(w, proc)
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
		mon := a.findMonitoredInstanceByPID(pid)
		if mon == nil {
			err := fmt.Errorf("Process %d not registered", pid)
			log.Print(err)
			api.WriteJSONErrorResponse(w, api.NewActionError(err), http.StatusBadRequest)

		} else {
			procList = make(map[int]*processInfo)
			procList[pid] = mon.ProcInfo
		}

	} else {
		for _, p := range a.processesAsList() {
			procList[p.GetPID()] = p
		}
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

func (a *Actions) refreshProcesses() (*processList, error) {
	ans, err := autoDetectProcesses()
	if err != nil {
		return nil, err
	}
	for instanceName, mon := range a.monitoredInstances {
		if mon.ProcInfo == nil {
			continue
		}
		pid := mon.ProcInfo.GetPID()
		if !ans.ContainsPID(pid) {
			mon.ProcInfo = nil
			log.Printf("WARNING: removed inactive process {pid: %d, instance: %s}", pid, instanceName)
			mail.SendNotification(a.conf.KonTextMonitoring,
				"KonText monitoring - unregistered OS process",
				fmt.Sprintf("KonText instance [%s] process %d removed as it is not present any more.",
					instanceName, pid),
				nil)
		}
	}
	for _, pinfo := range ans.ProcList {
		mon := a.findMonitoredInstanceByPID(pinfo.GetPID())
		if mon == nil {
			// we ignore the possible error here intentionally (autodetect processes => just use what matches config)
			a.addProcInfo(pinfo)
		}
	}
	for name, mon := range a.monitoredInstances {
		if mon.ProcInfo == nil {
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

func (a *Actions) ListAll(w http.ResponseWriter, req *http.Request) {
	ans := make([]*monitoredInstance, len(a.monitoredInstances))
	i := 0
	for _, mon := range a.monitoredInstances {
		ans[i] = mon
		i++
	}
	api.WriteJSONResponse(w, ans)
}
