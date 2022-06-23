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

package cache

import (
	"masm/v3/liveattrs/request/query"
	"masm/v3/liveattrs/request/response"
	"strings"
)

func mkKey(corpusID string, aligned []string) string {
	return strings.Join(append(aligned, corpusID), ":")
}

// EmptyQueryCache provides caching for any query with attributes empty.
// It is perfectly OK to Get/Set any query but only the ones with attributes
// empty will be actually stored. For other ones, nil is always returned by Get.
type EmptyQueryCache struct {
	data map[string]*response.QueryAns
}

func (qc *EmptyQueryCache) Get(corpusID string, qry query.Payload) *response.QueryAns {
	if len(qry.Attrs) > 0 {
		return nil
	}
	return qc.data[mkKey(corpusID, qry.Aligned)]
}

func (qc *EmptyQueryCache) Set(corpusID string, qry query.Payload, value *response.QueryAns) {
	if len(qry.Attrs) > 0 {
		return
	}
	qc.data[mkKey(corpusID, qry.Aligned)] = value
}

func NewEmptyQueryCache() *EmptyQueryCache {
	return &EmptyQueryCache{
		data: make(map[string]*response.QueryAns),
	}
}
