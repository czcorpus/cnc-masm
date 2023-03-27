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

	"github.com/rs/zerolog/log"
)

func mkKey(corpusID string, aligned []string) string {
	return strings.Join(append(aligned, corpusID), ":")
}

// EmptyQueryCache provides caching for any query with attributes empty.
// It is perfectly OK to Get/Set any query but only the ones with attributes
// empty will be actually stored. For other ones, nil is always returned by Get.
type EmptyQueryCache struct {

	// data contains cached results for initial corpus+aligned corpora text types listings
	data map[string]*response.QueryAns

	// corpKeyDeps maps corpus ID to cache keys it is involved in.
	// This allows us removing all the affected results once a single corpus
	// changes
	corpKeyDeps map[string][]string

	lock sync.Mutex
}

// Get returns a cached result based on provided corpus (and possible aligned corpora)
// In case nothing is found, nil is returned
func (qc *EmptyQueryCache) Get(corpusID string, qry query.Payload) *response.QueryAns {
	if len(qry.Attrs) > 0 {
		return nil
	}
	return qc.data[mkKey(corpusID, qry.Aligned)]
}

// setKeyCorpusDependency create a dependency between corpus and cache key
func (qc *EmptyQueryCache) setKeyCorpusDependency(corpusID, key string) {
	keys, ok := qc.corpKeyDeps[corpusID]
	if !ok {
		qc.corpKeyDeps[corpusID] = []string{key}

	} else {
		for _, k := range keys {
			if k == key {
				return // already linked
			}
		}
		qc.corpKeyDeps[corpusID] = append(qc.corpKeyDeps[corpusID], key)
	}
}

func (qc *EmptyQueryCache) Set(corpusID string, qry query.Payload, value *response.QueryAns) {
	if len(qry.Attrs) > 0 {
		return
	}
	qc.lock.Lock()
	cKey := mkKey(corpusID, qry.Aligned)
	qc.data[cKey] = value
	qc.setKeyCorpusDependency(corpusID, cKey)
	for _, alignedCorpusID := range qry.Aligned {
		qc.setKeyCorpusDependency(alignedCorpusID, cKey)
	}
	qc.lock.Unlock()
}

// pruneKeyInDeps in corpus key dependency mapping, remove all
// the values of "key". Return number of removed occurrences.
func (qc *EmptyQueryCache) pruneKeyInDeps(key string) int {
	var totalRemoved int
	for corpID, keys := range qc.corpKeyDeps {
		newKeys := make([]string, 0, len(keys))
		for _, k := range keys {
			if k != key {
				newKeys = append(newKeys, k)

			} else {
				totalRemoved++
			}
		}
		qc.corpKeyDeps[corpID] = newKeys
	}
	return totalRemoved
}

func (qc *EmptyQueryCache) Del(corpusID string) {
	qc.lock.Lock()
	cInv := qc.corpKeyDeps[corpusID]
	var totalPruned int
	for _, key := range cInv {
		delete(qc.data, key)
		totalPruned += qc.pruneKeyInDeps(key)
	}
	delete(qc.corpKeyDeps, corpusID)
	log.Info().
		Strs("keys", cInv).
		Str("corpusId", corpusID).
		Int("prunedKeyDeps", totalPruned).
		Msg("Deleting liveattrs cache keys")
	qc.lock.Unlock()
}

func NewEmptyQueryCache() *EmptyQueryCache {
	return &EmptyQueryCache{
		data:        make(map[string]*response.QueryAns),
		corpKeyDeps: make(map[string][]string),
	}
}
