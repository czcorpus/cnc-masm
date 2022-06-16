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

package liveattrs

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"masm/v3/api"
	"masm/v3/cncdb"
	"masm/v3/cnf"
	"masm/v3/corpus"
	"masm/v3/general/collections"
	"masm/v3/jobs"
	"masm/v3/liveattrs/qans"
	"masm/v3/liveattrs/qbuilder"
	"masm/v3/liveattrs/query"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	vteCnf "github.com/czcorpus/vert-tagextract/v2/cnf"
	vteDb "github.com/czcorpus/vert-tagextract/v2/db"
	vteLib "github.com/czcorpus/vert-tagextract/v2/library"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

const (
	jobType               = "liveattrs"
	emptyValuePlaceholder = "?"
	dfltMaxAttrListSize   = 50
)

type CreateLiveAttrsReqBody struct {
	Files []string `json:"files"`
}

func loadConf(basePath, corpname string) (*vteCnf.VTEConf, error) {
	return vteCnf.LoadConf(filepath.Join(basePath, fmt.Sprintf("%s.json", corpname)))
}

func installDatabase(corpusID, tmpPath, textTypesDbDirPath string) error {
	dbFileName := corpus.GenCorpusGroupName(corpusID) + ".db"
	absPath := filepath.Join(textTypesDbDirPath, dbFileName)
	srcFile, err := os.Open(tmpPath)
	if err != nil {
		return err
	}
	dstFile, err := os.Create(absPath)
	if err != nil {
		return err
	}
	_, err = io.Copy(dstFile, srcFile)
	return err
}

func arrayShowShortened(data []string) string {
	if len(data) <= 5 {
		return strings.Join(data, ", ")
	}
	ans := make([]string, 5)
	ans[0] = data[0]
	ans[1] = data[1]
	ans[2] = "..."
	ans[3] = data[2]
	ans[4] = data[3]
	return strings.Join(ans, ", ")
}

func groupBibItems(data *qans.QueryAns, bibLabel string) {
	grouping := make(map[string]*qans.ListedValue)
	entry := data.AttrValues[bibLabel]
	tEntry, ok := entry.([]*qans.ListedValue)
	if !ok {
		return
	}
	for _, item := range tEntry {
		val, ok := grouping[item.Label]
		if ok {
			grouping[item.Label].Count += val.Count
			grouping[item.Label].Grouping++

		} else {
			grouping[item.Label] = item
		}
		if grouping[item.Label].Grouping > 1 {
			grouping[item.Label].ID = "@" + grouping[item.Label].Label
		}
	}
	data.AttrValues[bibLabel] = make([]*qans.ListedValue, 0, len(grouping))
	for _, v := range grouping {
		entry, ok := (data.AttrValues[bibLabel]).([]*qans.ListedValue)
		if !ok {
			continue
		}
		data.AttrValues[bibLabel] = append(entry, v)
	}
}

func createLAConfig(corpusInfo *corpus.Info, atomStructure string, bibIdAttr string) *cnf.NewVTEConf {
	newConf := cnf.NewVTEConf{}
	newConf.Corpus = corpusInfo.ID
	newConf.VerticalFile = corpusInfo.RegistryConf.Vertical.Path
	newConf.Encoding = corpusInfo.RegistryConf.Encoding
	newConf.AtomStructure = atomStructure
	newConf.StackStructEval = false
	newConf.Structures = corpusInfo.RegistryConf.SubcorpAttrs
	if bibIdAttr != "" {
		bibView := vteDb.BibViewConf{}
		bibView.IDAttr = bibIdAttr
		for stru, attrs := range corpusInfo.RegistryConf.SubcorpAttrs {
			for _, attr := range attrs {
				bibView.Cols = append(bibView.Cols, fmt.Sprintf("%s_%s", stru, attr))
			}
		}
		newConf.BibView = &bibView
	}
	// TODO
	newConf.Ngrams = nil
	newConf.SelfJoin = nil

	return &newConf
}

// Actions wraps liveattrs-related actions
type Actions struct {
	exitEvent   <-chan os.Signal
	conf        *cnf.Conf
	jobActions  *jobs.Actions
	laConfCache *cnf.LiveAttrsBuildConfLoader
	laDB        *sql.DB
	cncDB       *cncdb.CNCMySQLHandler
}

func (a *Actions) OnExit() {}

func (a *Actions) ViewConf(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	corpusID := vars["corpusId"]
	conf, err := a.laConfCache.Get(corpusID)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionError("error fetching configuration: %s", err), http.StatusBadRequest)
		return
	}
	if conf == nil {
		api.WriteJSONErrorResponse(w, api.NewActionError("Configuration not found"), http.StatusNotFound)
		return
	}
	api.WriteJSONResponse(w, conf)
}

