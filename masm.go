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

//go:generate pigeon -o ./registry/parser/parser.go ./registry/parser/grammar.peg

package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/czcorpus/cnc-gokit/logging"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"masm/v3/cncdb"
	"masm/v3/cnf"
	"masm/v3/corpus"
	"masm/v3/corpus/query"
	"masm/v3/general"
	"masm/v3/liveattrs"
	"masm/v3/registry"
	"masm/v3/root"
)

var (
	version   string
	buildDate string
	gitCommit string
)

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
	conf := cnf.LoadConfig(flag.Arg(1))
	logging.SetupLogging(conf.Logging)
	log.Info().Msg("Starting MASM (Manatee Assets, Services and Metadata)")
	cnf.ApplyDefaults(conf)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-ctx.Done()
		stop()
	}()

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
		conf.CNCDB.Name,
		cTableName,
		pcTableName,
	)
	if err != nil {
		log.Fatal().Err(err)
	}
	log.Info().Msgf("CNC SQL database: %s@%s", conf.CNCDB.Name, conf.CNCDB.Host)

	if !conf.Logging.Level.IsDebugMode() {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(logging.GinMiddleware())
	engine.Use(uniresp.AlwaysJSONContentType())
	engine.NoMethod(uniresp.NoMethodHandler)
	engine.NoRoute(uniresp.NotFoundHandler)

	rootActions := root.Actions{Version: version, Conf: conf}

	corpusActions := corpus.NewActions(conf.CorporaSetup, cncDB)

	concCache := query.NewCache(conf.CorporaSetup.ConcCacheDirPath, conf.GetLocation())
	concCache.RestoreUnboundEntries()
	concActions := query.NewActions(conf.CorporaSetup, conf.GetLocation(), concCache)

	registryActions := registry.NewActions(conf.CorporaSetup)

	engine.GET(
		"/", rootActions.RootAction)
	engine.GET(
		"/corpora/:corpusId", corpusActions.GetCorpusInfo)

	engine.GET(
		"/freqs/:corpusId", concActions.FreqDistrib)

	engine.GET(
		"/collocs/:corpusId", concActions.Collocations)

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

	laActions, err := liveattrs.NewLiveAttrsActions(ctx, conf.LiveAttrsConf)
	engine.POST(
		"/liveAttributes/:corpusId/data", laActions.Create)

	engine.GET("/jobs/:jobId", laActions.Jobs)

	engine.GET("/jobs", laActions.Jobs)

	cncdbActions := cncdb.NewActions(conf.CNCDB, conf.CorporaSetup, cncDB)
	engine.POST(
		"/corpora-database/:corpusId/auto-update",
		cncdbActions.UpdateCorpusInfo)
	engine.PUT(
		"/corpora-database/:corpusId/kontextDefaults",
		cncdbActions.InferKontextDefaults)

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
			log.Error().Err(err).Send()
		}
	}()

	<-ctx.Done()
	log.Info().Err(err).Msg("Shutdown request error")

	ctxShutDown, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctxShutDown); err != nil {
		log.Fatal().Err(err).Msg("Server forced to shutdown")
	}
}
