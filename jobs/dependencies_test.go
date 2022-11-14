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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddDependency(t *testing.T) {
	deps := make(JobsDeps)
	deps.Add("child1", "parent1")
	assert.Equal(t, deps["child1"][0].jobID, "parent1")
}

func TestCheckDependencies(t *testing.T) {
	deps := make(JobsDeps)
	deps.Add("child1", "parent1")
	deps.Add("child1", "parent2")
	ans, err := deps.MustWait("child1")
	assert.NoError(t, err)
	assert.True(t, ans)
	ans, err = deps.HasFailedParent("child1")
	assert.NoError(t, err)
	assert.False(t, ans)
}

func TestFinishedParentUnblocking(t *testing.T) {
	deps := make(JobsDeps)
	err := deps.Add("child1", "parentA")
	assert.NoError(t, err)
	err = deps.Add("child1", "parentB")
	assert.NoError(t, err)
	err = deps.Add("child2", "parentA")
	assert.NoError(t, err)
	deps.SetParentFinished("parentA", false)

	ans, err := deps.MustWait("child1")
	assert.NoError(t, err)
	assert.True(t, ans) // because child1 has yet another dependency

	ans, err = deps.MustWait("child2")
	assert.NoError(t, err)
	assert.False(t, ans)
}

func TestFailedParent(t *testing.T) {
	deps := make(JobsDeps)
	err := deps.Add("child1", "parentA")
	assert.NoError(t, err)
	err = deps.Add("child1", "parentB")
	assert.NoError(t, err)
	deps.SetParentFinished("parentA", true)

	ans, err := deps.MustWait("child1")
	assert.NoError(t, err)
	assert.False(t, ans)
	ans, err = deps.HasFailedParent("child1")
	assert.NoError(t, err)
	assert.True(t, ans)
}

func TestCannotCreateCircle(t *testing.T) {
	deps := make(JobsDeps)
	deps.Add("item1", "item1_1")
	deps.Add("item2", "parent2")
	deps.Add("item3", "parent2")
	deps.Add("item1_1", "item1_1_1")
	deps.Add("item1_1", "item1_1_2")
	deps.Add("item1_1_1", "item2")
	deps.Add("item1_1_2", "item1_1_2_1")
	err := deps.Add("item2", "item1")
	assert.Equal(t, ErrorCircularJobDependency, err)
}

func TestFindTrivialCircle(t *testing.T) {
	deps := make(JobsDeps)
	err := deps.Add("item1", "item1")
	assert.Equal(t, ErrorCircularJobDependency, err)
}

func TestCannotRepeatParent(t *testing.T) {
	deps := make(JobsDeps)
	deps.Add("item1", "item2")
	deps.Add("item1", "item3")
	err := deps.Add("item1", "item2")
	assert.Equal(t, ErrorDuplicateDependency, err)
	assert.Equal(t, 2, len(deps["item1"]))
}
