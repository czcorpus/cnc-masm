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

package qs

import (
	"masm/v3/jobs"
	"time"
)

type ExportJobInfoArgs struct {
}

// ExportJobInfo
type ExportJobInfo struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	CorpusID    string            `json:"corpusId"`
	Start       jobs.JSONTime     `json:"start"`
	Update      jobs.JSONTime     `json:"update"`
	Finished    bool              `json:"finished"`
	Error       error             `json:"error,omitempty"`
	NumRestarts int               `json:"numRestarts"`
	Args        ExportJobInfoArgs `json:"args"`
	Result      exporterStatus    `json:"result"`
}

func (j *ExportJobInfo) GetID() string {
	return j.ID
}

func (j *ExportJobInfo) GetType() string {
	return j.Type
}

func (j *ExportJobInfo) GetStartDT() jobs.JSONTime {
	return j.Start
}

func (j *ExportJobInfo) GetNumRestarts() int {
	return j.NumRestarts
}

func (j *ExportJobInfo) GetCorpus() string {
	return j.CorpusID
}

func (j *ExportJobInfo) SetFinished() {
	j.Update = jobs.CurrentDatetime()
	j.Finished = true
}

func (j *ExportJobInfo) IsFinished() bool {
	return j.Finished
}

func (j *ExportJobInfo) FullInfo() any {
	return struct {
		ID          string            `json:"id"`
		Type        string            `json:"type"`
		CorpusID    string            `json:"corpusId"`
		Start       jobs.JSONTime     `json:"start"`
		Update      jobs.JSONTime     `json:"update"`
		Finished    bool              `json:"finished"`
		Error       string            `json:"error,omitempty"`
		OK          bool              `json:"ok"`
		NumRestarts int               `json:"numRestarts"`
		Args        ExportJobInfoArgs `json:"args"`
		Result      exporterStatus    `json:"result"`
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

func (j *ExportJobInfo) CompactVersion() jobs.JobInfoCompact {
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

func (j *ExportJobInfo) GetError() error {
	return j.Error
}

func (j *ExportJobInfo) CloneWithError(err error) jobs.GeneralJobInfo {
	return &ExportJobInfo{
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
