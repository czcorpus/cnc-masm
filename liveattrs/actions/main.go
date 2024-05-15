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
	"errors"
	"fmt"
	"masm/v3/cncdb"
	"masm/v3/corpus"
	"masm/v3/general"
	"masm/v3/jobs"
	"masm/v3/kontext"
	"masm/v3/liveattrs"
	"masm/v3/liveattrs/cache"
	"masm/v3/liveattrs/db"
	"masm/v3/liveattrs/laconf"
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

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	vteCnf "github.com/czcorpus/vert-tagextract/v2/cnf"
	vteLib "github.com/czcorpus/vert-tagextract/v2/library"
	vteProc "github.com/czcorpus/vert-tagextract/v2/proc"
	"github.com/google/uuid"

	"github.com/czcorpus/cnc-gokit/uniresp"
)

const (
	emptyValuePlaceholder = "?"
	dfltMaxAttrListSize   = 30
	shortLabelMaxLength   = 30
)

var (
	ErrorMissingVertical = errors.New("missing vertical file")
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

type LAConf struct {
	LA      *liveattrs.Conf
	Ngram   *liveattrs.NgramDBConf
	KonText *kontext.Conf
	Corp    *corpus.CorporaSetup
}

// Actions wraps liveattrs-related actions
type Actions struct {
	conf LAConf

	// exitEvent channel recieves value once user (or OS) terminates masm process
	exitEvent <-chan os.Signal

	// vteExitEvent stores "exit" channels for running vert-tagextract jobs
	// (max 1 per corpus)
	vteExitEvents map[string]chan os.Signal

	// jobStopChannel receives job ID based on user interaction with job HTTP API in
	// case users asks for stopping the vte process
	jobStopChannel <-chan string

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

// applyNgramConf based on configuration stored in `jsonArgs`
//
// NOTE: no n-gram config means "do not touch the current" while zero
// content n-gram config will rewrite the current one by empty values
func (a *Actions) applyNgramConf(targetConf *vteCnf.VTEConf, jsonArgs *liveattrsJsonArgs) error {
	if jsonArgs.Ngrams == nil { // won't update anything
		return nil
	}
	if jsonArgs.Ngrams.IsZero() { // data filled but zero => will overwrite everything
		targetConf.Ngrams = *jsonArgs.Ngrams
		return nil
	}
	if len(jsonArgs.Ngrams.VertColumns) > 0 {
		if jsonArgs.Ngrams.NgramSize <= 0 {
			return fmt.Errorf("invalid n-gram size: %d", jsonArgs.Ngrams.NgramSize)
		}
		targetConf.Ngrams = *jsonArgs.Ngrams

	} else if jsonArgs.Ngrams.NgramSize > 0 {
		return fmt.Errorf("missing columns to extract n-grams from")
	}
	return nil
}

func (a *Actions) ensureVerticalFile(vconf *vteCnf.VTEConf, corpusInfo *corpus.Info) error {
	var verticalPath string
	if corpusInfo.RegistryConf.Vertical.FileExists {
		verticalPath = corpusInfo.RegistryConf.Vertical.VisiblePath()

	} else {
		vpInfo, err := corpus.FindVerticalFile(a.conf.LA.VerticalFilesDirPath, corpusInfo.ID)
		if err != nil {
			return err
		}
		if vpInfo.FileExists {
			verticalPath = vpInfo.Path
			log.Warn().
				Str("origPath", corpusInfo.RegistryConf.Vertical.VisiblePath()).
				Str("foundPath", verticalPath).
				Msg("failed to apply configured VERTICAL, but found a different file")

		} else {
			return ErrorMissingVertical
		}
	}
	vconf.VerticalFile = verticalPath
	return nil
}

type liveattrsJsonArgs struct {
	VerticalFiles []string          `json:"verticalFiles"`
	Ngrams        *vteCnf.NgramConf `json:"ngrams"`
}

// createDataFromJobStatus starts data extraction and generation
// based on (initial) job status
func (a *Actions) createDataFromJobStatus(initialStatus *liveattrs.LiveAttrsJobInfo) {
	fn := func(updateJobChan chan<- jobs.GeneralJobInfo) {
		a.vteExitEvents[initialStatus.ID] = make(chan os.Signal)
		procStatus, err := vteLib.ExtractData(
			&initialStatus.Args.VteConf,
			initialStatus.Args.Append,
			a.vteExitEvents[initialStatus.ID],
		)
		if err != nil {
			updateJobChan <- initialStatus.WithError(
				fmt.Errorf("failed to start vert-tagextract: %s", err)).AsFinished()
			close(updateJobChan)
		}
		go func() {
			defer func() {
				close(updateJobChan)
				close(a.vteExitEvents[initialStatus.ID])
				delete(a.vteExitEvents, initialStatus.ID)
			}()
			jobStatus := liveattrs.LiveAttrsJobInfo{
				ID:          initialStatus.ID,
				Type:        liveattrs.JobType,
				CorpusID:    initialStatus.CorpusID,
				Start:       initialStatus.Start,
				Update:      jobs.CurrentDatetime(),
				NumRestarts: initialStatus.NumRestarts,
				Args:        initialStatus.Args,
			}

			for upd := range procStatus {
				if upd.Error == vteProc.ErrorTooManyParsingErrors {
					jobStatus.Error = upd.Error
				}
				jobStatus.ProcessedAtoms = upd.ProcessedAtoms
				jobStatus.ProcessedLines = upd.ProcessedLines
				updateJobChan <- jobStatus

				if upd.Error == vteProc.ErrorTooManyParsingErrors {
					log.Error().Err(upd.Error).Msg("live attributes extraction failed")
					return

				} else if upd.Error != nil {
					log.Error().Err(upd.Error).Msg("(just registered)")
				}
			}

			a.eqCache.Del(jobStatus.CorpusID)
			switch jobStatus.Args.VteConf.DB.Type {
			case "mysql":
				if !jobStatus.Args.NoCorpusUpdate {
					transact, err := a.cncDB.StartTx()
					if err != nil {
						updateJobChan <- jobStatus.WithError(err)
						return
					}
					var bibIDStruct, bibIDAttr string
					if jobStatus.Args.VteConf.BibView.IDAttr != "" {
						bibIDAttrElms := strings.SplitN(jobStatus.Args.VteConf.BibView.IDAttr, "_", 2)
						bibIDStruct = bibIDAttrElms[0]
						bibIDAttr = bibIDAttrElms[1]
					}
					err = a.cncDB.SetLiveAttrs(
						transact, jobStatus.CorpusID, bibIDStruct, bibIDAttr)
					if err != nil {
						updateJobChan <- jobStatus.WithError(err)
						transact.Rollback()
					}
					err = kontext.SendSoftReset(a.conf.KonText)
					if err != nil {
						updateJobChan <- jobStatus.WithError(err)
					}
					err = transact.Commit()
					if err != nil {
						updateJobChan <- jobStatus.WithError(err)
					}
				}
			case "sqlite":
				err = kontext.SendSoftReset(a.conf.KonText)
				if err != nil {
					updateJobChan <- initialStatus.WithError(err)
				}
			}
			updateJobChan <- jobStatus.AsFinished()
		}()
	}
	a.jobActions.EnqueueJob(&fn, initialStatus)
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

func (a *Actions) Query(ctx *gin.Context) {
	t0 := time.Now()
	corpusID := ctx.Param("corpusId")
	baseErrTpl := "failed to query liveattrs in corpus %s: %w"
	var qry query.Payload
	err := json.NewDecoder(ctx.Request.Body).Decode(&qry)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusBadRequest)
		return
	}
	corpInfo, err := a.cncDB.LoadInfo(corpusID)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		return
	}
	usageEntry := db.RequestData{
		CorpusID: corpusID,
		Payload:  qry,
		Created:  time.Now(),
	}

	ans := a.eqCache.Get(corpusID, qry)
	if ans != nil {
		uniresp.WriteJSONResponse(ctx.Writer, &ans)
		usageEntry.IsCached = true
		usageEntry.ProcTime = time.Since(t0)
		a.usageData <- usageEntry
		return
	}
	ans, err = a.getAttrValues(corpInfo, qry)
	if err == laconf.ErrorNoSuchConfig {
		log.Error().Err(err).Msgf("configuration not found for %s", corpusID)
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusNotFound)
		return

	} else if err != nil {
		log.Error().Err(err).Msg("")
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		return
	}
	usageEntry.ProcTime = time.Since(t0)
	a.usageData <- usageEntry
	a.eqCache.Set(corpusID, qry, ans)
	uniresp.WriteJSONResponse(ctx.Writer, &ans)
}