// Create starts a process of creating fresh liveattrs data for a a specified corpus.
// URL args:
// * atomStructure - a minimal structure masm will be able to search for (typically 'doc', 'text')
// * noCache - if '1' then masm regenerates data extraction configuration based on actual corpus
//   registry file
// * bibIdAttr - if defined then masm will create bibliography entries with IDs matching values from
//   from referred bibIdAttr values
func (a *Actions) Create(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	corpusID := vars["corpusId"]
	noCache := false
	if req.URL.Query().Get("noCache") == "1" {
		noCache = true
	}
	var err error
	var conf *vteCnf.VTEConf
	if noCache {
		conf, err = a.laConfCache.GetWithoutCache(corpusID)

	} else {
		conf, err = a.laConfCache.Get(corpusID)
	}

	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionError("Cannot run liveattrs generator: '%s'", err), http.StatusBadRequest)
		return
	} else if conf == nil {
		corpusInfo, err := corpus.GetCorpusInfo(corpusID, "", a.conf.CorporaSetup)
		if err != nil {
			api.WriteJSONErrorResponse(w, api.NewActionError("Cannot run liveattrs generator: '%s'", err), http.StatusBadRequest)
			return
		}

		newConf := createLAConfig(
			corpusInfo,
			req.URL.Query().Get("atomStructure"),
			req.URL.Query().Get("bibIdAttr"),
		)

		err = a.laConfCache.Save(newConf)
		if err != nil {
			api.WriteJSONErrorResponse(w, api.NewActionError("Cannot run liveattrs generator: '%s'", err), http.StatusBadRequest)
			return
		}
		conf, err = a.laConfCache.Get(corpusID)
		if err != nil {
			api.WriteJSONErrorResponse(w, api.NewActionError("Cannot run liveattrs generator: '%s'", err), http.StatusBadRequest)
			return
		}
	}

	// TODO search collisions only in liveattrs type jobs

	jobID, err := uuid.NewUUID()
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionError("Failed to start liveattrs job for '%s'", corpusID), http.StatusUnauthorized)
		return
	}

	if prevRunning := a.jobActions.GetUnfinishedJob(corpusID, jobType); prevRunning != nil {
		api.WriteJSONErrorResponse(
			w,
			api.NewActionError("Cannot run liveattrs generator - the previous job '%s' have not finished yet", prevRunning.GetID()),
			http.StatusConflict,
		)
		return
	}

	status := &JobInfo{
		ID:       jobID.String(),
		CorpusID: corpusID,
		Start:    jobs.CurrentDatetime(),
	}

	procStatus, err := vteLib.ExtractData(conf, false, a.exitEvent)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionError("Cannot run liveattrs generator:", err), http.StatusNotFound)
		return
	}
	go func() {
		updateJobChan := a.jobActions.AddJobInfo(status)
		defer close(updateJobChan)
		var lastErr error
		for upd := range procStatus {
			if upd.Error != nil {
				lastErr = upd.Error
			}
			updateJobChan <- &JobInfo{
				ID:             status.ID,
				Type:           jobType,
				CorpusID:       status.CorpusID,
				Start:          status.Start,
				Update:         jobs.CurrentDatetime(),
				Error:          jobs.NewJSONError(upd.Error),
				ProcessedAtoms: upd.ProcessedAtoms,
				ProcessedLines: upd.ProcessedLines,
			}
		}
		if lastErr != nil {
			return
		}

		if conf.DB.Type == "sqlite" {
			err := installDatabase(conf.Corpus, conf.DB.Name, a.conf.CorporaSetup.TextTypesDbDirPath)
			if err != nil {
				updateJobChan <- &JobInfo{
					ID:       status.ID,
					Type:     jobType,
					CorpusID: status.CorpusID,
					Start:    status.Start,
					Error:    jobs.NewJSONError(err),
				}

			} else {
				resp, err := http.Post(a.conf.KontextSoftResetURL, "application/json", nil)
				if err != nil || resp.StatusCode < 400 {
					updateJobChan <- &JobInfo{
						ID:       status.ID,
						Type:     jobType,
						CorpusID: status.CorpusID,
						Start:    status.Start,
						Error:    jobs.NewJSONError(err),
					}
				}
			}
		}
	}()
	api.WriteJSONResponse(w, status)
}

func (a *Actions) Delete(w http.ResponseWriter, req *http.Request) {

}

