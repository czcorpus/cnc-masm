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
	"testing"

	"github.com/stretchr/testify/assert"
)

func createTestingCache() (*EmptyQueryCache, query.Payload, response.QueryAns) {
	qcache := NewEmptyQueryCache()
	qry := query.Payload{
		Aligned: []string{"corp2", "corp3"},
	}
	value := response.QueryAns{
		AlignedCorpora: []string{"corp2", "corp3"},
		AttrValues: map[string]any{
			"attrA": []string{"valA1", "valA2"},
			"attrB": []string{"valB1", "valB2", "valB3"},
		},
	}
	qcache.Set("corp1", qry, &value)
	return qcache, qry, value
}

func TestCacheSet(t *testing.T) {
	qcache, qry, value := createTestingCache()
	v := qcache.Get("corp1", qry)
	assert.Equal(t, *v, value)
	assert.Contains(t, qcache.corpInvolvements, "corp1")
	assert.Contains(t, qcache.corpInvolvements, "corp2")
	assert.Contains(t, qcache.corpInvolvements, "corp3")
}

func TestCacheDel(t *testing.T) {
	qcache, qry, _ := createTestingCache()
	qry2 := query.Payload{
		Aligned: []string{"corp1"},
	}
	value := response.QueryAns{
		AlignedCorpora: []string{"corp1"},
		AttrValues: map[string]any{
			"attrA": []string{"valA1", "valA2"},
		},
	}
	qcache.Set("corp4", qry2, &value)

	qcache.Del("corp1")
	assert.Nil(t, qcache.Get("corp1", qry))
	assert.NotContains(t, qcache.corpInvolvements, "corp1")
	assert.Contains(t, qcache.corpInvolvements, "corp2")
	assert.Contains(t, qcache.corpInvolvements, "corp3")
	assert.Equal(t, 0, len(qcache.data))
}

func TestCacheDelAligned(t *testing.T) {
	qcache, _, _ := createTestingCache()
	qcache.Del("corp1")
	qcache.Del("corp2")
	qcache.Del("corp3")
	assert.Equal(t, 0, len(qcache.data))
	assert.Equal(t, 0, len(qcache.corpInvolvements))
}
