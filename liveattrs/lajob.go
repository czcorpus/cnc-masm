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

package liveattrs

import (
	"masm/v3/jobs"
	"time"

	vteCnf "github.com/czcorpus/vert-tagextract/v2/cnf"
)

const (
	JobType = "liveattrs"
)

type JobInfoArgs struct {
	Append         bool           `json:"append"`
	VteConf        vteCnf.VTEConf `json:"vteConf"`
	NoCorpusUpdate bool           `json:"noCorpusUpdate"`
}

// LiveAttrsJobInfo collects information about corpus data synchronization job
type LiveAttrsJobInfo struct {
	ID             string        `json:"id"`
	Type           string        `json:"type"`
	CorpusID       string        `json:"corpusId"`
	Start          jobs.JSONTime `json:"start"`
	Update         jobs.JSONTime `json:"update"`
	Finished       bool          `json:"finished"`
	Error          error         `json:"error,omitempty"`
	ProcessedAtoms int           `json:"processedAtoms"`
	ProcessedLines int           `json:"processedLines"`
	NumRestarts    int           `json:"numRestarts"`
	Args           JobInfoArgs   `json:"args"`
}

func (j *LiveAttrsJobInfo) GetID() string {
	return j.ID
}

func (j *LiveAttrsJobInfo) GetType() string {
	return j.Type
}

func (j *LiveAttrsJobInfo) GetStartDT() jobs.JSONTime {
	return j.Start
}

func (j *LiveAttrsJobInfo) GetNumRestarts() int {
	return j.NumRestarts
}

func (j *LiveAttrsJobInfo) GetCorpus() string {
	return j.CorpusID
}

func (j *LiveAttrsJobInfo) SetFinished() {
	j.Update = jobs.CurrentDatetime()
	j.Finished = true
}

func (j *LiveAttrsJobInfo) IsFinished() bool {
	return j.Finished
}

func (j *LiveAttrsJobInfo) FullInfo() any {
	return struct {
		ID             string        `json:"id"`
		Type           string        `json:"type"`
		CorpusID       string        `json:"corpusId"`
		Start          jobs.JSONTime `json:"start"`
		Update         jobs.JSONTime `json:"update"`
		Finished       bool          `json:"finished"`
		Error          error         `json:"error,omitempty"`
		OK             bool          `json:"ok"`
		ProcessedAtoms int           `json:"processedAtoms"`
		ProcessedLines int           `json:"processedLines"`
		NumRestarts    int           `json:"numRestarts"`
		Args           JobInfoArgs   `json:"args"`
	}{
		ID:             j.ID,
		Type:           j.Type,
		CorpusID:       j.CorpusID,
		Start:          j.Start,
		Update:         j.Update,
		Finished:       j.Finished,
		Error:          j.Error,
		OK:             j.Error == nil,
		ProcessedAtoms: j.ProcessedAtoms,
		ProcessedLines: j.ProcessedLines,
		NumRestarts:    j.NumRestarts,
		Args:           j.Args,
	}
}

func (j *LiveAttrsJobInfo) CompactVersion() jobs.JobInfoCompact {
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

func (j *LiveAttrsJobInfo) GetError() error {
	return j.Error
}

// CloneWithError creates a new instance of LiveAttrsJobInfo with
// the Error property set to the value of 'err'.
func (j *LiveAttrsJobInfo) CloneWithError(err error) jobs.GeneralJobInfo {
	return &LiveAttrsJobInfo{
		ID:          j.ID,
		Type:        JobType,
		CorpusID:    j.CorpusID,
		Start:       j.Start,
		Update:      jobs.JSONTime(time.Now()),
		Error:       err,
		NumRestarts: j.NumRestarts,
		Args:        j.Args,
	}
}
