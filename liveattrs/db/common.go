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

package db

import (
	"errors"
	"fmt"
	"strings"
)

// ErrorEmptyResult is a general representation
// of "nothing found" for any liveattrs db operation.
// It is up to a concrete implementation whether this
// applies for multi-row return values too.
var ErrorEmptyResult = errors.New("no result")

type StructAttr struct {
	Struct string
	Attr   string
}

func (sattr StructAttr) Values() [2]string {
	return [2]string{sattr.Struct, sattr.Attr}
}

func (sattr StructAttr) Key() string {
	return fmt.Sprintf("%s.%s", sattr.Struct, sattr.Attr)
}

// --

func ImportStructAttr(v string) StructAttr {
	tmp := strings.Split(v, ".")
	return StructAttr{Struct: tmp[0], Attr: tmp[1]}
}
