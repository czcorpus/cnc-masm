// Copyright 2023 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2023 Institute of the Czech National Corpus,
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

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"path/filepath"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/czcorpus/cnc-gokit/collections"
)

const (
	DfltConcBackoffInitialInterval = 200 * time.Millisecond
	DfltConcBackoffMaxElapsedTime  = 2 * time.Minute
)

var (
	ErrEntryNotFound    = errors.New("cache entry not found")
	ErrEntryNotReadyYet = errors.New("cache entry not ready yet")
)

type CacheEntry struct {
	PromisedAt  time.Time
	FulfilledAt time.Time
	FilePath    string
	Err         error
}

type Cache struct {
	data              *collections.ConcurrentMap[string, CacheEntry]
	loc               *time.Location
	rootPath          string
	maxEntriesPerCorp int
	nextListenerId    int
	waitLimit         time.Duration
	waitCheckInterval time.Duration
}

func (cache *Cache) mkKey(corpusID, query string) string {
	enc := sha1.New()
	enc.Write([]byte(corpusID))
	enc.Write([]byte(query))
	return hex.EncodeToString(enc.Sum(nil))
}

func (cache *Cache) mkPath(corpusID, query string) string {
	return filepath.Join(cache.rootPath, corpusID, cache.mkKey(corpusID, query))
}

func (cache *Cache) Contains(corpusID, query string) bool {
	return cache.data.HasKey(cache.mkKey(corpusID, query))
}

func (cache *Cache) Promise(corpusID, query string, fn func(path string) error) <-chan CacheEntry {
	targetPath := cache.mkPath(corpusID, query)
	entry := CacheEntry{
		PromisedAt: time.Now().In(cache.loc),
		FilePath:   targetPath,
	}
	entryKey := cache.mkKey(corpusID, query)
	cache.data.Set(entryKey, entry)
	ans := make(chan CacheEntry)
	go func(entry2 CacheEntry) {
		err := fn(targetPath)
		if err != nil {
			entry2.Err = err
		}
		entry2.FulfilledAt = time.Now().In(cache.loc)
		cache.data.Set(entryKey, entry2)
		ans <- entry2
		close(ans)
	}(entry)
	return ans
}

func (cache *Cache) Get(corpusID, query string) (CacheEntry, error) {
	entryKey := cache.mkKey(corpusID, query)
	operation := func() (CacheEntry, error) {
		entry, ok := cache.data.GetWithTest(entryKey)
		if !ok {
			e := CacheEntry{
				Err:         ErrEntryNotFound,
				FulfilledAt: time.Now().In(cache.loc),
			}
			return e, backoff.Permanent(ErrEntryNotFound)
		}
		if entry.FulfilledAt.IsZero() {
			entry.Err = ErrEntryNotReadyYet
			return entry, nil
		}
		return entry, nil
	}
	bkoff := backoff.NewExponentialBackOff()
	bkoff.InitialInterval = DfltConcBackoffInitialInterval
	bkoff.MaxElapsedTime = DfltConcBackoffMaxElapsedTime
	return backoff.RetryWithData(operation, bkoff)
}

func NewCache(rootPath string, location *time.Location) *Cache {
	return &Cache{
		rootPath:          rootPath,
		loc:               location,
		data:              collections.NewConcurrentMap[string, CacheEntry](),
		waitCheckInterval: time.Millisecond * 500,
		waitLimit:         time.Second * 10,
	}
}
