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

package actions

import (
	"encoding/json"
	"errors"
	"io"
	"masm/v3/liveattrs/db/freqdb"
	"masm/v3/liveattrs/laconf"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/czcorpus/cnc-gokit/uniresp"
)

type reqArgs struct {
	ColMapping freqdb.QSAttributes `json:"colMapping"`

	// PosColIdx defines a vertical column number (starting from zero)
	// where PoS can be extracted. In case no direct "pos" tag exists,
	// a "tag" can be used along with a proper "transformFn" defined
	// in the data extraction configuration ("vertColumns" section).
	PosColIdx int    `json:"posColIdx"` // TODO do we need this?
	PosTagset string `json:"posTagset"`
}

func (args reqArgs) Validate() error {
	if args.PosColIdx < 0 {
		return errors.New("invalid value for posColIdx")
	}
	if args.PosTagset == "" {
		return errors.New("missing posTagset")
	}
	if args.ColMapping.Lemma == "" {
		return errors.New("missing column mapping for lemma")
	}
	if args.ColMapping.Sublemma == "" {
		return errors.New("missing column mapping for sublemma")
	}
	if args.ColMapping.Word == "" {
		return errors.New("missing column mapping for word")
	}
	if args.ColMapping.Tag == "" {
		return errors.New("missing column mapping for tag")
	}
	tmp := make(map[string]int)
	tmp[args.ColMapping.Lemma]++
	tmp[args.ColMapping.Sublemma]++
	tmp[args.ColMapping.Word]++
	tmp[args.ColMapping.Tag]++
	if len(tmp) < 4 {
		return errors.New(
			"each of the lemma, sublemma, word, tag must be mapped to a unique table column")
	}
	return nil
}

func (a *Actions) getNgramArgs(req *http.Request) (reqArgs, error) {
	var jsonArgs reqArgs
	err := json.NewDecoder(req.Body).Decode(&jsonArgs)
	if err == io.EOF {
		err = nil
	}
	return jsonArgs, err
}

func (a *Actions) GenerateNgrams(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	baseErrTpl := "failed to generate n-grams for %s: %w"

	args, err := a.getNgramArgs(ctx.Request)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusBadRequest)
		return
	}
	if err = args.Validate(); err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusUnprocessableEntity)
		return
	}

	laConf, err := a.laConfCache.Get(corpusID)
	if err == laconf.ErrorNoSuchConfig {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusNotFound)
		return

	} else if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		return
	}
	posFn, err := applyPosProperties(laConf, args.PosColIdx, args.PosTagset)

	corpusDBInfo, err := a.cncDB.LoadInfo(corpusID)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		return
	}

	generator := freqdb.NewNgramFreqGenerator(
		a.laDB,
		a.jobActions,
		corpusDBInfo.GroupedName(),
		corpusDBInfo.Name,
		posFn,
		args.ColMapping,
	)
	jobInfo, err := generator.GenerateAfter(corpusID, ctx.Request.URL.Query().Get("parentJobId"))
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, jobInfo.FullInfo())
}
