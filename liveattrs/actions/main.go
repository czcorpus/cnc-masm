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
	"database/sql"
	"encoding/json"
	"fmt"
	"masm/v3/api"
	"masm/v3/cncdb"
	"masm/v3/corpus"
	"masm/v3/db/sqlite"
	"masm/v3/general"
	"masm/v3/jobs"
	"masm/v3/liveattrs"
	"masm/v3/liveattrs/cache"
	"masm/v3/liveattrs/db"
	"masm/v3/liveattrs/laconf"
	"masm/v3/liveattrs/request/biblio"
	"masm/v3/liveattrs/request/equery"
	"masm/v3/liveattrs/request/fillattrs"
	"masm/v3/liveattrs/request/query"
	"masm/v3/liveattrs/request/response"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	vteCnf "github.com/czcorpus/vert-tagextract/v2/cnf"
	vteLib "github.com/czcorpus/vert-tagextract/v2/library"
	vteProc "github.com/czcorpus/vert-tagextract/v2/proc"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

const (
	emptyValuePlaceholder = "?"
	dfltMaxAttrListSize   = 30
	shortLabelMaxLength   = 30
)

type CreateLiveAttrsReqBody struct {
	Files []string `json:"files"`
}

func loadConf(basePath, corpname string) (*vteCnf.VTEConf, error) {
	return vteCnf.LoadConf(filepath.Join(basePath, fmt.Sprintf("%s.json", corpname)))
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

// Actions wraps liveattrs-related actions
type Actions struct {
	// exitEvent channel recieves value once user (or OS) terminates masm process
	exitEvent <-chan os.Signal

	// vteExitEvent stores "exit" channels for running vert-tagextract jobs
	// (max 1 per corpus)
	vteExitEvents map[string]chan os.Signal

	// jobStopChannel receives job ID based on user interaction with job HTTP API in
	// case users asks for stopping the vte process
	jobStopChannel <-chan string

	conf *corpus.Conf

	jobActions *jobs.Actions

	laConfCache *laconf.LiveAttrsBuildConfProvider

	// laDB is a live-attributes-specific database where masm needs full privileges
	laDB *sql.DB

	// cncDB is CNC's main database
	cncDB *cncdb.CNCMySQLHandler

	// eqCache stores results for live-attributes empty queries (= initial text types data)
	eqCache *cache.EmptyQueryCache

	structAttrStats *db.StructAttrUsage

	usageData chan<- db.RequestData
}

func (a *Actions) OnExit() {
	close(a.usageData)
}

func (a *Actions) applyNgramConf(targetConf *vteCnf.VTEConf, jsonArgs *liveattrsJsonArgs) {
	if len(jsonArgs.Ngrams.VertColumns) > 0 {
		targetConf.Ngrams.NgramSize = jsonArgs.Ngrams.NgramSize
		targetConf.Ngrams.CalcARF = jsonArgs.Ngrams.CalcARF
		targetConf.Ngrams.AttrColumns = make([]int, len(jsonArgs.Ngrams.VertColumns))
		targetConf.Ngrams.ColumnMods = make([]string, len(jsonArgs.Ngrams.VertColumns))
		targetConf.Ngrams.UniqKeyColumns = make([]int, len(jsonArgs.Ngrams.VertColumns))
		for i, item := range jsonArgs.Ngrams.VertColumns {
			targetConf.Ngrams.AttrColumns[i] = item.Idx
			targetConf.Ngrams.ColumnMods[i] = item.TransformFn
			targetConf.Ngrams.UniqKeyColumns[i] = item.Idx
		}
	}
}

type ngramColumn struct {
	Idx         int    `json:"idx"`
	TransformFn string `json:"transformFn"`
}

type ngramConf struct {
	VertColumns []ngramColumn `json:"vertColumns"`
	NgramSize   int           `json:"ngramSize"`
	CalcARF     bool          `json:"calcARF"`
}

type liveattrsJsonArgs struct {
	VerticalFiles []string  `json:"verticalFiles"`
	Ngrams        ngramConf `json:"ngrams"`
}

func (a *Actions) setSoftResetToKontext() error {
	if a.conf.KontextSoftResetURL == "" {
		log.Warn().Msgf("The kontextSoftResetURL configuration not set - ignoring the action")
		return nil
	}
	resp, err := http.Post(a.conf.KontextSoftResetURL, "application/json", nil)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("kontext soft reset failed - unexpected status code %d", resp.StatusCode)
	}
	return nil
}

