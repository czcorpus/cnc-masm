// Copyright 2019 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2019 Institute of the Czech National Corpus,
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

package corpus

import (
	"database/sql"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/czcorpus/cnc-gokit/uniresp"
)

type CorpusInfoProvider interface {
	LoadInfo(corpusID string) (*DBInfo, error)
}

// Actions contains all the server HTTP REST actions
type Actions struct {
	conf         *CorporaSetup
	osSignal     chan os.Signal
	infoProvider CorpusInfoProvider
}

// GetCorpusInfo provides some basic information about stored data
func (a *Actions) GetCorpusInfo(ctx *gin.Context) {
	var err error
	corpusID := ctx.Param("corpusId")
	baseErrTpl := "failed to get corpus info for %s: %w"
	dbInfo, err := a.infoProvider.LoadInfo(corpusID)

	if err == sql.ErrNoRows {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusNotFound)
		log.Error().Err(err)
		return

	} else if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		log.Error().Err(err)
		return
	}
	ans, err := GetCorpusInfo(corpusID, a.conf, dbInfo.HasLimitedVariant)
	if err == CorpusNotFound {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusNotFound)
		log.Error().Err(err)
		return

	} else if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		log.Error().Err(err)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, ans)
}

// NewActions is the default factory
func NewActions(
	conf *CorporaSetup,
	infoProvider CorpusInfoProvider,
) *Actions {
	return &Actions{
		conf:         conf,
		infoProvider: infoProvider,
	}
}
