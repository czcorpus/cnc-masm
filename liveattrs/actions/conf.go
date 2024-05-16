// Copyright 2020 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2020 Institute of the Czech National Corpus,
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

package actions

import (
	"encoding/json"
	"fmt"
	"io"
	"masm/v3/corpus"
	"masm/v3/liveattrs/laconf"
	"net/http"

	"github.com/czcorpus/cnc-gokit/uniresp"
	vteCnf "github.com/czcorpus/vert-tagextract/v2/cnf"
	"github.com/gin-gonic/gin"
)

func (a *Actions) getJsonArgs(req *http.Request) (*laconf.PatchArgs, error) {
	var jsonArgs laconf.PatchArgs
	err := json.NewDecoder(req.Body).Decode(&jsonArgs)
	if err == io.EOF {
		err = nil
	}
	return &jsonArgs, err
}

// createConf creates a data extraction configuration
// (for vert-tagextract library) based on provided corpus
// (= effectively a vertical file) and request data
// (where it expects JSON version of liveattrsJsonArgs).
func (a *Actions) createConf(
	corpusID string,
	jsonArgs *laconf.PatchArgs,
) (*vteCnf.VTEConf, error) {
	corpusInfo, err := corpus.GetCorpusInfo(corpusID, a.conf.Corp, false)
	if err != nil {
		return nil, err
	}
	corpusDBInfo, err := a.cncDB.LoadInfo(corpusID)
	if err != nil {
		return nil, err
	}

	conf, err := laconf.Create(
		a.conf.LA,
		corpusInfo,
		corpusDBInfo,
		jsonArgs,
	)
	if err != nil {
		return conf, err
	}

	err = a.applyPatchArgs(conf, jsonArgs)
	if err != nil {
		return conf, fmt.Errorf("failed to create conf: %w", err)
	}

	err = a.ensureVerticalFile(conf, corpusInfo)
	if err != nil {
		return conf, fmt.Errorf("failed to create conf: %w", err)
	}
	return conf, err
}

func (a *Actions) ViewConf(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	baseErrTpl := "failed to get liveattrs conf for %s: %w"
	var conf *vteCnf.VTEConf
	var err error
	if ctx.Request.URL.Query().Get("noCache") == "1" {
		conf, err = a.laConfCache.GetUncachedWithoutPasswords(corpusID)

	} else {
		conf, err = a.laConfCache.GetWithoutPasswords(corpusID)
	}
	if err == laconf.ErrorNoSuchConfig {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusNotFound)

	} else if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusBadRequest)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, conf)
}

func (a *Actions) CreateConf(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	baseErrTpl := "failed to create liveattrs config for %s: %w"
	jsonArgs, err := a.getJsonArgs(ctx.Request)
	if err != nil {
		uniresp.RespondWithErrorJSON(
			ctx,
			err,
			http.StatusBadRequest,
		)
	}
	newConf, err := a.createConf(corpusID, jsonArgs)
	if err == ErrorMissingVertical {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusConflict)
		return

	} else if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusBadRequest)
		return
	}
	err = a.laConfCache.Clear(corpusID)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusBadRequest)
		return
	}
	err = a.laConfCache.Save(newConf)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusBadRequest)
		return
	}
	expConf := newConf.WithoutPasswords()
	uniresp.WriteJSONResponse(ctx.Writer, &expConf)
}

func (a *Actions) FlushCache(ctx *gin.Context) {
	ok := a.laConfCache.Uncache(ctx.Param("corpusId"))
	if !ok {
		uniresp.RespondWithErrorJSON(ctx, fmt.Errorf("config not in cache"), http.StatusNotFound)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, map[string]bool{"ok": true})
}

func (a *Actions) PatchConfig(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	conf, err := a.laConfCache.Get(corpusID)
	if err == laconf.ErrorNoSuchConfig {
		uniresp.RespondWithErrorJSON(ctx, fmt.Errorf("no such config"), http.StatusNotFound)
		return
	}

	jsonArgs, err := a.getJsonArgs(ctx.Request)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusBadRequest)
		return
	}
	if jsonArgs == nil {
		uniresp.RespondWithErrorJSON(ctx, fmt.Errorf("no update data provided"), http.StatusBadRequest)
		return
	}
	err = a.applyPatchArgs(conf, jsonArgs)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusBadRequest)
		return
	}

	corpusInfo, err := corpus.GetCorpusInfo(corpusID, a.conf.Corp, false)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}

	err = a.ensureVerticalFile(conf, corpusInfo)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}

	a.laConfCache.Save(conf)
	out := conf.WithoutPasswords()
	uniresp.WriteJSONResponse(ctx.Writer, &out)
}