// createDataFromJobStatus starts data extraction and generation
// based on (initial) job status
func (a *Actions) createDataFromJobStatus(status *liveattrs.LiveAttrsJobInfo) error {
	a.vteExitEvents[status.ID] = make(chan os.Signal)
	procStatus, err := vteLib.ExtractData(
		&status.Args.VteConf, status.Args.Append, a.vteExitEvents[status.ID])
	if err != nil {
		return fmt.Errorf("failed to start vert-tagextract: %s", err)
	}
	go func() {
		updateJobChan := a.jobActions.AddJobInfo(status)
		defer func() {
			close(updateJobChan)
			close(a.vteExitEvents[status.ID])
			delete(a.vteExitEvents, status.ID)
		}()

		for upd := range procStatus {
			updateJobChan <- &liveattrs.LiveAttrsJobInfo{
				ID:             status.ID,
				Type:           liveattrs.JobType,
				CorpusID:       status.CorpusID,
				Start:          status.Start,
				Update:         jobs.CurrentDatetime(),
				Error:          upd.Error,
				ProcessedAtoms: upd.ProcessedAtoms,
				ProcessedLines: upd.ProcessedLines,
				NumRestarts:    status.NumRestarts,
				Args:           status.Args,
			}
			if upd.Error == vteProc.ErrorTooManyParsingErrors {
				log.Error().Err(upd.Error).Msg("live attributes extraction failed")
				return

			} else if upd.Error != nil {
				log.Error().Err(upd.Error).Msg("(just registered)")
			}
		}

		a.eqCache.Del(status.CorpusID)

		switch status.Args.VteConf.DB.Type {
		case "mysql":
			if !status.Args.NoCorpusUpdate {
				transact, err := a.cncDB.StartTx()
				if err != nil {
					updateJobChan <- status.CloneWithError(err)
					return
				}
				var bibIDStruct, bibIDAttr string
				if status.Args.VteConf.BibView.IDAttr != "" {
					bibIDAttrElms := strings.SplitN(status.Args.VteConf.BibView.IDAttr, "_", 2)
					bibIDStruct = bibIDAttrElms[0]
					bibIDAttr = bibIDAttrElms[1]
				}
				err = a.cncDB.SetLiveAttrs(
					transact, status.CorpusID, bibIDStruct, bibIDAttr)
				if err != nil {
					updateJobChan <- status.CloneWithError(err)
					transact.Rollback()
				}
				err = a.setSoftResetToKontext()
				if err != nil {
					updateJobChan <- status.CloneWithError(err)
				}
				err = transact.Commit()
				if err != nil {
					updateJobChan <- status.CloneWithError(err)
				}
			}
		case "sqlite":
			err := sqlite.InstallSqliteDatabase(
				status.Args.VteConf.Corpus,
				status.Args.VteConf.DB.Name,
				a.conf.CorporaSetup.TextTypesDbDirPath,
			)
			if err != nil {
				updateJobChan <- status.CloneWithError(err)
			}
			err = a.setSoftResetToKontext()
			if err != nil {
				updateJobChan <- status.CloneWithError(err)
			}
		}
	}()
	return nil
}

func (a *Actions) runStopJobListener() {
	for id := range a.jobStopChannel {
		if job, ok := a.jobActions.GetJob(id); ok {
			if tJob, ok2 := job.(*liveattrs.LiveAttrsJobInfo); ok2 {
				if stopChan, ok3 := a.vteExitEvents[tJob.ID]; ok3 {
					stopChan <- os.Interrupt
				}
			}
		}
	}
}

func (a *Actions) Query(w http.ResponseWriter, req *http.Request) {
	t0 := time.Now()
	vars := mux.Vars(req)
	corpusID := vars["corpusId"]
	baseErrTpl := fmt.Sprintf("failed to query liveattrs in corpus %s", corpusID)
	var qry query.Payload
	err := json.NewDecoder(req.Body).Decode(&qry)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusBadRequest)
		return
	}
	corpInfo, err := a.cncDB.LoadInfo(corpusID)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusInternalServerError)
		return
	}
	usageEntry := db.RequestData{
		CorpusID: corpusID,
		Payload:  qry,
		Created:  time.Now(),
	}

	ans := a.eqCache.Get(corpusID, qry)
	if ans != nil {
		api.WriteJSONResponse(w, &ans)
		usageEntry.IsCached = true
		usageEntry.ProcTime = time.Since(t0)
		a.usageData <- usageEntry
		return
	}
	ans, err = a.getAttrValues(corpInfo, qry)
	if err == laconf.ErrorNoSuchConfig {
		log.Error().Err(err).Msgf("configuration not found for %s", corpusID)
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusNotFound)
		return

	} else if err != nil {
		log.Error().Err(err).Msg("")
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusInternalServerError)
		return
	}
	usageEntry.ProcTime = time.Since(t0)
	a.usageData <- usageEntry
	a.eqCache.Set(corpusID, qry, ans)
	api.WriteJSONResponse(w, &ans)
}

