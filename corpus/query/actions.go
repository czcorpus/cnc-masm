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
	"masm/v3/corpus"
	"masm/v3/mango"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/czcorpus/cnc-gokit/fs"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

var (
	collFunc = map[string]byte{
		"absoluteFreq":  'f',
		"LLH":           'l',
		"logDice":       'd',
		"minSens":       's',
		"mutualInf":     'm',
		"mutualInf3":    '3',
		"mutualInfLogF": 'p',
		"relativeFreq":  'r',
		"tScore":        't',
	}
)

type Actions struct {
	conf      *corpus.CorporaSetup
	concCache *Cache
}

func (a *Actions) FreqDistrib(ctx *gin.Context) {
	q := ctx.Request.URL.Query().Get("q")
	log.Debug().
		Str("query", q).
		Msg("processing Mango query")
	flimit := 1
	if ctx.Request.URL.Query().Has("flimit") {
		var err error
		flimit, err = strconv.Atoi(ctx.Request.URL.Query().Get("flimit"))
		if err != nil {
			uniresp.WriteJSONErrorResponse(
				ctx.Writer,
				uniresp.NewActionErrorFrom(err),
				http.StatusUnprocessableEntity,
			)
			return
		}
	}

	var err error
	var corp *mango.GoCorpus
	corp, err = corpus.OpenCorpus(ctx.Param("corpusId"), a.conf)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusUnprocessableEntity,
		)
		return
	}

	var conc *mango.GoConc
	if a.concCache.Contains(ctx.Param("corpusId"), q) {
		cacheEntry, err := a.concCache.Get(ctx.Param("corpusId"), q)
		if cacheEntry.Err != nil {
			err = cacheEntry.Err
			uniresp.WriteJSONErrorResponse(
				ctx.Writer,
				uniresp.NewActionErrorFrom(cacheEntry.Err),
				http.StatusInternalServerError,
			)
			return

		} else {
			conc, err = mango.OpenConcordance(corp, cacheEntry.FilePath)
			if err != nil {
				uniresp.WriteJSONErrorResponse(
					ctx.Writer,
					uniresp.NewActionErrorFrom(err),
					http.StatusInternalServerError,
				)
				return
			}
		}

	} else {
		conc, err = mango.CreateConcordance(corp, q)
		a.concCache.Promise(
			ctx.Param("corpusId"),
			q,
			func(targetPath string) error {
				targetDir := path.Dir(targetPath)
				if !fs.PathExists(targetDir) {
					if err := os.MkdirAll(targetDir, 0755); err != nil {
						return err
					}
				}
				return mango.SaveConcordance(conc, targetPath)
			},
		)
	}

	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError, // TODO the status should be based on err type
		)
		return
	}

	freqs, err := mango.CalcFreqDist(conc, "lemma/e 0~0>0", flimit)
	ans := make([]*FreqDistribItem, len(freqs.Freqs))
	for i, _ := range ans {
		norm := freqs.Norms[i]
		if norm == 0 {
			norm = conc.CorpSize()
		}
		ans[i] = &FreqDistribItem{
			Freq: freqs.Freqs[i],
			Norm: norm,
			IPM:  float32(freqs.Freqs[i]) / float32(norm) * 1e6,
			Word: freqs.Words[i],
		}
	}
	uniresp.WriteJSONResponse(
		ctx.Writer,
		map[string]any{
			"concSize": conc.Size(),
			"freqs":    ans,
		},
	)
}

func (a *Actions) Collocations(ctx *gin.Context) {
	q := ctx.Request.URL.Query().Get("q")
	log.Debug().
		Str("query", q).
		Msg("processing Mango query")

	corp, err := corpus.OpenCorpus(ctx.Param("corpusId"), a.conf)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		return
	}

	conc, err := mango.CreateConcordance(corp, q)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError, // TODO the status should be based on err type
		)
		return
	}
	collFnArg := ctx.Request.URL.Query().Get("fn")
	collFn, ok := collFunc[collFnArg]
	if !ok {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError("unknown collocations function %s", collFnArg),
			http.StatusUnprocessableEntity,
		)
		return
	}
	collocs, err := mango.GetCollcations(conc, "word", collFn, 20, 20)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError, // TODO the status should be based on err type
		)
		return
	}
	uniresp.WriteJSONResponse(
		ctx.Writer,
		map[string]any{
			"collocs": collocs,
		},
	)

}

func NewActions(conf *corpus.CorporaSetup, location *time.Location) *Actions {
	return &Actions{
		conf:      conf,
		concCache: NewCache(conf.ConcCacheDirPath, location),
	}
}
