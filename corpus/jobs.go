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

package corpus

import (
	"encoding/gob"
	"log"
	"os"
	"strings"
	"time"
)

// JobInfo collects information about corpus data synchronization job
type JobInfo struct {
	ID       string        `json:"id"`
	CorpusID string        `json:"corpusId"`
	Start    string        `json:"start"`
	Finish   string        `json:"finish"`
	Error    string        `json:"error"`
	Result   *syncResponse `json:"result"`
}

type JobInfoList []*JobInfo

func (jil *JobInfoList) Serialize(path string) error {
	fw, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer fw.Close()
	enc := gob.NewEncoder(fw)
	return enc.Encode(jil)
}

func LoadJobList(path string) (JobInfoList, error) {
	fw, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}
	dec := gob.NewDecoder(fw)
	ans := make(JobInfoList, 0, 50)
	err = dec.Decode(&ans)
	return ans, err
}

func (jil JobInfoList) Len() int {
	return len(jil)
}

func (jil JobInfoList) Less(i, j int) bool {
	return strings.Compare(jil[i].Start, jil[j].Start) == -1
}

func (jil JobInfoList) Swap(i, j int) {
	jil[i], jil[j] = jil[j], jil[i]
}

func clearOldJobs(data map[string]*JobInfo) {
	curr := time.Now()
	for k, v := range data {
		t, err := time.Parse(time.RFC3339, v.Start)
		if err != nil {
			log.Print("WARNING: job datetime info malformed: ", err)
		}
		if curr.Sub(t) > time.Duration(168)*time.Hour {
			delete(data, k)
		}
	}
}

func getUnfinishedJobForCorpus(data map[string]*JobInfo, corpusID string) string {
	for k, v := range data {
		if v.CorpusID == corpusID && v.Finish == "" {
			return k
		}
	}
	return ""
}

func findJob(syncJobs map[string]*JobInfo, jobID string) *JobInfo {
	var ans *JobInfo
	for ident, job := range syncJobs {
		if strings.HasPrefix(ident, jobID) {
			if ans != nil {
				return nil
			}
			ans = job
		}
	}
	return ans
}

type JobInfoCompact struct {
	ID       string `json:"id"`
	CorpusID string `json:"corpusId"`
	Start    string `json:"start"`
	Finish   string `json:"finish"`
	OK       bool   `json:"ok"`
}

type JobInfoListCompact []*JobInfoCompact

func (cjil JobInfoListCompact) Len() int {
	return len(cjil)
}

func (cjil JobInfoListCompact) Less(i, j int) bool {
	return strings.Compare(cjil[i].Start, cjil[j].Start) == -1
}

func (cjil JobInfoListCompact) Swap(i, j int) {
	cjil[i], cjil[j] = cjil[j], cjil[i]
}
