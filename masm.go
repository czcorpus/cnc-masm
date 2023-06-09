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

package main

import (
	"context"
	"encoding/gob"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"masm/v3/cncdb"
	"masm/v3/corpdata"
	"masm/v3/corpus"
	"masm/v3/db/mysql"
	"masm/v3/debug"
	"masm/v3/general"
	"masm/v3/jobs"
	"masm/v3/liveattrs"
	laActions "masm/v3/liveattrs/actions"
	"masm/v3/registry"
	"masm/v3/root"

	_ "masm/v3/translations"
)

var (
	version   string
	buildDate string
	gitCommit string
)

type ExitHandler interface {
	OnExit()
}

func setupLog(path string, debugMode bool) {
	if path != "" {
		logf, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatal().Msgf("Failed to initialize log. File: %s", path)
		}
		log.Logger = log.Output(logf)

	} else {
		log.Logger = log.Output(
			zerolog.ConsoleWriter{
				Out:        os.Stderr,
				TimeFormat: time.RFC3339,
			},
		)
	}

	if debugMode {
		log.Logger = log.Logger.Level(zerolog.DebugLevel)
	} else {
		log.Logger = log.Logger.Level(zerolog.InfoLevel)
	}
	log.Debug().Msg("Running application in debug mode...")
}

func coreMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Content-Type", "application/json")
		c.Next()
	}
}

func notFoundHandler(c *gin.Context) {
	uniresp.WriteJSONErrorResponse(
		c.Writer, uniresp.NewActionError("action not found"), http.StatusNotFound)
}

func noMethodHandler(c *gin.Context) {
	uniresp.WriteJSONErrorResponse(
		c.Writer, uniresp.NewActionError("method not allowed"), http.StatusMethodNotAllowed)
}

func legacyHandler(fn func(w http.ResponseWriter, r *http.Request)) func(*gin.Context) {
	return func(ctx *gin.Context) {
		fn(ctx.Writer, ctx.Request)
	}
}

func GinZerolog() gin.HandlerFunc {

	return func(c *gin.Context) {
		// Start timer
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery
		if raw != "" {
			path = path + "?" + raw
		}

		// Process request
		c.Next()

		var logEvent *zerolog.Event
		if c.Writer.Status() >= 500 {
			logEvent = log.Error()

		} else {
			logEvent = log.Info()
		}
		t0 := time.Now()
		logEvent.
			Float64("latency", t0.Sub(start).Seconds()).
			Str("clientIP", c.ClientIP()).
			Str("method", c.Request.Method).
			Int("status", c.Writer.Status()).
			Str("errorMessage", c.Errors.ByType(gin.ErrorTypePrivate).String()).
			Int("bodySize", c.Writer.Size()).
			Str("path", path).
			Send()
	}
}

func init() {
	gob.Register(&liveattrs.LiveAttrsJobInfo{})
	gob.Register(&liveattrs.IdxUpdateJobInfo{})
	gob.Register(&corpus.JobInfo{})
}

