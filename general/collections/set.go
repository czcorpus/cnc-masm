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

package collections

import (
	"sort"

	"golang.org/x/exp/constraints"
)

type Set[T constraints.Ordered] struct {
	data map[T]bool
}

func (set *Set[T]) Add(value T) {
	set.data[value] = true
}

func (set *Set[T]) Remove(value T) {
	delete(set.data, value)
}

func (set *Set[T]) Contains(value T) bool {
	_, ok := set.data[value]
	return ok
}

func (set *Set[T]) ToSlice() []T {
	ans := make([]T, 0, len(set.data))
	for k := range set.data {
		ans = append(ans, k)
	}
	return ans
}

func (set *Set[T]) ToOrderedSlice() []T {
	ans := set.ToSlice()
	sort.Slice(
		ans,
		func(i, j int) bool {
			return ans[i] < ans[j]
		},
	)
	return ans
}

func (set *Set[T]) ForEach(fn func(item T)) {
	for k := range set.data {
		fn(k)
	}
}

func (set *Set[T]) Union(other Set[T]) *Set[T] {
	ans := NewSet(set.ToSlice()...)
	other.ForEach(func(item T) {
		ans.Add(item)
	})
	return ans
}

func NewSet[T constraints.Ordered](values ...T) *Set[T] {
	ans := Set[T]{data: make(map[T]bool)}
	for _, v := range values {
		ans.data[v] = true
	}
	return &ans
}
