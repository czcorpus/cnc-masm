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

package cncdb

import (
	"database/sql"
	"fmt"
	"masm/v3/corpus"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/czcorpus/cnc-gokit/uniresp"
)

// DataHandler describes functions expected from
// CNC information system database as needed by KonText
// (and possibly other apps).
type DataHandler interface {
	UpdateSize(transact *sql.Tx, corpus string, size int64) error
	UpdateDescription(transact *sql.Tx, corpus, descCs, descEn string) error
	GetSimpleQueryDefaultAttrs(corpus string) ([]string, error)
	GetCorpusTagsetAttrs(corpus string) ([]string, error)
	UpdateDefaultViewOpts(transact *sql.Tx, corpus string, defaultViewOpts DefaultViewOpts) error
	StartTx() (*sql.Tx, error)
	CommitTx(transact *sql.Tx) error
	RollbackTx(transact *sql.Tx) error
}

type updateSizeResp struct {
	OK bool `json:"ok"`
}

// Actions contains all the server HTTP REST actions
type Actions struct {
	conf  *corpus.DatabaseSetup
	cConf *corpus.CorporaSetup
	db    DataHandler
}

// NewActions is the default factory
func NewActions(
	conf *corpus.DatabaseSetup,
	cConf *corpus.CorporaSetup,
	db DataHandler,
) *Actions {
	return &Actions{
		conf:  conf,
		cConf: cConf,
		db:    db,
	}
}

func (a *Actions) UpdateCorpusInfo(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	baseErrTpl := "failed to update info for corpus %s: %w"
	corpusInfo, err := corpus.GetCorpusInfo(corpusID, "", a.cConf)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		return
	}
	if !corpusInfo.IndexedData.Path.FileExists {
		err := fmt.Errorf("data not found for corpus %s", corpusID)
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusNotFound)
		return
	}
	transact, err := a.db.StartTx()
	err = a.db.UpdateSize(transact, corpusID, corpusInfo.IndexedData.Size)
	if err != nil {
		err2 := a.db.RollbackTx(transact)
		if err2 != nil {
			log.Error().Err(err2).Msg("failed to rollback transaction")
		}
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		return
	}

	err = a.db.UpdateDescription(transact, corpusID, ctx.PostForm("description_cs"),
		ctx.PostForm("description_en"))
	if err != nil {
		err2 := a.db.RollbackTx(transact)
		if err2 != nil {
			log.Error().Err(err2).Msg("failed to rollback transaction")
		}
	}

	err = a.db.CommitTx(transact)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
	}
	uniresp.WriteJSONResponse(ctx.Writer, updateSizeResp{OK: true})
}

func (a *Actions) InferKontextDefaults(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")

	defaultViewAttrs, err := a.db.GetSimpleQueryDefaultAttrs(corpusID)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError("Failed to get simple query default attrs: %w", err), http.StatusInternalServerError)
		return
	}
	defaultViewOpts := DefaultViewOpts{
		Attrs: defaultViewAttrs,
	}

	if len(defaultViewOpts.Attrs) == 0 {
		corpusAttrs, err := corpus.GetCorpusAttrs(corpusID, a.cConf)
		if err != nil {
			uniresp.WriteJSONErrorResponse(
				ctx.Writer, uniresp.NewActionError("Failed to get corpus attrs: %w", err), http.StatusInternalServerError)
			return
		}

		defaultViewOpts.Attrs = append(defaultViewOpts.Attrs, "word")
		for _, attr := range corpusAttrs {
			if attr == "lemma" {
				defaultViewOpts.Attrs = append(defaultViewOpts.Attrs, "lemma")
				break
			}
		}
	}

	tagsetAttrs, err := a.db.GetCorpusTagsetAttrs(corpusID)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError("Failed to get corpus tagset attrs: %w", err), http.StatusInternalServerError)
		return
	}
	defaultViewOpts.Attrs = append(defaultViewOpts.Attrs, tagsetAttrs...)

	tx, err := a.db.StartTx()
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError("Failed to start database transaction: %w", err), http.StatusInternalServerError)
		return
	}
	err = a.db.UpdateDefaultViewOpts(tx, corpusID, defaultViewOpts)
	if err != nil {
		tx.Rollback()
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError("Failed to update `default_view_opts`: %w", err), http.StatusInternalServerError)
		return
	}
	tx.Commit()

	uniresp.WriteJSONResponse(ctx.Writer, defaultViewOpts)
}