func (a *Actions) FillAttrs(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	corpusID := vars["corpusId"]
	baseErrTpl := fmt.Sprintf("failed to fill attributes for corpus %s", corpusID)

	var qry fillattrs.Payload
	err := json.NewDecoder(req.Body).Decode(&qry)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusInternalServerError)
		return
	}
	corpusDBInfo, err := a.cncDB.LoadInfo(corpusID)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusInternalServerError)
		return
	}
	ans, err := db.FillAttrs(a.laDB, corpusDBInfo, qry)
	if err == db.ErrorEmptyResult {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusNotFound)
		return

	} else if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusInternalServerError)
		return
	}
	api.WriteJSONResponse(w, &ans)
}

func (a *Actions) GetAdhocSubcSize(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	corpusID := vars["corpusId"]
	baseErrTpl := fmt.Sprintf("failed to get ad-hoc subcorpus of corpus %s", corpusID)

	var qry equery.Payload
	err := json.NewDecoder(req.Body).Decode(&qry)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusInternalServerError)
		return
	}
	corpora := append([]string{corpusID}, qry.Aligned...)
	corpusDBInfo, err := a.cncDB.LoadInfo(corpusID)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusInternalServerError)
		return
	}
	size, err := db.GetSubcSize(a.laDB, corpusDBInfo.GroupedName(), corpora, qry.Attrs)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusInternalServerError)
		return
	}
	api.WriteJSONResponse(w, response.GetSubcSize{Total: size})
}

func (a *Actions) GetBibliography(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	corpusID := vars["corpusId"]
	baseErrTpl := fmt.Sprintf("failed to get bibliography from corpus %s", corpusID)

	var qry biblio.Payload
	err := json.NewDecoder(req.Body).Decode(&qry)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusBadRequest)
		return
	}
	corpInfo, err := a.cncDB.LoadInfo(corpusID)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusInternalServerError)
		return
	}
	laConf, err := a.laConfCache.Get(corpInfo.Name)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusInternalServerError)
		return
	}
	ans, err := db.GetBibliography(a.laDB, corpInfo, laConf, qry)
	if err == db.ErrorEmptyResult {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusNotFound)
		return

	} else if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusInternalServerError)
		return
	}
	api.WriteJSONResponse(w, &ans)
}

func (a *Actions) FindBibTitles(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	corpusID := vars["corpusId"]
	baseErrTpl := fmt.Sprintf("failed to find bibliography titles in corpus %s", corpusID)

	var qry biblio.PayloadList
	err := json.NewDecoder(req.Body).Decode(&qry)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusBadRequest)
		return
	}
	corpInfo, err := a.cncDB.LoadInfo(corpusID)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusInternalServerError)
		return
	}
	laConf, err := a.laConfCache.Get(corpInfo.Name)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusInternalServerError)
		return
	}
	ans, err := db.FindBibTitles(a.laDB, corpInfo, laConf, qry)
	if err == db.ErrorEmptyResult {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusNotFound)
		return

	} else if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusInternalServerError)
		return
	}
	api.WriteJSONResponse(w, &ans)
}

func (a *Actions) AttrValAutocomplete(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	corpusID := vars["corpusId"]
	baseErrTpl := fmt.Sprintf("failed to find autocomplete suggestions in corpus %s", corpusID)

	var qry query.Payload
	err := json.NewDecoder(req.Body).Decode(&qry)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusBadRequest)
		return
	}
	corpInfo, err := a.cncDB.LoadInfo(corpusID)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusInternalServerError)
		return
	}
	ans, err := a.getAttrValues(corpInfo, qry)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(baseErrTpl, err), http.StatusInternalServerError)
		return
	}
	api.WriteJSONResponse(w, &ans)
}

func (a *Actions) Stats(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	corpusID := vars["corpusId"]
	ans, err := db.LoadUsage(a.laDB, corpusID)
	if err != nil {
		api.WriteJSONErrorResponse(
			w, api.NewActionErrorFrom(fmt.Sprintf("failed to get stats for corpus %s", corpusID), err), http.StatusInternalServerError)
		return
	}
	api.WriteJSONResponse(w, &ans)
}

