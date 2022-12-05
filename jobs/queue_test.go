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

func TestEnqueue(t *testing.T) {
	q := JobQueue{}
	f1 := func(chan<- GeneralJobInfo) {}
	f2 := func(chan<- GeneralJobInfo) {}
	f3 := func(chan<- GeneralJobInfo) {}
	q.Enqueue(&f1, &DummyJobInfo{ID: "1"})
	q.Enqueue(&f2, &DummyJobInfo{ID: "2"})
	q.Enqueue(&f3, &DummyJobInfo{ID: "3"})
	assert.Equal(t, &f1, q.firstEntry.job)
	assert.Equal(t, "1", q.firstEntry.initialState.GetID())
	assert.Equal(t, &f3, q.lastEntry.job)
	assert.Equal(t, "3", q.lastEntry.initialState.GetID())
	assert.Equal(t, 3, q.Size())
}

func TestDequeueOne(t *testing.T) {
	q := JobQueue{}
	f1 := func(chan<- GeneralJobInfo) {}
	f2 := func(chan<- GeneralJobInfo) {}
	f3 := func(chan<- GeneralJobInfo) {}
	q.Enqueue(&f1, &DummyJobInfo{ID: "1"})
	q.Enqueue(&f2, &DummyJobInfo{ID: "2"})
	q.Enqueue(&f3, &DummyJobInfo{ID: "3"})
	ans, st, err := q.Dequeue()
	assert.NoError(t, err)
	assert.Equal(t, &f1, ans)
	assert.Equal(t, "1", st.GetID())
	assert.Equal(t, 2, q.Size())
}

func TestDequeueAll(t *testing.T) {
	q := JobQueue{}
	var err error
	f1 := func(chan<- GeneralJobInfo) {}
	f2 := func(chan<- GeneralJobInfo) {}
	f3 := func(chan<- GeneralJobInfo) {}

	q.Enqueue(&f1, &DummyJobInfo{ID: "1"})
	q.Enqueue(&f2, &DummyJobInfo{ID: "2"})
	q.Enqueue(&f3, &DummyJobInfo{ID: "3"})
	_, _, err = q.Dequeue()
	assert.NoError(t, err)
	_, _, err = q.Dequeue()
	assert.NoError(t, err)
	var f *QueuedFunc
	var st GeneralJobInfo
	f, st, err = q.Dequeue()
	assert.NoError(t, err)
	assert.Equal(t, &f3, f)
	assert.Equal(t, "3", st.GetID())
	assert.Equal(t, 0, q.Size())
}

func TestRepeatedlyEmptied(t *testing.T) {
	q := JobQueue{}
	f1 := func(chan<- GeneralJobInfo) {}
	f2 := func(chan<- GeneralJobInfo) {}
	f3 := func(chan<- GeneralJobInfo) {}

	q.Enqueue(&f1, &DummyJobInfo{ID: "1"})
	q.Enqueue(&f2, &DummyJobInfo{ID: "2"})
	q.Dequeue()
	q.Dequeue()
	q.Enqueue(&f3, &DummyJobInfo{ID: "3"})
	assert.Equal(t, 1, q.Size())
	assert.Equal(t, &f3, q.firstEntry.job)
	assert.Equal(t, "3", q.firstEntry.initialState.GetID())
	assert.Equal(t, &f3, q.lastEntry.job)
	assert.Equal(t, "3", q.lastEntry.initialState.GetID())
}

func TestDequeueOnEmpty(t *testing.T) {
	q := JobQueue{}
	_, _, err := q.Dequeue()
	assert.Equal(t, ErrorEmptyQueue, err)
}

func TestDelayNextOnEmpty(t *testing.T) {
	q := JobQueue{}
	err := q.DelayNext()
	assert.Equal(t, err, ErrorEmptyQueue)
}
func TestDelayNextOnTwoItemQueue(t *testing.T) {
	q := JobQueue{}
	f1 := func(chan<- GeneralJobInfo) {}
	f2 := func(chan<- GeneralJobInfo) {}
	q.Enqueue(&f1, &DummyJobInfo{ID: "1"})
	q.Enqueue(&f2, &DummyJobInfo{ID: "2"})
	err := q.DelayNext()
	assert.NoError(t, err)
	assert.Equal(t, &f1, q.lastEntry.job)
	assert.Equal(t, &f2, q.firstEntry.job)
	v, st, err := q.Dequeue()
	assert.Equal(t, "2", st.GetID())
	assert.Equal(t, &f2, v)
	assert.NoError(t, err)
}
