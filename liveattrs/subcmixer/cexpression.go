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

import (
	"fmt"
	"strings"
)

var (
	operators = map[string]string{"==": "<>", "<>": "==", "<=": ">=", ">=": "<="}
)

type CategoryExpression struct {
	attr  string
	Op    string
	value string
}

func (ce *CategoryExpression) String() string {
	return fmt.Sprintf("%s %s '%s'", ce.attr, ce.Op, ce.value)
}

func (ce *CategoryExpression) Negate() AbstractExpression {
	ans, _ := NewCategoryExpression(ce.attr, operators[ce.Op], ce.value)
	return ans
}

func (ce *CategoryExpression) IsComposed() bool {
	return false
}

func (ce *CategoryExpression) GetAtoms() []AbstractAtomicExpression {
	return []AbstractAtomicExpression{ce}
}

func (ce *CategoryExpression) IsEmpty() bool {
	return ce.attr == "" && ce.Op == "" && ce.value == ""
}

func (ce *CategoryExpression) Add(other AbstractExpression) {
	panic("adding value to a non-composed expression type CategoryExpression")
}

func (ce *CategoryExpression) OpSQL() string {
	if ce.Op == "==" {
		return "="
	}
	return ce.Op
}

func (ce *CategoryExpression) Attr() string {
	return ce.attr
}

func (ce *CategoryExpression) Value() string {
	return ce.value
}

func NewCategoryExpression(attr, op, value string) (*CategoryExpression, error) {
	_, ok := operators[op]
	if !ok {
		return &CategoryExpression{}, fmt.Errorf("invalid operator: %s", op)
	}
	return &CategoryExpression{
		attr:  strings.Replace(attr, ".", "_", 1),
		Op:    op,
		value: value,
	}, nil
}