func (a *Actions) updateIndexesFromJobStatus(status *liveattrs.IdxUpdateJobInfo) {
	go func() {
		updateJobChan := a.jobActions.AddJobInfo(status)
		defer close(updateJobChan)
		finalStatus := *status
		corpusDBInfo, err := a.cncDB.LoadInfo(status.CorpusID)
		if err != nil {
			finalStatus.Error = err
		}
		ans := db.UpdateIndexes(a.laDB, corpusDBInfo, status.Args.MaxColumns)
		if ans.Error != nil {
			finalStatus.Error = ans.Error
		}
		finalStatus.Update = jobs.CurrentDatetime()
		finalStatus.Finished = true
		finalStatus.Result.RemovedIndexes = ans.RemovedIndexes
		finalStatus.Result.UsedIndexes = ans.UsedIndexes
		updateJobChan <- &finalStatus
	}()
}

func (a *Actions) UpdateIndexes(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	corpusID := vars["corpusId"]
	maxColumnsArg := req.URL.Query().Get("maxColumns")
	if maxColumnsArg == "" {
		api.WriteJSONErrorResponse(
			w, api.NewActionError("missing maxColumns argument"), http.StatusBadRequest)
		return
	}
	maxColumns, err := strconv.Atoi(maxColumnsArg)
	if err != nil {
		api.WriteJSONErrorResponse(
			w, api.NewActionErrorFrom("failed to update indexes", err), http.StatusUnprocessableEntity)
		return
	}
	jobID, err := uuid.NewUUID()
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionError("Failed to start 'update indexes' job for '%s'", corpusID), http.StatusUnauthorized)
		return
	}
	newStatus := liveattrs.IdxUpdateJobInfo{
		ID:       jobID.String(),
		Type:     "liveattrs-idx-update",
		CorpusID: corpusID,
		Start:    jobs.CurrentDatetime(),
		Update:   jobs.CurrentDatetime(),
		Finished: false,
		Args:     liveattrs.IdxJobInfoArgs{MaxColumns: maxColumns},
	}
	a.updateIndexesFromJobStatus(&newStatus)
	api.WriteJSONResponseWithStatus(w, http.StatusCreated, &newStatus)
}

func (a *Actions) RestartLiveAttrsJob(jinfo *liveattrs.LiveAttrsJobInfo) error {
	if jinfo.NumRestarts >= a.conf.Jobs.MaxNumRestarts {
		return fmt.Errorf("cannot restart job %s - max. num. of restarts reached", jinfo.ID)
	}
	jinfo.Start = jobs.CurrentDatetime()
	jinfo.NumRestarts++
	jinfo.Update = jobs.CurrentDatetime()
	err := a.createDataFromJobStatus(jinfo)
	if err != nil {
		return err
	}
	log.Info().Msgf("Restarted liveAttributes job %s", jinfo.ID)
	return nil
}

func (a *Actions) RestartIdxUpdateJob(jinfo *liveattrs.IdxUpdateJobInfo) error {
	return nil
}

func (a *Actions) InferredAtomStructure(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	corpusID := vars["corpusId"]

	conf, err := a.laConfCache.Get(corpusID)
	if err != nil {
		api.WriteJSONErrorResponse(
			w, api.NewActionErrorFrom("failed to get inferred atom structure", err),
			http.StatusInternalServerError,
		)
		return
	}

	ans := map[string]any{"structure": nil}
	if len(conf.Structures) == 1 {
		for k := range conf.Structures {
			ans["structure"] = k
			break
		}
	}
	api.WriteJSONResponse(w, &ans)
}

// NewActions is the default factory for Actions
func NewActions(
	conf *corpus.Conf,
	exitEvent <-chan os.Signal,
	jobStopChannel <-chan string,
	jobActions *jobs.Actions,
	cncDB *cncdb.CNCMySQLHandler,
	laDB *sql.DB,
	version general.VersionInfo,
) *Actions {
	usageChan := make(chan db.RequestData)
	vteExitEvents := make(map[string]chan os.Signal)
	go func() {
		for v := range exitEvent {
			for _, ch := range vteExitEvents {
				ch <- v
			}
		}
	}()
	actions := &Actions{
		exitEvent:      exitEvent,
		vteExitEvents:  vteExitEvents,
		conf:           conf,
		jobActions:     jobActions,
		jobStopChannel: jobStopChannel,
		laConfCache: laconf.NewLiveAttrsBuildConfProvider(
			conf.LiveAttrs.ConfDirPath,
			conf.LiveAttrs.DB,
		),
		cncDB:           cncDB,
		laDB:            laDB,
		eqCache:         cache.NewEmptyQueryCache(),
		structAttrStats: db.NewStructAttrUsage(laDB, usageChan),
		usageData:       usageChan,
	}
	go actions.structAttrStats.RunHandler()
	go actions.runStopJobListener()
	return actions
}
