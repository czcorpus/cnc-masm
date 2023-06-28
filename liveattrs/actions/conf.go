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
	"io"
	"masm/v3/corpus"
	"masm/v3/liveattrs/laconf"
	"net/http"
	"strconv"

	"github.com/czcorpus/cnc-gokit/uniresp"
	vteCnf "github.com/czcorpus/vert-tagextract/v2/cnf"
	"github.com/gin-gonic/gin"
)

func (a *Actions) getJsonArgs(req *http.Request) (*liveattrsJsonArgs, error) {
	var jsonArgs liveattrsJsonArgs
	err := json.NewDecoder(req.Body).Decode(&jsonArgs)
	if err == io.EOF {
		err = nil
	}
	return &jsonArgs, err
}

// createConf creates a data extraction configuration
// (for vert-tagextract library) based on provided corpus
// (= effectively a vertical file) and request data.
// Please note that JSON data provided in request body
// can be understood either as a "transient" parameters
// for a single job or they can be saved along with other
// parameters to the returned vteCnf.VTEConf value. For the
// transient mode, *liveattrsJsonArgs can be used to access
// provided values.
func (a *Actions) createConf(
	corpusID string,
	req *http.Request,
	saveJSONArgs bool,
	maxNumErr int,
) (*vteCnf.VTEConf, *liveattrsJsonArgs, error) {
	corpusInfo, err := corpus.GetCorpusInfo(corpusID, "", a.conf.Corp)
	if err != nil {
		return nil, nil, err
	}
	corpusDBInfo, err := a.cncDB.LoadInfo(corpusID)
	if err != nil {
		return nil, nil, err
	}
	maxNumErrReq := req.URL.Query().Get("maxNumErrors")
	if maxNumErrReq != "" {
		maxNumErr, err = strconv.Atoi(maxNumErrReq)
		if err != nil {
			return nil, nil, err
		}
	}

	jsonArgs, err := a.getJsonArgs(req)
	if err != nil {
		return nil, nil, err
	}

	conf, err := laconf.Create(
		a.conf.LA,
		corpusInfo,
		corpusDBInfo,
		req.URL.Query().Get("atomStructure"),
		req.URL.Query().Get("bibIdAttr"),
		req.URL.Query()["mergeAttr"],
		req.URL.Query().Get("mergeFn"), // e.g. "identity", "intecorp"
		maxNumErr,
	)

	a.applyNgramConf(conf, jsonArgs)

	return conf, jsonArgs, err
}

func (a *Actions) ViewConf(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	baseErrTpl := "failed to get liveattrs conf for %s: %w"
	conf, err := a.laConfCache.GetWithoutPasswords(corpusID)
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
	newConf, _, err := a.createConf(corpusID, ctx.Request, true, a.conf.LA.VertMaxNumErrors)
	if err != nil {
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
	uniresp.WriteJSONResponse(ctx.Writer, newConf)
}
