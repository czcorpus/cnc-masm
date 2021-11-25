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
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"masm/v2/cncdb"
	"masm/v2/cnf"
	"masm/v2/corpus"
	"masm/v2/fsops"
	"masm/v2/jobs"
	"masm/v2/liveattrs"
	"masm/v2/registry"

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
			log.Fatalf("Failed to initialize log. File: %s", path)
		}
		log.SetOutput(logf) // runtime should close the file when program exits
	}
}

func coreMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func init() {
	gob.Register(liveattrs.JobInfo{})
	gob.Register(corpus.JobInfo{})
}

func main() {
	version := cnf.VersionInfo{
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
		log.Fatal("Unknown action ", action)
	}
	conf := cnf.LoadConfig(flag.Arg(1))
	setupLog(conf.LogFile)
	log.Print("INFO: starting Portal Corpus Adminstration Manatee middleware server")

	syscallChan := make(chan os.Signal, 1)
	signal.Notify(syscallChan, os.Interrupt)
	signal.Notify(syscallChan, syscall.SIGTERM)
	exitEvent := make(chan os.Signal)

	router := mux.NewRouter()
	router.Use(coreMiddleware)

	fsopsActions := fsops.NewActions(conf, version)
	router.HandleFunc("/corpora-storage/available-locations", fsopsActions.AvailableDataLocations).Methods(http.MethodGet)

	jobActions := jobs.NewActions(conf, exitEvent, version)
	corpusActions := corpus.NewActions(conf, jobActions, version)
	liveattrsActions := liveattrs.NewActions(conf, exitEvent, jobActions, version)
	registryActions := registry.NewActions(conf)

	router.HandleFunc("/", corpusActions.RootAction).Methods(http.MethodGet)
	router.HandleFunc("/corpora/{corpusId}", corpusActions.GetCorpusInfo).Methods(http.MethodGet)
	router.HandleFunc("/corpora/{corpusId}/_syncData", corpusActions.SynchronizeCorpusData).Methods(http.MethodPost)
	router.HandleFunc("/corpora/{corpusId}/_createLiveAttrs", liveattrsActions.Create).Methods(http.MethodPost)
	router.HandleFunc("/corpora/{subdir}/{corpusId}", corpusActions.GetCorpusInfo).Methods(http.MethodGet)
	router.HandleFunc("/corpora/{subdir}/{corpusId}/_syncData", corpusActions.SynchronizeCorpusData).Methods(http.MethodPost)

	router.HandleFunc("/jobs", jobActions.SyncJobsList).Methods(http.MethodGet)
	router.HandleFunc("/jobs/{jobId}", jobActions.SyncJobInfo).Methods(http.MethodGet)

	router.HandleFunc("/registry/defaults/attribute/dynamic-functions", registryActions.DynamicFunctions).Methods(http.MethodGet)
	router.HandleFunc("/registry/defaults/wposlist", registryActions.PosSets).Methods(http.MethodGet)
	router.HandleFunc("/registry/defaults/wposlist/{posId}", registryActions.GetPosSetInfo).Methods(http.MethodGet)
	router.HandleFunc("/registry/defaults/attribute/multivalue", registryActions.GetAttrMultivalueDefaults).Methods(http.MethodGet)
	router.HandleFunc("/registry/defaults/attribute/multisep", registryActions.GetAttrMultisepDefaults).Methods(http.MethodGet)
	router.HandleFunc("/registry/defaults/attribute/dynlib", registryActions.GetAttrDynlibDefaults).Methods(http.MethodGet)
	router.HandleFunc("/registry/defaults/attribute/transquery", registryActions.GetAttrTransqueryDefaults).Methods(http.MethodGet)
	router.HandleFunc("/registry/defaults/structure/multivalue", registryActions.GetStructMultivalueDefaults).Methods(http.MethodGet)
	router.HandleFunc("/registry/defaults/structure/multisep", registryActions.GetStructMultisepDefaults).Methods(http.MethodGet)

	go func(exitHandlers []ExitHandler) {
		select {
		case evt := <-syscallChan:
			for _, h := range exitHandlers {
				h.OnExit()
			}
			exitEvent <- evt
			close(exitEvent)
		}
	}([]ExitHandler{fsopsActions, jobActions, corpusActions, liveattrsActions})

	cncDB, err := cncdb.NewCNCMySQLHandler(conf.CNCDB.Host, conf.CNCDB.User, conf.CNCDB.Passwd, conf.CNCDB.DBName)
	if err != nil {
		log.Fatal("FATAL: ", err)
	}
	log.Printf("INFO: corpora SQL database at '%s'", conf.CNCDB.Host)
	cncdbActions := cncdb.NewActions(conf, cncDB)
	router.HandleFunc("/corpora-database/{corpusId}/auto-update", cncdbActions.UpdateCorpusInfo).Methods(http.MethodPost)

	log.Printf("INFO: starting to listen at %s:%d", conf.ListenAddress, conf.ListenPort)
	srv := &http.Server{
		Handler:      router,
		Addr:         fmt.Sprintf("%s:%d", conf.ListenAddress, conf.ListenPort),
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  time.Duration(conf.ServerReadTimeoutSecs) * time.Second,
	}

	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			log.Print(err)
		}
		syscallChan <- syscall.SIGTERM
	}()

	select {
	case <-exitEvent:
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := srv.Shutdown(ctx)
		if err != nil {
			log.Printf("Shutdown request error: %v", err)
		}
	}
}
