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

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"masm/v3/cncdb"
	"masm/v3/corpdata"
	"masm/v3/corpus"
	"masm/v3/db/mysql"
	"masm/v3/general"
	"masm/v3/jobs"
	"masm/v3/liveattrs"
	laActions "masm/v3/liveattrs/actions"
	"masm/v3/registry"
	"masm/v3/root"

	"github.com/gorilla/mux"
)

var (
	version   string
	buildDate string
	gitCommit string
)

type ExitHandler interface {
	OnExit()
}

func setupLog(path string) {
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
}

func coreMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
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
	setupLog(conf.LogFile)
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
	log.Info().Msgf("LiveAttrs SQL database(s): '%s", dbInfo)

	router := mux.NewRouter()
	router.Use(coreMiddleware)
	router.MethodNotAllowedHandler = NotAllowedHandler{}
	router.NotFoundHandler = NotFoundHandler{}

	rootActions := root.Actions{Version: version}

	corpdataActions := corpdata.NewActions(conf, version)
	router.HandleFunc("/corpora-storage/available-locations", corpdataActions.AvailableDataLocations).Methods(http.MethodGet)

	jobStopChannel := make(chan string)

	jobActions := jobs.NewActions(conf.Jobs, exitEvent, jobStopChannel)
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

	router.HandleFunc(
		"/", rootActions.RootAction).Methods(http.MethodGet)
	router.HandleFunc(
		"/corpora/{corpusId}", corpusActions.GetCorpusInfo).Methods(http.MethodGet)
	router.HandleFunc(
		"/corpora/{corpusId}/_syncData", corpusActions.SynchronizeCorpusData).Methods(http.MethodPost)
	router.HandleFunc(
		"/corpora/{subdir}/{corpusId}", corpusActions.GetCorpusInfo).Methods(http.MethodGet)
	router.HandleFunc(
		"/corpora/{subdir}/{corpusId}/_syncData", corpusActions.SynchronizeCorpusData).Methods(http.MethodPost)

	router.HandleFunc(
		"/liveAttributes/{corpusId}/data", liveattrsActions.Create).Methods(http.MethodPost)
	router.HandleFunc(
		"/liveAttributes/{corpusId}/data", liveattrsActions.Delete).Methods(http.MethodDelete)
	router.HandleFunc(
		"/liveAttributes/{corpusId}/conf", liveattrsActions.ViewConf).Methods(http.MethodGet)
	router.HandleFunc(
		"/liveAttributes/{corpusId}/conf", liveattrsActions.CreateConf).Methods(http.MethodPut)
	router.HandleFunc(
		"/liveAttributes/{corpusId}/query", liveattrsActions.Query).Methods(http.MethodPost)
	router.HandleFunc(
		"/liveAttributes/{corpusId}/fillAttrs", liveattrsActions.FillAttrs).Methods(http.MethodPost)
	router.HandleFunc(
		"/liveAttributes/{corpusId}/selectionSubcSize", liveattrsActions.GetAdhocSubcSize).Methods(http.MethodPost)
	router.HandleFunc(
		"/liveAttributes/{corpusId}/attrValAutocomplete", liveattrsActions.AttrValAutocomplete).Methods(http.MethodPost)
	router.HandleFunc(
		"/liveAttributes/{corpusId}/getBibliography", liveattrsActions.GetBibliography).Methods(http.MethodPost)
	router.HandleFunc(
		"/liveAttributes/{corpusId}/findBibTitles", liveattrsActions.FindBibTitles).Methods(http.MethodPost)
	router.HandleFunc(
		"/liveAttributes/{corpusId}/stats", liveattrsActions.Stats)
	router.HandleFunc(
		"/liveAttributes/{corpusId}/updateIndexes", liveattrsActions.UpdateIndexes).Methods(http.MethodPost)
	router.HandleFunc(
		"/liveAttributes/{corpusId}/mixSubcorpus", liveattrsActions.MixSubcorpus).Methods(http.MethodPost)
	router.HandleFunc(
		"/liveAttributes/{corpusId}/inferredAtomStructure", liveattrsActions.InferredAtomStructure).Methods(http.MethodGet)
	router.HandleFunc(
		"/liveAttributes/{corpusId}/ngrams", liveattrsActions.GenerateNgrams).Methods(http.MethodPost)
	router.HandleFunc(
		"/liveAttributes/{corpusId}/querySuggestions", liveattrsActions.CreateQuerySuggestions).Methods(http.MethodPost)
	router.HandleFunc(
		"/liveAttributes/{corpusId}/ngramsAndQuerySuggestions", liveattrsActions.CreateNgramsAndQuerySuggestions).Methods(http.MethodPost)

	router.HandleFunc(
		"/jobs", jobActions.JobList).Methods(http.MethodGet)
	router.HandleFunc(
		"/jobs/{jobId}", jobActions.JobInfo).Methods(http.MethodGet)
	router.HandleFunc(
		"/jobs/{jobId}", jobActions.Delete).Methods(http.MethodDelete)
	router.HandleFunc(
		"/jobs/{jobId}/clearIfFinished", jobActions.ClearIfFinished).Methods(http.MethodGet)

	router.HandleFunc(
		"/registry/defaults/attribute/dynamic-functions", registryActions.DynamicFunctions).Methods(http.MethodGet)
	router.HandleFunc(
		"/registry/defaults/wposlist", registryActions.PosSets).Methods(http.MethodGet)
	router.HandleFunc(
		"/registry/defaults/wposlist/{posId}", registryActions.GetPosSetInfo).Methods(http.MethodGet)
	router.HandleFunc(
		"/registry/defaults/attribute/multivalue", registryActions.GetAttrMultivalueDefaults).Methods(http.MethodGet)
	router.HandleFunc(
		"/registry/defaults/attribute/multisep", registryActions.GetAttrMultisepDefaults).Methods(http.MethodGet)
	router.HandleFunc(
		"/registry/defaults/attribute/dynlib", registryActions.GetAttrDynlibDefaults).Methods(http.MethodGet)
	router.HandleFunc(
		"/registry/defaults/attribute/transquery", registryActions.GetAttrTransqueryDefaults).Methods(http.MethodGet)
	router.HandleFunc(
		"/registry/defaults/structure/multivalue", registryActions.GetStructMultivalueDefaults).Methods(http.MethodGet)
	router.HandleFunc(
		"/registry/defaults/structure/multisep", registryActions.GetStructMultisepDefaults).Methods(http.MethodGet)

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
	router.HandleFunc("/corpora-database/{corpusId}/auto-update", cncdbActions.UpdateCorpusInfo).Methods(http.MethodPost)
	router.HandleFunc("/corpora-database/{corpusId}/kontextDefaults", cncdbActions.InferKontextDefaults).Methods(http.MethodPut)

	log.Info().Msgf("starting to listen at %s:%d", conf.ListenAddress, conf.ListenPort)
	srv := &http.Server{
		Handler:      router,
		Addr:         fmt.Sprintf("%s:%d", conf.ListenAddress, conf.ListenPort),
		WriteTimeout: time.Duration(conf.ServerWriteTimeoutSecs) * time.Second,
		ReadTimeout:  time.Duration(conf.ServerReadTimeoutSecs) * time.Second,
	}

	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			log.Error().Err(err)
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
