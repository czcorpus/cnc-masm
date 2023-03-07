// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
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

package jobs

import (
	"time"
)

type DummyJobResult struct {
	Payload string `json:"payload"`
}

//DummyJobInfo collects information about corpus data synchronization job
type DummyJobInfo struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	CorpusID    string          `json:"corpusId"`
	Start       JSONTime        `json:"start"`
	Update      JSONTime        `json:"update"`
	Finished    bool            `json:"finished"`
	Error       error           `json:"error,omitempty"`
	Result      *DummyJobResult `json:"result"`
	NumRestarts int             `json:"numRestarts"`
}

func (j DummyJobInfo) GetID() string {
	return j.ID
}

func (j DummyJobInfo) GetType() string {
	return j.Type
}

func (j DummyJobInfo) GetStartDT() JSONTime {
	return j.Start
}

func (j DummyJobInfo) GetNumRestarts() int {
	return j.NumRestarts
}

func (j DummyJobInfo) GetCorpus() string {
	return j.CorpusID
}

func (j DummyJobInfo) IsFinished() bool {
	return j.Finished
}

func (j DummyJobInfo) AsFinished() GeneralJobInfo {
	j.Update = CurrentDatetime()
	j.Finished = true
	return j
}

func (j DummyJobInfo) CompactVersion() JobInfoCompact {
	item := JobInfoCompact{
		ID:       j.ID,
		Type:     j.Type,
		CorpusID: j.CorpusID,
		Start:    j.Start,
		Update:   j.Update,
		Finished: j.Finished,
		OK:       true,
	}
	if j.Error != nil || (j.Result == nil) {
		item.OK = false
	}
	return item
}

func (j DummyJobInfo) FullInfo() any {
	return struct {
		ID          string          `json:"id"`
		Type        string          `json:"type"`
		CorpusID    string          `json:"corpusId"`
		Start       JSONTime        `json:"start"`
		Update      JSONTime        `json:"update"`
		Finished    bool            `json:"finished"`
		Error       string          `json:"error,omitempty"`
		OK          bool            `json:"ok"`
		Result      *DummyJobResult `json:"result"`
		NumRestarts int             `json:"numRestarts"`
	}{
		ID:          j.ID,
		Type:        j.Type,
		CorpusID:    j.CorpusID,
		Start:       j.Start,
		Update:      j.Update,
		Finished:    j.Finished,
		Error:       ErrorToString(j.Error),
		OK:          j.Error == nil,
		Result:      j.Result,
		NumRestarts: j.NumRestarts,
	}
}

func (j DummyJobInfo) GetError() error {
	return j.Error
}

func (j DummyJobInfo) WithError(err error) GeneralJobInfo {
	return DummyJobInfo{
		ID:          j.ID,
		Type:        j.Type,
		CorpusID:    j.CorpusID,
		Start:       j.Start,
		Update:      JSONTime(time.Now()),
		Finished:    j.Finished,
		Error:       err,
		Result:      j.Result,
		NumRestarts: j.NumRestarts,
	}
}
