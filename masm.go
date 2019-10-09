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
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
)

type Conf struct {
	ListenAddress      string `json:"listenAddress"`
	ListenPort         int    `json:"listenPort"`
	RegistryDirPath    string `json:"registryDirPath"`
	TextTypesDbDirPath string `json:"textTypesDbDirPath"`
}

func loadConfig(path string) *Conf {
	if path == "" {
		log.Fatal("Config path not specified")
	}
	rawData, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	var conf Conf
	json.Unmarshal(rawData, &conf)
	return &conf
}

func actionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		next.ServeHTTP(w, req)
	})
}

func main() {

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "CNC-MASM - Manatee administration setup middleware\n\nUsage:\n\t%s [options] [config.json]\n",
			filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
	flag.Parse()
	conf := loadConfig(flag.Arg(0))

	router := mux.NewRouter().StrictSlash(true)
	router.Use(actionMiddleware)
	actions := NewActions(conf)
	router.HandleFunc("/", actions.rootAction).Methods(http.MethodGet)
	router.HandleFunc("/corpora/{corpusId}/", actions.getCorpusInfo).Methods(http.MethodGet)
	router.HandleFunc("/textTypesDb/{dbName}/", actions.getTextTypeDbInfo).Methods(http.MethodGet)
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", conf.ListenAddress, conf.ListenPort), router))
}