func (a *Actions) FillAttrs(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	baseErrTpl := "failed to fill attributes for corpus %s: %w"

	var qry fillattrs.Payload
	err := json.NewDecoder(ctx.Request.Body).Decode(&qry)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		return
	}
	corpusDBInfo, err := a.cncDB.LoadInfo(corpusID)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		return
	}
	ans, err := db.FillAttrs(a.laDB, corpusDBInfo, qry)
	if err == db.ErrorEmptyResult {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusNotFound)
		return

	} else if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, &ans)
}

func (a *Actions) GetAdhocSubcSize(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	baseErrTpl := "failed to get ad-hoc subcorpus of corpus %s: %w"

	var qry equery.Payload
	err := json.NewDecoder(ctx.Request.Body).Decode(&qry)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		return
	}
	corpora := append([]string{corpusID}, qry.Aligned...)
	corpusDBInfo, err := a.cncDB.LoadInfo(corpusID)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		return
	}
	size, err := db.GetSubcSize(a.laDB, corpusDBInfo, corpora, qry.Attrs)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, response.GetSubcSize{Total: size})
}

func (a *Actions) AttrValAutocomplete(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	baseErrTpl := "failed to find autocomplete suggestions in corpus %s: %w"

	var qry query.Payload
	err := json.NewDecoder(ctx.Request.Body).Decode(&qry)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusBadRequest)
		return
	}
	corpInfo, err := a.cncDB.LoadInfo(corpusID)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		return
	}
	ans, err := a.getAttrValues(corpInfo, qry)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, &ans)
}

