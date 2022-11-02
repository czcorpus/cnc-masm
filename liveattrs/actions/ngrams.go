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
	"errors"
	"fmt"
	"masm/v3/api"
	"masm/v3/jobs"
	"masm/v3/liveattrs/db/freqdb"
	"masm/v3/liveattrs/laconf"
	"masm/v3/liveattrs/qs"
	"net/http"
	"strconv"

	"github.com/czcorpus/vert-tagextract/v2/cnf"
	"github.com/czcorpus/vert-tagextract/v2/ptcount/modders"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

var (
	errorPosNotDefined = errors.New("PoS not defined")
)

func appendPosModder(prev, curr string) string {
	if prev == "" {
		return curr
	}
	return prev + ":" + curr
}

// posExtractorFactory creates a proper modders.StringTransformer instance
// to extract PoS in MASM and also a string representation of it for proper
// vert-tagexract configuration.
func posExtractorFactory(
	currMods string,
	tagsetName string,
) (*modders.StringTransformerChain, string) {
	modderSpecif := appendPosModder(currMods, tagsetName)
	return modders.NewStringTransformerChain(modderSpecif), modderSpecif
}

// applyPosProperties takes posIdx and posTagset and adds a column modder
// to Ngrams.columnMods column matching the "PoS" one (preserving string modders
// already configured there!).
func applyPosProperties(
	conf *cnf.VTEConf,
	posIdx int, posTagset string,
) (*modders.StringTransformerChain, error) {
	for i, col := range conf.Ngrams.AttrColumns {
		if posIdx == col {
			fn, modderSpecif := posExtractorFactory(conf.Ngrams.ColumnMods[i], posTagset)
			conf.Ngrams.ColumnMods[i] = modderSpecif
			return fn, nil
		}
	}
	return modders.NewStringTransformerChain(""), errorPosNotDefined
}

func (a *Actions) GenerateNgrams(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	corpusID := vars["corpusId"]
	baseErrTpl := fmt.Sprintf("failed to generate n-grams for %s", corpusID)
	// PosColumnIdx defines a vertical column number (starting from zero)
	// where PoS can be extracted. In case no direct "pos" tag exists,
	// a "tag" can be used along with a proper "transformFn" defined for
	// in the data extraction configuration ("vertColumns" section).
	// Also note that the value must be present in the "vertColumns" section
	// otherwise, the action produces an error
	posColumnIdx, err := strconv.Atoi(req.URL.Query().Get("posColIdx"))
	if err != nil {
		api.WriteJSONErrorResponse(
			w,
			api.NewActionErrorFrom("invalid value for posColIdx", err),
			http.StatusBadRequest)
		return
	}

	posTagset := req.URL.Query().Get("posTagset")
	if posTagset == "" {
		api.WriteJSONErrorResponse(
			w, api.NewActionError("missing URL argument posTagset"), http.StatusBadRequest)
		return
	}

	laConf, err := a.laConfCache.Get(corpusID)
	if err == laconf.ErrorNoSuchConfig {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusNotFound)
		return

	} else if err != nil {
		api.WriteJSONErrorResponse(
			w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusInternalServerError)
		return
	}
	posFn, err := applyPosProperties(laConf, posColumnIdx, posTagset)

	corpusDBInfo, err := a.cncDB.LoadInfo(corpusID)
	if err != nil {
		api.WriteJSONErrorResponse(
			w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusInternalServerError)
		return
	}

	generator := freqdb.NewNgramFreqGenerator(
		a.laDB,
		a.jobActions,
		corpusDBInfo.GroupedName(),
		corpusDBInfo.Name,
		posFn,
	)
	jobInfo, _, err := generator.GenerateAsync(corpusID)
	if err != nil {
		api.WriteJSONErrorResponse(
			w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusInternalServerError)
		return
	}
	api.WriteJSONResponse(w, jobInfo.FullInfo())
}

func (a *Actions) CreateQuerySuggestions(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	corpusID := vars["corpusId"]
	baseErrTpl := fmt.Sprintf("failed to generate query suggestions for %s", corpusID)
	multiValuesEnabled := req.URL.Query().Get("multiValuesEnabled") == "1"

	corpusDBInfo, err := a.cncDB.LoadInfo(corpusID)
	if err != nil {
		api.WriteJSONErrorResponse(
			w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusInternalServerError)
		return
	}
	exporter := qs.NewExporter(
		&a.conf.NgramDB,
		a.laDB,
		corpusDBInfo.GroupedName(),
		a.conf.NgramDB.ReadAccessUsers,
		multiValuesEnabled,
		a.jobActions,
	)
	jobInfo, _, err := exporter.RunAsyncExportJob(nil)
	if err != nil {
		api.WriteJSONErrorResponse(
			w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusInternalServerError)
		return
	}
	api.WriteJSONResponse(w, jobInfo)
}

