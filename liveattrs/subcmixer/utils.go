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

package subcmixer

import "fmt"

type Maybe[T int | string | bool] struct {
	val   T
	empty bool
}

func (m Maybe[T]) String() string {
	if !m.empty {
		return fmt.Sprintf("%v", m.val)
	}
	return ""
}

func (m Maybe[T]) Empty() bool {
	return m.empty
}

func (m Maybe[T]) Value() (T, bool) {
	return m.val, !m.empty
}

func (m Maybe[T]) Matches(fn func(v T) bool) bool {
	if !m.empty {
		return fn(m.val)
	}
	return false
}

func (m Maybe[T]) Apply(fn func(v T)) {
	if !m.empty {
		fn(m.val)
	}
}

func NewMaybe[T int | string | bool](v T) Maybe[T] {
	return Maybe[T]{val: v, empty: false}
}

func NewEmptyMaybe[T int | string | bool]() Maybe[T] {
	return Maybe[T]{empty: true}
}

// ----

func mapContains[K int | string, V any](m map[K]V, key K) bool {
	_, ok := m[key]
	return ok
}

func sumOfMapped[T any](v []T, mapFn func(item T) float64) float64 {
	var ans float64
	for _, item := range v {
		ans += mapFn(item)
	}
	return ans
}

func min[T int | float64](items ...T) T {
	ans := items[0]
	for i := 1; i < len(items); i++ {
		if items[i] < ans {
			ans = items[i]
		}
	}
	return ans
}

func mapSlice[T any, U any](items []T, mapFn func(T) U) []U {
	ans := make([]U, len(items))
	for i, v := range items {
		ans[i] = mapFn(v)
	}
	return ans
}

func mapSliceToAny[T any](items []T) []any {
	ans := make([]any, len(items))
	for i, v := range items {
		ans[i] = v
	}
	return ans
}

func dotProduct[T int | float64](vec1 []T, vec2 []T) (T, error) {
	if len(vec1) != len(vec2) {
		return -1, fmt.Errorf("vectors must have the same size")
	}
	var ans T
	for i := 0; i < len(vec1); i++ {
		ans += vec1[i] * vec2[i]
	}
	return ans, nil
}

func subtract[T int | float64](items1 []T, items2 []T) ([]T, error) {
	if len(items1) != len(items2) {
		return []T{}, fmt.Errorf("slices must be of the same size")
	}
	ans := make([]T, len(items1))
	for i := 0; i < len(items1); i++ {
		ans[i] = items1[i] - items2[i]
	}
	return ans, nil
}

func indexOf[T int | float64](items []T, srch T) int {
	for i, v := range items {
		if v == srch {
			return i
		}
	}
	return -1
}
