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

package api

import (
	"html/template"
	"log"
	"net/http"
	"sync"
)

type AlarmPage struct {
	InstanceID string
	RootURL    string
	AlarmID    string
	Error      error
}

const (
	alarmPage = `
<!DOCTYPE html>
<html>
	<head>
		<meta charset="UTF-8">
		<title>KonText alarm</title>
		<style type="text/css">
		html {
			background-color: #000000;
		}
		body {
			font-size: 1.2em;
			width: 40em;
			margin: 0 auto;
			font-family: sans-serif;
			color: #EEEEEE;
		}
		h1 {
			font-size: 1.7em;
		}
		</style>
	</head>
	<body>
		<h1>CNC-MASM - KonText monitoring and middleware</h1>
		{{ if .Error }}
		<p><strong className="err">ERROR:</strong> {{ .Error }}</p>
		{{ else }}
		<p>The active ALARM {{ .AlarmID }} of a monitored KonText instance <strong>{{ .InstanceID}}</strong> has been
		turned OFF. Please make sure the actual problem is solved.</p>
		{{ end }}
	</body>
</html>`
)

var (
	initOnce sync.Once
	tpl      *template.Template
)

func compileAlarmPage() {
	initOnce.Do(func() {
		var err error
		tpl, err = template.New("alarm").Parse(alarmPage)
		if err != nil {
			log.Fatal("Failed to parse the template")
		}
	})
}

// WriteHTMLResponse writes 'value' to an HTTP response encoded as JSON
func WriteHTMLResponse(w http.ResponseWriter, data *AlarmPage) error {
	compileAlarmPage()
	w.Header().Add("Content-Type", "text/html")
	if data.Error != nil {
		w.WriteHeader(http.StatusBadGateway)
	}
	err := tpl.Execute(w, data)
	if err != nil {
		return err
	}
	return nil
}
