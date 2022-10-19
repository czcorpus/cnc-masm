// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Institute of the Czech National Corpus,
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

package actions

import (
	"masm/v3/jobs"
	"time"
)

type SubJobInfo struct {
	ID       string `json:"id"`
	Running  bool   `json:"running"`
	Finished bool   `json:"finished"`
}

type NgramsAndQSJobInfoArgs struct {
}

type NgramsAndQSResult struct {
	NgramsJob SubJobInfo `json:"ngramsJob"`
	ExportJob SubJobInfo `json:"exportJob"`
}

// NgramsAndQSJobInfo
type NgramsAndQSJobInfo struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	CorpusID    string                 `json:"corpusId"`
	Start       jobs.JSONTime          `json:"start"`
	Update      jobs.JSONTime          `json:"update"`
	Finished    bool                   `json:"finished"`
	Error       error                  `json:"error,omitempty"`
	NumRestarts int                    `json:"numRestarts"`
	Args        NgramsAndQSJobInfoArgs `json:"args"`
	Result      NgramsAndQSResult      `json:"result"`
}

func (j *NgramsAndQSJobInfo) GetID() string {
	return j.ID
}

func (j *NgramsAndQSJobInfo) GetType() string {
	return j.Type
}

func (j *NgramsAndQSJobInfo) GetStartDT() jobs.JSONTime {
	return j.Start
}

func (j *NgramsAndQSJobInfo) GetNumRestarts() int {
	return j.NumRestarts
}

func (j *NgramsAndQSJobInfo) GetCorpus() string {
	return j.CorpusID
}

func (j *NgramsAndQSJobInfo) SetFinished() {
	j.Update = jobs.CurrentDatetime()
	j.Finished = true
}

func (j *NgramsAndQSJobInfo) IsFinished() bool {
	return j.Finished
}

func (j *NgramsAndQSJobInfo) FullInfo() any {
	return struct {
		ID          string                 `json:"id"`
		Type        string                 `json:"type"`
		CorpusID    string                 `json:"corpusId"`
		Start       jobs.JSONTime          `json:"start"`
		Update      jobs.JSONTime          `json:"update"`
		Finished    bool                   `json:"finished"`
		Error       string                 `json:"error,omitempty"`
		OK          bool                   `json:"ok"`
		NumRestarts int                    `json:"numRestarts"`
		Args        NgramsAndQSJobInfoArgs `json:"args"`
		Result      NgramsAndQSResult      `json:"result"`
	}{
		ID:          j.ID,
		Type:        j.Type,
		CorpusID:    j.CorpusID,
		Start:       j.Start,
		Update:      j.Update,
		Finished:    j.Finished,
		Error:       jobs.ErrorToString(j.Error),
		OK:          j.Error == nil,
		NumRestarts: j.NumRestarts,
		Args:        j.Args,
		Result:      j.Result,
	}
}

func (j *NgramsAndQSJobInfo) CompactVersion() jobs.JobInfoCompact {
	item := jobs.JobInfoCompact{
		ID:       j.ID,
		Type:     j.Type,
		CorpusID: j.CorpusID,
		Start:    j.Start,
		Update:   j.Update,
		Finished: j.Finished,
		OK:       true,
	}
	item.OK = j.Error == nil
	return item
}

func (j *NgramsAndQSJobInfo) GetError() error {
	return j.Error
}

func (j *NgramsAndQSJobInfo) CloneWithError(err error) jobs.GeneralJobInfo {
	return &NgramsAndQSJobInfo{
		ID:          j.ID,
		Type:        j.Type,
		CorpusID:    j.CorpusID,
		Start:       j.Start,
		Update:      jobs.JSONTime(time.Now()),
		Finished:    j.Finished,
		Error:       err,
		Result:      j.Result,
		NumRestarts: j.NumRestarts,
	}
}
