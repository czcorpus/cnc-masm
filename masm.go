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
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"masm/cnf"
	"masm/corpus"
	"masm/kontext"

	"github.com/gorilla/mux"
)

const (
	version = "0.1.0"
)

func actionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		next.ServeHTTP(w, req)
	})
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

func main() {

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "CNC-MASM - Manatee administration setup middleware\n\nUsage:\n\t%s [options] [config.json]\n",
			filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
	flag.Parse()
	conf := cnf.LoadConfig(flag.Arg(0))
	setupLog(conf.LogFile)
	log.Print("INFO: starting Portal Corpus Adminstration Manatee middleware server")

	router := mux.NewRouter()
	router.Use(actionMiddleware)
	corpusActions := corpus.NewActions(conf, version)
	kontextActions := kontext.NewActions(conf, version)
	router.HandleFunc("/", corpusActions.RootAction).Methods(http.MethodGet)
	router.HandleFunc("/corpora/{corpusId}", corpusActions.GetCorpusInfo).Methods(http.MethodGet)
	router.HandleFunc("/kontext-services/list-all", kontextActions.ListAll).Methods(http.MethodGet)
	router.HandleFunc("/kontext-services/soft-reset-all", kontextActions.SoftReset).Methods(http.MethodPost)
	router.HandleFunc("/kontext-services/auto-detect", kontextActions.AutoDetectProcesses).Methods(http.MethodPost)
	router.HandleFunc("/kontext-services/alarms/{token}", kontextActions.ResetAlarm).Methods(http.MethodGet) // we need GET here (e.g. click via email to reset)
	router.HandleFunc("/kontext-services/{pid}", kontextActions.RegisterProcess).Methods(http.MethodPut)
	router.HandleFunc("/kontext-services/{pid}", kontextActions.UnregisterProcess).Methods(http.MethodDelete)
	router.HandleFunc("/kontext-services/{pid}/soft-reset", kontextActions.SoftReset).Methods(http.MethodPost)
	log.Printf("INFO: starting to listen at %s:%d", conf.ListenAddress, conf.ListenPort)
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", conf.ListenAddress, conf.ListenPort), router))
}