func (a *Actions) Stats(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	ans, err := db.LoadUsage(a.laDB, corpusID)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError("failed to get stats for corpus %s: %w", corpusID, err), http.StatusInternalServerError)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, &ans)
}

func (a *Actions) updateIndexesFromJobStatus(status *liveattrs.IdxUpdateJobInfo) {
	fn := func(updateJobChan chan<- jobs.GeneralJobInfo) {
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
	}
	a.jobActions.EnqueueJob(&fn, status)
}

func (a *Actions) UpdateIndexes(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	maxColumnsArg := ctx.Request.URL.Query().Get("maxColumns")
	if maxColumnsArg == "" {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError("missing maxColumns argument"), http.StatusBadRequest)
		return
	}
	maxColumns, err := strconv.Atoi(maxColumnsArg)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError("failed to update indexes: %w", err), http.StatusUnprocessableEntity)
		return
	}
	jobID, err := uuid.NewUUID()
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError("Failed to start 'update indexes' job for '%s'", corpusID), http.StatusUnauthorized)
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
	uniresp.WriteJSONResponseWithStatus(ctx.Writer, http.StatusCreated, &newStatus)
}

func (a *Actions) RestartLiveAttrsJob(jinfo *liveattrs.LiveAttrsJobInfo) error {
	err := a.jobActions.TestAllowsJobRestart(jinfo)
	if err != nil {
		return err
	}
	jinfo.Start = jobs.CurrentDatetime()
	jinfo.NumRestarts++
	jinfo.Update = jobs.CurrentDatetime()
	a.createDataFromJobStatus(jinfo)
	log.Info().Msgf("Restarted liveAttributes job %s", jinfo.ID)
	return nil
}

func (a *Actions) RestartIdxUpdateJob(jinfo *liveattrs.IdxUpdateJobInfo) error {
	return nil
}

func (a *Actions) InferredAtomStructure(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")

	conf, err := a.laConfCache.Get(corpusID)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError("failed to get inferred atom structure: %w", err),
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
	uniresp.WriteJSONResponse(ctx.Writer, &ans)
}

// NewActions is the default factory for Actions
func NewActions(
	conf LAConf,
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
		conf:           conf,
		exitEvent:      exitEvent,
		vteExitEvents:  vteExitEvents,
		jobActions:     jobActions,
		jobStopChannel: jobStopChannel,
		laConfCache: laconf.NewLiveAttrsBuildConfProvider(
			conf.LA.ConfDirPath,
			conf.LA.DB,
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