func main() {
	version := general.VersionInfo{
		Version:   version,
		BuildDate: buildDate,
		GitCommit: gitCommit,
	}

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "CNC-MASM - Manatee administration setup middleware\n\nUsage:\n\t%s [options] start [config.json]\n\t%s [options] version\n",
			filepath.Base(os.Args[0]), filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
	flag.Parse()
	action := flag.Arg(0)
	if action == "version" {
		fmt.Printf("cnc-masm %s\nbuild date: %s\nlast commit: %s\n", version.Version, version.BuildDate, version.GitCommit)
		return

	} else if action != "start" {
		log.Fatal().Msgf("Unknown action %s", action)
	}
	conf := corpus.LoadConfig(flag.Arg(1))
	setupLog(conf.LogFile, conf.DebugMode)
	log.Info().Msg("Starting MASM (Manatee Assets, Services and Metadata)")
	corpus.ApplyDefaults(conf)
	syscallChan := make(chan os.Signal, 1)
	signal.Notify(syscallChan, os.Interrupt)
	signal.Notify(syscallChan, syscall.SIGTERM)
	exitEvent := make(chan os.Signal)

	cTableName := "corpora"
	if conf.CNCDB.OverrideCorporaTableName != "" {
		log.Warn().Msgf(
			"Overriding default corpora table name to '%s'", conf.CNCDB.OverrideCorporaTableName)
		cTableName = conf.CNCDB.OverrideCorporaTableName
	}
	pcTableName := "parallel_corpus"
	if conf.CNCDB.OverridePCTableName != "" {
		log.Warn().Msgf(
			"Overriding default parallel corpora table name to '%s'", conf.CNCDB.OverridePCTableName)
		pcTableName = conf.CNCDB.OverridePCTableName
	}
	cncDB, err := cncdb.NewCNCMySQLHandler(
		conf.CNCDB.Host,
		conf.CNCDB.User,
		conf.CNCDB.Passwd,
		conf.CNCDB.DBName,
		cTableName,
		pcTableName,
	)
	if err != nil {
		log.Fatal().Err(err)
	}
	log.Info().Msgf("CNC SQL database: %s", conf.CNCDB.Host)

	laDB, err := mysql.OpenDB(conf.LiveAttrs.DB)
	if err != nil {
		log.Fatal().Err(err)
	}
	var dbInfo string
	if conf.LiveAttrs.DB.Type == "mysql" {
		dbInfo = conf.LiveAttrs.DB.Host

	} else {
		dbInfo = fmt.Sprintf("file://%s/*.db", conf.CorporaSetup.TextTypesDbDirPath)
	}
	log.Info().Msgf("LiveAttrs SQL database(s): %s", dbInfo)

	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(GinZerolog())
	engine.Use(coreMiddleware())
	engine.NoMethod(noMethodHandler)
	engine.NoRoute(notFoundHandler)

	rootActions := root.Actions{Version: version}

	corpdataActions := corpdata.NewActions(conf, version)
	engine.GET(
		"/corpora-storage/available-locations",
		legacyHandler(corpdataActions.AvailableDataLocations),
	)

	jobStopChannel := make(chan string)

	jobActions := jobs.NewActions(conf.Jobs, conf.Language, exitEvent, jobStopChannel)
	corpusActions := corpus.NewActions(conf, jobActions)
	liveattrsActions := laActions.NewActions(
		conf, exitEvent, jobStopChannel, jobActions, cncDB, laDB, version)
	registryActions := registry.NewActions(conf)

	for _, dj := range jobActions.GetDetachedJobs() {
		if dj.IsFinished() {
			continue
		}
		switch tdj := dj.(type) {
		case *liveattrs.LiveAttrsJobInfo:
			err := liveattrsActions.RestartLiveAttrsJob(tdj)
			if err != nil {
				log.Error().Err(err).Msgf("Failed to restart job %s. The job will be removed.", tdj.ID)
			}
			jobActions.ClearDetachedJob(tdj.ID)
		case *liveattrs.IdxUpdateJobInfo:
			err := liveattrsActions.RestartIdxUpdateJob(tdj)
			if err != nil {
				log.Error().Err(err).Msgf("Failed to restart job %s. The job will be removed.", tdj.ID)
			}
			jobActions.ClearDetachedJob(tdj.ID)
		case *corpus.JobInfo:
			err := corpusActions.RestartJob(tdj)
			if err != nil {
				log.Error().Err(err).Msgf("Failed to restart job %s. The job will be removed.", tdj.ID)
			}
			jobActions.ClearDetachedJob(tdj.ID)
		default:
			log.Error().Msg("unknown detached job type")
		}
	}

	engine.GET(
		"/", rootActions.RootAction)
	engine.GET(
		"/corpora/:corpusId", corpusActions.GetCorpusInfo)
	engine.POST(
		"/corpora/:corpusId/_syncData", corpusActions.SynchronizeCorpusData)

	engine.POST(
		"/liveAttributes/:corpusId/data", liveattrsActions.Create)
	engine.DELETE(
		"/liveAttributes/:corpusId/data", liveattrsActions.Delete)
	engine.GET(
		"/liveAttributes/:corpusId/conf", liveattrsActions.ViewConf)
	engine.PUT(
		"/liveAttributes/:corpusId/conf", liveattrsActions.CreateConf)
	engine.POST(
		"/liveAttributes/:corpusId/query", liveattrsActions.Query)
	engine.POST(
		"/liveAttributes/:corpusId/fillAttrs", liveattrsActions.FillAttrs)
	engine.POST(
		"/liveAttributes/:corpusId/selectionSubcSize",
		liveattrsActions.GetAdhocSubcSize)
	engine.POST(
		"/liveAttributes/:corpusId/attrValAutocomplete",
		liveattrsActions.AttrValAutocomplete)
	engine.POST(
		"/liveAttributes/:corpusId/getBibliography",
		liveattrsActions.GetBibliography)
	engine.POST(
		"/liveAttributes/:corpusId/findBibTitles",
		liveattrsActions.FindBibTitles)
	engine.GET(
		"/liveAttributes/:corpusId/stats", liveattrsActions.Stats)
	engine.POST(
		"/liveAttributes/:corpusId/updateIndexes",
		liveattrsActions.UpdateIndexes)
	engine.POST(
		"/liveAttributes/:corpusId/mixSubcorpus",
		liveattrsActions.MixSubcorpus)
	engine.GET(
		"/liveAttributes/:corpusId/inferredAtomStructure",
		liveattrsActions.InferredAtomStructure)
	engine.POST(
		"/liveAttributes/:corpusId/ngrams",
		liveattrsActions.GenerateNgrams)
	engine.POST(
		"/liveAttributes/:corpusId/querySuggestions",
		liveattrsActions.CreateQuerySuggestions)
	engine.POST(
		"/liveAttributes/:corpusId/documentList",
		liveattrsActions.DocumentList)
	engine.POST(
		"/liveAttributes/:corpusId/numMatchingDocuments",
		liveattrsActions.NumMatchingDocuments)

	engine.GET(
		"/jobs", jobActions.JobList)
	engine.GET(
		"/jobs/utilization", jobActions.Utilization)
	engine.GET(
		"/jobs/:jobId", jobActions.JobInfo)
	engine.DELETE(
		"/jobs/:jobId", jobActions.Delete)
	engine.GET(
		"/jobs/:jobId/clearIfFinished", jobActions.ClearIfFinished)
	engine.GET(
		"/jobs/:jobId/emailNotification", jobActions.GetNotifications)
	engine.GET(
		"/jobs/:jobId/emailNotification/:address",
		jobActions.CheckNotification)
	engine.PUT(
		"/jobs/:jobId/emailNotification/:address",
		jobActions.AddNotification)
	engine.DELETE(
		"/jobs/:jobId/emailNotification/:address",
		jobActions.RemoveNotification)

	engine.GET(
		"/registry/defaults/attribute/dynamic-functions",
		registryActions.DynamicFunctions)
	engine.GET(
		"/registry/defaults/wposlist", registryActions.PosSets)
	engine.GET(
		"/registry/defaults/wposlist/:posId", registryActions.GetPosSetInfo)
	engine.GET(
		"/registry/defaults/attribute/multivalue",
		registryActions.GetAttrMultivalueDefaults)
	engine.GET(
		"/registry/defaults/attribute/multisep",
		registryActions.GetAttrMultisepDefaults)
	engine.GET(
		"/registry/defaults/attribute/dynlib",
		registryActions.GetAttrDynlibDefaults)
	engine.GET(
		"/registry/defaults/attribute/transquery",
		registryActions.GetAttrTransqueryDefaults)
	engine.GET(
		"/registry/defaults/structure/multivalue",
		registryActions.GetStructMultivalueDefaults)
	engine.GET(
		"/registry/defaults/structure/multisep",
		registryActions.GetStructMultisepDefaults)

	go func(exitHandlers []ExitHandler) {
		select {
		case evt := <-syscallChan:
			for _, h := range exitHandlers {
				h.OnExit()
			}
			exitEvent <- evt
			close(exitEvent)
		}
	}([]ExitHandler{corpdataActions, jobActions, corpusActions, liveattrsActions})

	cncdbActions := cncdb.NewActions(conf, cncDB)
	engine.POST(
		"/corpora-database/:corpusId/auto-update",
		cncdbActions.UpdateCorpusInfo)
	engine.PUT(
		"/corpora-database/:corpusId/kontextDefaults",
		cncdbActions.InferKontextDefaults)

	if conf.DebugMode {
		debugActions := debug.NewActions(jobActions)
		engine.POST("/debug/createJob", debugActions.CreateDummyJob)
		engine.POST("/debug/finishJob/:jobId", debugActions.FinishDummyJob)
	}

	log.Info().Msgf("starting to listen at %s:%d", conf.ListenAddress, conf.ListenPort)
	srv := &http.Server{
		Handler:      engine,
		Addr:         fmt.Sprintf("%s:%d", conf.ListenAddress, conf.ListenPort),
		WriteTimeout: time.Duration(conf.ServerWriteTimeoutSecs) * time.Second,
		ReadTimeout:  time.Duration(conf.ServerReadTimeoutSecs) * time.Second,
	}

	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			log.Error().Err(err).Msg("")
		}
		syscallChan <- syscall.SIGTERM
	}()

	select {
	case <-exitEvent:
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := srv.Shutdown(ctx)
		if err != nil {
			log.Info().Err(err).Msg("Shutdown request error")
		}
	}
}
