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

package query

import "fmt"

type Attrs map[string]any

func (q Attrs) AttrIsRange(attr string) bool {
	v, ok := q[attr]
	if !ok {
		return false
	}
	tv, ok := v.(map[string]string)
	return ok && tv["from"] != "" && tv["to"] != ""
}

func (q Attrs) GetAttrVals(attr string) ([]string, error) {
	v, ok := q[attr]
	if !ok {
		return []string{}, nil
	}
	tv, ok := v.(map[string][]string)
	if !ok {
		return []string{}, fmt.Errorf("attribute %s does not contain value listing", attr)
	}
	return tv[attr], nil
}

type Payload struct {
	Corpname string   `json:"corpname"`
	Aligned  []string `json:"aligned"`
	Attrs    Attrs    `json:"attrs"`
}