func (a *Actions) CreateNgramsAndQuerySuggestions(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	corpusID := vars["corpusId"]
	multiValuesEnabled := req.URL.Query().Get("multiValuesEnabled") == "1"
	baseErrTpl := fmt.Sprintf("failed to generate n-grams and query suggestions for %s", corpusID)
	// PosColumnIdx defines a vertical column number (starting from zero)
	// where PoS can be extracted. In case no direct "pos" tag exists,
	// a "tag" can be used along with a proper "transformFn" defined for
	// in the data extraction configuration ("vertColumns" section).
	// Also note that the value must be present in the "vertColumns" section
	// otherwise, the action produces an error
	posColumnIdx, err := strconv.Atoi(req.URL.Query().Get("posColIdx"))
	if err != nil {
		api.WriteJSONErrorResponse(
			w,
			api.NewActionErrorFrom("invalid value for posColIdx", err),
			http.StatusBadRequest)
		return
	}

	posTagset := req.URL.Query().Get("posTagset")
	if posTagset == "" {
		api.WriteJSONErrorResponse(
			w, api.NewActionError("missing URL argument posTagset"), http.StatusBadRequest)
		return
	}

	laConf, err := a.laConfCache.Get(corpusID)
	if err == laconf.ErrorNoSuchConfig {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusNotFound)
		return

	} else if err != nil {
		api.WriteJSONErrorResponse(
			w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusInternalServerError)
		return
	}
	posFn, err := applyPosProperties(laConf, posColumnIdx, posTagset)

	corpusDBInfo, err := a.cncDB.LoadInfo(corpusID)
	if err != nil {
		api.WriteJSONErrorResponse(
			w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusInternalServerError)
		return
	}

	// prepare new job structure
	jobID, err := uuid.NewUUID()
	if err != nil {
		api.WriteJSONErrorResponse(
			w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusInternalServerError)
		return
	}
	jobInfo := NgramsAndQSJobInfo{
		ID:       jobID.String(),
		Type:     "ngram-and-qs-generating",
		CorpusID: corpusID,
		Start:    jobs.CurrentDatetime(),
		Update:   jobs.CurrentDatetime(),
		Finished: false,
		Args:     NgramsAndQSJobInfoArgs{},
	}

	// prepare ngrams generator job
	generator := freqdb.NewNgramFreqGenerator(
		a.laDB,
		a.jobActions,
		corpusDBInfo.GroupedName(),
		corpusDBInfo.Name,
		posFn,
	)
	ngramsJobInfo, updateNgramsJobChan, err := generator.GenerateAsync(corpusID)
	if err != nil {
		api.WriteJSONErrorResponse(
			w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusInternalServerError)
		return
	}
	jobInfo.Result.NgramsJob = SubJobInfo{
		ID:       ngramsJobInfo.ID,
		Running:  false,
		Finished: ngramsJobInfo.Finished,
	}

	// prepare query suggestions export job
	exporter := qs.NewExporter(
		&a.conf.NgramDB,
		a.laDB,
		corpusDBInfo.GroupedName(),
		a.conf.NgramDB.ReadAccessUsers,
		multiValuesEnabled,
		a.jobActions,
	)
	ngramsReadyChan := make(chan bool, 1) // channel to run export after generation is done
	qsJobInfo, updateExportJobChan, err := exporter.RunAsyncExportJob(ngramsReadyChan)
	if err != nil {
		api.WriteJSONErrorResponse(
			w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusInternalServerError)
		return
	}
	jobInfo.Result.ExportJob = SubJobInfo{
		ID:       qsJobInfo.ID,
		Running:  false,
		Finished: qsJobInfo.Finished,
	}

	// handle multijob
	updateJobChan := a.jobActions.AddJobInfo(&jobInfo)
	go func(jobInfo NgramsAndQSJobInfo) {
		for subJobUpd := range updateNgramsJobChan {
			jobInfo.Result.NgramsJob.Running = !subJobUpd.IsFinished()
			jobInfo.Result.NgramsJob.Finished = subJobUpd.IsFinished()
			jobInfo.Error = subJobUpd.GetError()
			jobInfo.Update = jobs.CurrentDatetime()
			updateJobChan <- &jobInfo
		}
		if jobInfo.Error != nil {
			ngramsReadyChan <- false
		} else {
			ngramsReadyChan <- true
		}
		for subJobUpd := range updateExportJobChan {
			jobInfo.Result.ExportJob.Running = !subJobUpd.IsFinished()
			jobInfo.Result.ExportJob.Finished = subJobUpd.IsFinished()
			jobInfo.Error = subJobUpd.GetError()
			jobInfo.Update = jobs.CurrentDatetime()
			updateJobChan <- &jobInfo
		}
		jobInfo.Update = jobs.CurrentDatetime()
		jobInfo.Finished = true
		updateJobChan <- &jobInfo
		close(updateJobChan)
	}(jobInfo)

	api.WriteJSONResponse(w, jobInfo)
}
