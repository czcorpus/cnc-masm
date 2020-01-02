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

package cnf

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

// CorporaSetup defines masm application configuration related
// to a corpus
type CorporaSetup struct {
	RegistryDirPath      string `json:"registryDirPath"`
	TextTypesDbDirPath   string `json:"textTypesDbDirPath"`
	DataDirPath          string `json:"dataDirPath"`
	WordSketchDefDirPath string `json:"wordSketchDefDirPath"`
	VerticalFilesDirPath string `json:"verticalFilesDirPath"`
}

type KontextMonitoringSetup struct {
	Instances          []string `json:"instances"`
	NotificationEmails []string `json:"notificationEmails"`
	SMTPServer         string   `json:"smtpServer"`
	Sender             string   `json:"sender"`
	CheckIntervalSecs  int      `json:"checkIntervalSecs"`
	AlarmNumErrors     int      `json:"alarmNumErrors"`
	AlarmResetURL      string   `json:"alarmResetUrl"`
}

// Conf is a global configuration of the app
type Conf struct {
	ListenAddress     string                  `json:"listenAddress"`
	ListenPort        int                     `json:"listenPort"`
	CorporaSetup      *CorporaSetup           `json:"corporaSetup"`
	LogFile           string                  `json:"logFile"`
	KonTextMonitoring *KontextMonitoringSetup `json:"kontextMonitoring"`
}

func LoadConfig(path string) *Conf {
	if path == "" {
		log.Fatal("FATAL: Cannot load config - path not specified")
	}
	rawData, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatal("FATAL: Cannot load config - ", err)
	}
	var conf Conf
	err = json.Unmarshal(rawData, &conf)
	if err != nil {
		log.Fatal("FATAL: Cannot load config - ", err)
	}
	return &conf
}