func (a *Actions) Query(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	corpusID := vars["corpusId"]
	autocompleteAttr := req.URL.Query().Get("autocomplete_attr")

	var qry query.Query
	err := json.NewDecoder(req.Body).Decode(&qry)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(err), http.StatusBadRequest)
	}
	laConf, err := a.laConfCache.Get(corpusID) // set(self._get_subcorp_attrs(corpus))
	if err != nil {
		api.WriteJSONErrorResponse(
			w,
			api.NewActionError(fmt.Sprintf("corpus %s not supported by liveattrs", corpusID)),
			http.StatusNotFound,
		)
	}
	srchAttrs := collections.NewSet(cnf.GetSubcorpAttrs(laConf)...)
	expandAttrs := collections.NewSet[string]()
	corpInfo, err := a.cncDB.LoadInfo(corpusID)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(err), http.StatusInternalServerError)
	}
	bibLabel := qbuilder.ImportKey(corpInfo.BibLabelAttr)
	if bibLabel != "" {
		srchAttrs.Add(bibLabel)
	}
	// if in autocomplete mode then always expand list of the target column
	if autocompleteAttr != "" {
		a := qbuilder.ImportKey(autocompleteAttr)
		srchAttrs.Add(a)
		expandAttrs.Add(a)
	}
	// also make sure that range attributes are expanded to full lists
	for attr := range qry.Attrs {
		if qry.Attrs.AttrIsRange(attr) {
			expandAttrs.Add(qbuilder.ImportKey(attr))
		}
	}
	qBuilder := &qbuilder.Builder{
		CorpusInfo:          corpInfo,
		AttrMap:             qry.Attrs,
		SearchAttrs:         srchAttrs.ToOrderedSlice(),
		AlignedCorpora:      qry.Aligned,
		AutocompleteAttr:    autocompleteAttr,
		EmptyValPlaceholder: emptyValuePlaceholder,
	}
	dataIterator := qbuilder.DataIterator{
		DB:      a.laDB,
		Builder: qBuilder,
	}

	ans := qans.QueryAns{
		Poscount:   0,
		AttrValues: make(map[string]any),
	}
	for _, sattr := range qBuilder.SearchAttrs {
		ans.AttrValues[sattr] = make([]*qans.ListedValue, 0, 100)
	}
	// 1) values collected one by one are collected in tmp_ans and then moved to 'ans' with some exporting tweaks
	// 2) in case of values exceeding max. allowed list size we just accumulate their size directly to ans[attr]
	// {attr_id: {attr_val: num_positions,...},...}
	tmpAns := make(map[string]map[string]*qans.ListedValue)
	shortenVal := func(v string) string {
		if len(v) > 20 {
			return v[:20] + "..." // TODO !!
		}
		return v
	}
	bibID := qbuilder.ImportKey(qBuilder.CorpusInfo.BibIDAttr)

	err = dataIterator.Iterate(func(row qbuilder.ResultRow) error {
		for dbKey, dbVal := range row.Attrs {
			colKey := qbuilder.ExportKey(dbKey)
			switch tColVal := ans.AttrValues[colKey].(type) {
			case []*qans.ListedValue:
				var valIdent string
				if colKey == bibLabel {
					valIdent = row.Attrs[bibID]

				} else {
					valIdent = row.Attrs[dbKey]
				}
				attrVal := qans.ListedValue{
					ID:         valIdent,
					ShortLabel: shortenVal(dbVal),
					Label:      dbVal,
					Grouping:   1,
				}
				_, ok := tmpAns[colKey]
				if !ok {
					tmpAns[colKey] = make(map[string]*qans.ListedValue)
				}
				currAttrVal, ok := tmpAns[colKey][attrVal.ID]
				if ok {
					currAttrVal.Count += row.Poscount

				} else {
					tmpAns[colKey][attrVal.ID] = &attrVal
				}
			case int:
				ans.AttrValues[colKey] = tColVal + row.Poscount
			default:
				return fmt.Errorf("invalid attr value type for data iterator")
			}
		}
		return nil
	})
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(err), http.StatusInternalServerError)
	}

	for attr, v := range tmpAns {
		for _, c := range v {
			ans.AddListedValue(attr, c)
		}
	}

	// now each line contains: (shortened_label, identifier, label, num_grouped_items, num_positions)
	// where num_grouped_items is initialized to 1
	if corpInfo.BibGroupDuplicates > 0 {
		groupBibItems(&ans, bibLabel)
	}
	qans.ExportAttrValues(
		ans,
		qBuilder.AlignedCorpora,
		expandAttrs.ToOrderedSlice(),
		corpInfo.Locale,
		dfltMaxAttrListSize,
	)
	api.WriteJSONResponse(w, &ans)
}

// NewActions is the default factory for Actions
func NewActions(
	conf *cnf.Conf,
	exitEvent <-chan os.Signal,
	jobActions *jobs.Actions,
	cncDB *cncdb.CNCMySQLHandler,
	laDB *sql.DB,
	version cnf.VersionInfo,
) *Actions {
	return &Actions{
		exitEvent:  exitEvent,
		conf:       conf,
		jobActions: jobActions,
		laConfCache: cnf.NewLiveAttrsBuildConfLoader(
			conf.LiveAttrs.ConfDirPath,
			conf.LiveAttrs.DB,
		),
		cncDB: cncDB,
		laDB:  laDB,
	}
}
