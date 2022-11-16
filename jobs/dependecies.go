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

package jobs

import (
	"errors"
	"time"
)

var (
	ErrorNoSuchJobDependency   = errors.New("no such dependecy")
	ErrorCircularJobDependency = errors.New("circular job dependency")
	ErrorDuplicateDependency   = errors.New("duplicate dependency")
)

type depInfo struct {
	createdAt time.Time
	jobID     string
	finished  bool
	hasError  bool
}

type JobsDeps map[string][]*depInfo

func (jd JobsDeps) Add(jobID string, parentID string) error {
	if _, ok := jd[jobID]; !ok {
		jd[jobID] = make([]*depInfo, 0, 10)
	}
	for _, parent := range jd[jobID] {
		if parent.jobID == parentID {
			return ErrorDuplicateDependency
		}
	}
	jd[jobID] = append(jd[jobID], &depInfo{time.Now(), parentID, false, false})
	hasCircle := jd.findCircle(jobID)
	if hasCircle {
		jd[jobID] = jd[jobID][:len(jd[jobID])-1]
		return ErrorCircularJobDependency
	}
	return nil
}

func (jd JobsDeps) getParentIDs(jobID string) []string {
	v := jd[jobID]
	ans := make([]string, len(v))
	for i, item := range v {
		ans[i] = item.jobID
	}
	return ans
}

func (jd JobsDeps) findCircle(jobID string) bool {
	visited := make(map[string]bool)
	queue := []string{jobID}

	for len(queue) > 0 {
		curr := queue[0]
		if _, ok := visited[curr]; ok {
			return true
		}
		visited[curr] = true
		queue = append(queue[1:], jd.getParentIDs(curr)...)
	}
	return false
}

func (jd JobsDeps) SetParentFinished(parentID string, hasError bool) error {
	for _, depJob := range jd {
		for _, parent := range depJob {
			if parent.jobID == parentID {
				parent.finished = true
				parent.hasError = hasError
			}
		}
	}
	return ErrorNoSuchJobDependency
}

// MustWait tests whether jobID must wait because one of its
// parents are not finished yet (i.e. the parent must be unfinished
// and not failed).
// In case no dependency is defined for jobID, ErrorNoSuchJobDependency
// is returned.
func (jd JobsDeps) MustWait(jobID string) (bool, error) {
	v, ok := jd[jobID]
	if !ok {
		return false, ErrorNoSuchJobDependency
	}
	var someFailed, someRunning bool
	for _, parent := range v {
		if parent.hasError {
			someFailed = true
		}
		if !parent.finished {
			someRunning = true
		}
	}
	return someRunning && !someFailed, nil
}

func (jd JobsDeps) HasFailedParent(jobID string) (bool, error) {
	v, ok := jd[jobID]
	if !ok {
		return false, ErrorNoSuchJobDependency
	}
	for _, parent := range v {
		if parent.hasError {
			return true, nil
		}
	}
	return false, nil
}
