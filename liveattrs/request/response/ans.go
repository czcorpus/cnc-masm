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

package response

import (
	"encoding/json"
	"masm/v3/general/collections"
	"sort"
	"strings"
)

type ListedValue struct {
	ID         string
	Label      string
	ShortLabel string
	Count      int
	Grouping   int
}

type SummarizedValue struct {
	Length int `json:"length"`
}

type QueryAns struct {
	Poscount       int
	AttrValues     map[string]any
	AlignedCorpora []string
}

func (qa *QueryAns) MarshalJSON() ([]byte, error) {
	expAllAttrValues := make(map[string]any)
	for k, v := range qa.AttrValues {
		var attrValues any
		tv, ok := v.([]*ListedValue)
		if ok {
			tAttrValues := make([][5]any, 0, len(qa.AttrValues))
			for _, item := range tv {
				tAttrValues = append(
					tAttrValues,
					[5]any{
						item.ShortLabel,
						item.ID,
						item.Label,
						item.Grouping,
						item.Count,
					},
				)
			}
			attrValues = tAttrValues

		} else {
			attrValues = v
		}
		expAllAttrValues[k] = attrValues

	}
	return json.Marshal(&struct {
		Poscount       int            `json:"poscount"`
		AttrValues     map[string]any `json:"attr_values"`
		AlignedCorpora []string       `json:"aligned"`
	}{
		Poscount:       qa.Poscount,
		AttrValues:     expAllAttrValues,
		AlignedCorpora: qa.AlignedCorpora,
	})
}

func (qa *QueryAns) AddListedValue(attr string, v *ListedValue) {
	entry := qa.AttrValues[attr]
	tEntry, ok := entry.([]*ListedValue)
	if ok {
		qa.AttrValues[attr] = append(tEntry, v)
	}
}

func exportKey(k string) string {
	if k == "corpus_id" {
		return k
	}
	return strings.Replace(k, "_", ".", 1)
}

func ExportAttrValues(
	data QueryAns,
	alignedCorpora []string,
	expandAttrs []string,
	collatorLocale string,
	maxAttrListSize int,
) {
	values := make(map[string]any)
	for k, v := range data.AttrValues {
		tVal, ok := v.([]*ListedValue)
		if ok {
			if maxAttrListSize == 0 || len(tVal) < maxAttrListSize ||
				collections.SliceContains(expandAttrs, k) {
				sort.Slice(
					tVal,
					func(i, j int) bool {
						return strings.Compare(tVal[i].Label, tVal[j].Label) == -1
					},
				)
				values[exportKey(k)] = tVal

			} else if tVal, ok := v.(int); ok {
				values[exportKey(k)] = SummarizedValue{Length: tVal}
			}

		} else {
			values[exportKey(k)] = v
		}
	}
	data.AttrValues = values
}
