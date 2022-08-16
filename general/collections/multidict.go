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

import "errors"

var ErrorStopIteration = errors.New("stopped iteration")

type Multidict[T any] struct {
	data map[string][]T
}

func (md *Multidict[T]) Add(k string, v T) {
	_, ok := md.data[k]
	if !ok {
		md.data[k] = make([]T, 0, 10)
	}
	md.data[k] = append(md.data[k], v)
}

func (md *Multidict[T]) Get(k string) []T {
	return md.data[k]
}

func (md *Multidict[T]) ForEach(applyFn func(k string, v []T) error) error {
	for k, v := range md.data {
		err := applyFn(k, v)
		if err != nil {
			return err
		}
	}
	return nil
}

func NewMultidict[T any]() *Multidict[T] {
	return &Multidict[T]{
		data: make(map[string][]T),
	}
}
