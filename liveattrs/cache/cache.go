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
	"sync"
)

func mkKey(corpusID string, aligned []string) string {
	return strings.Join(append(aligned, corpusID), ":")
}

// EmptyQueryCache provides caching for any query with attributes empty.
// It is perfectly OK to Get/Set any query but only the ones with attributes
// empty will be actually stored. For other ones, nil is always returned by Get.
type EmptyQueryCache struct {
	data             map[string]*response.QueryAns
	corpInvolvements map[string][]string
	lock             sync.Mutex
}

// Get returns a cached result based on provided corpus (and possible aligned corpora)
// In case nothing is found, nil is returned
func (qc *EmptyQueryCache) Get(corpusID string, qry query.Payload) *response.QueryAns {
	if len(qry.Attrs) > 0 {
		return nil
	}
	return qc.data[mkKey(corpusID, qry.Aligned)]
}

func (qc *EmptyQueryCache) linkCorpusToKey(corpusID, key string) {
	keys, ok := qc.corpInvolvements[corpusID]
	if !ok {
		qc.corpInvolvements[corpusID] = []string{key}

	} else {
		for _, k := range keys {
			if k == key {
				return // already linked
			}
		}
		qc.corpInvolvements[corpusID] = append(qc.corpInvolvements[corpusID], key)
	}
}

func (qc *EmptyQueryCache) Set(corpusID string, qry query.Payload, value *response.QueryAns) {
	if len(qry.Attrs) > 0 {
		return
	}
	qc.lock.Lock()
	cKey := mkKey(corpusID, qry.Aligned)
	qc.data[cKey] = value
	qc.linkCorpusToKey(corpusID, cKey)
	for _, alignedCorpusID := range qry.Aligned {
		qc.linkCorpusToKey(alignedCorpusID, cKey)
	}
	qc.lock.Unlock()
}

func (qc *EmptyQueryCache) Del(corpusID string) {
	qc.lock.Lock()
	cInv := qc.corpInvolvements[corpusID]
	for _, key := range cInv {
		delete(qc.data, key)
	}
	delete(qc.corpInvolvements, corpusID)
	qc.lock.Unlock()
}

func NewEmptyQueryCache() *EmptyQueryCache {
	return &EmptyQueryCache{
		data:             make(map[string]*response.QueryAns),
		corpInvolvements: make(map[string][]string),
	}
}
