// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Institute of the Czech National Corpus,
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

package qs

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"masm/v3/common"
	"masm/v3/corpus"
	"masm/v3/db/couchdb"
	"masm/v3/jobs"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

const (
	exportChunkSize = 100
)

var (
	keyAlphabet     = []byte{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z', 'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z'}
	validWordRegexp = regexp.MustCompile(`^[\sA-Za-z0-9áÁéÉěĚšŠčČřŘžŽýÝíÍúÚůťŤďĎňŇóÓ\-\|]`)
)

func mkID(x int) string {
	ans := []byte{'0', '0', '0', '0', '0', '0'}
	idx := len(ans) - 1
	for x > 0 {
		p := x % len(keyAlphabet)
		ans[idx] = keyAlphabet[p]
		x = int(x / len(keyAlphabet))
		idx -= 1
	}
	return strings.Join(common.MapSlice(ans, func(v byte, _ int) string { return string(v) }), "")
}

type exporterStatus struct {
	TablesReady  bool
	NumProcLines int
	Error        error
}

func (es exporterStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(
		struct {
			TablesReady  bool   `json:"tablesReady"`
			NumProcLines int    `json:"numProcLines"`
			Error        string `json:"error,omitempty"`
		}{
			TablesReady:  es.TablesReady,
			NumProcLines: es.NumProcLines,
			Error:        jobs.ErrorToString(es.Error),
		},
	)
}

type Form struct {
	Value string  `json:"word"`
	Count int     `json:"count"`
	ARF   float64 `json:"arf"`
}

type Sublemma struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

type Lemma struct {
	ID        string     `json:"_id"`
	Lemma     string     `json:"lemma"`
	Forms     []Form     `json:"forms"`
	Sublemmas []Sublemma `json:"sublemmas"`
	PoS       string     `json:"pos"`
	ARF       float64    `json:"arf"`
	IsPname   bool       `json:"is_pname"`
	Count     int        `json:"count"`
}

func (lemma *Lemma) ToJSON() ([]byte, error) {
	return json.Marshal(lemma)
}

type Exporter struct {
	db          *sql.DB
	cb          *couchdb.ClientBase
	groupedName string
	jobActions  *jobs.Actions
}

func (exp *Exporter) processRows(rows *sql.Rows, statusChan chan<- exporterStatus, status exporterStatus) error {
	bulkWriter := couchdb.NewDocHandler[*Lemma](exp.cb)
	var idBase, procRecords int
	chunk := make([]*Lemma, 0, exportChunkSize)
	var currLemma *Lemma
	sublemmas := make(map[string]int)

	for rows.Next() {
		var lemmaValue, sublemmaValue, wordValue, lemmaPos, wordPos string
		var sublemmaCount, lemmaCount, wordCount int
		var lemmaArf, wordArf float64
		var isPname bool
		err := rows.Scan(
			&wordValue, &lemmaValue, &sublemmaValue, &sublemmaCount,
			&wordPos, &wordCount, &wordArf, &lemmaPos, &lemmaCount, &lemmaArf,
			&isPname)
		if err != nil {
			status.Error = err
			statusChan <- status
			return err
		}
		if validWordRegexp.MatchString(lemmaValue) {
			newLemma := lemmaValue
			newPos := lemmaPos
			if currLemma == nil || newLemma != currLemma.Lemma || newPos != currLemma.PoS {
				if currLemma != nil {
					chunk = append(chunk, currLemma)
					currLemma.Sublemmas = make([]Sublemma, 0, len(sublemmas))
					for sValue, sCount := range sublemmas {
						currLemma.Sublemmas = append(
							currLemma.Sublemmas,
							Sublemma{Value: sValue, Count: sCount},
						)
					}
					sublemmas = make(map[string]int)
				}
				currLemma = &Lemma{
					ID:        mkID(idBase),
					Lemma:     newLemma,
					Forms:     []Form{},
					Sublemmas: []Sublemma{},
					PoS:       newPos,
					ARF:       lemmaArf,
					IsPname:   isPname,
					Count:     lemmaCount,
				}
				idBase++
			}
			currLemma.Forms = append(
				currLemma.Forms,
				Form{
					Value: wordValue,
					Count: wordCount,
					ARF:   wordArf,
				},
			)
			sublemmas[sublemmaValue] = sublemmaCount
			if len(chunk) == exportChunkSize {
				bulkWriter.BulkInsert(chunk)
				chunk = make([]*Lemma, 0, exportChunkSize)
			}
		}
		procRecords++
		if procRecords%100000 == 0 {
			status.NumProcLines = procRecords
			statusChan <- status
			log.Debug().Msgf("Processed %d records", procRecords)
		}
	}
	if procRecords == 0 {
		err := fmt.Errorf("there were no n-gram records to process")
		status.Error = err
		statusChan <- status
		return err
	}
	if currLemma != nil {
		chunk = append(chunk, currLemma)
	}
	if len(chunk) > 0 {
		err := bulkWriter.BulkInsert(chunk)
		if err != nil {
			status.Error = err
			statusChan <- status
			return err
		}
	}
	return nil
}

func (exp *Exporter) exportValuesToCouchDB(statusChan chan<- exporterStatus) error {
	defer close(statusChan)
	status := exporterStatus{}

	couchdbSchema := couchdb.NewSchema(exp.cb)
	err := couchdbSchema.CreateDatabase("masm")
	if err != nil {
		status.Error = err
		statusChan <- status
		return err
	}
	err = couchdbSchema.CreateViews()
	if err != nil {
		status.Error = err
		statusChan <- status
		return err
	}
	status.TablesReady = true
	statusChan <- status
	rows, err := exp.db.Query(fmt.Sprintf( // TODO w.pos AS lemma_pos !?
		"SELECT w.value, w.lemma, s.value AS sublemma, s.count AS sublemma_count, "+
			"w.pos, w.count, w.arf, w.pos as lemma_pos, m.count as lemma_count, m.arf as lemma_arf, "+
			"m.is_pname as lemma_is_pname "+
			"FROM %s_word AS w "+
			"JOIN %s_sublemma AS s ON s.value = w.sublemma AND s.lemma = w.lemma AND s.pos = w.pos "+
			"JOIN %s_lemma AS m ON m.value = s.lemma AND m.pos = s.pos "+
			"ORDER BY w.lemma, w.pos, w.value", exp.groupedName, exp.groupedName, exp.groupedName))
	if err != nil {
		status.Error = err
		statusChan <- status
		return err
	}
	return exp.processRows(rows, statusChan, status)
}

func (exp *Exporter) RunAsyncExportJob() (ExportJobInfo, error) {
	jobID, err := uuid.NewUUID()
	if err != nil {
		return ExportJobInfo{}, err
	}
	status := ExportJobInfo{
		ID:       jobID.String(),
		Type:     "qs-exporting",
		CorpusID: exp.groupedName,
		Start:    jobs.CurrentDatetime(),
		Update:   jobs.CurrentDatetime(),
		Finished: false,
		Args:     ExportJobInfoArgs{},
	}
	statusChan := make(chan exporterStatus)
	updateJobChan := exp.jobActions.AddJobInfo(&status)
	go func(runStatus ExportJobInfo) {
		for statUpd := range statusChan {
			runStatus.Result = statUpd
			runStatus.Error = statUpd.Error
			runStatus.Update = jobs.CurrentDatetime()
			updateJobChan <- &runStatus
		}
		runStatus.Update = jobs.CurrentDatetime()
		runStatus.Finished = true
		updateJobChan <- &runStatus
		close(updateJobChan)
	}(status)
	go func() {
		exp.exportValuesToCouchDB(statusChan)
	}()
	return status, nil
}

func NewExporter(
	conf *corpus.NgramDB,
	db *sql.DB,
	groupedName string,
	jobActions *jobs.Actions,
) *Exporter {
	return &Exporter{
		cb: &couchdb.ClientBase{
			BaseURL: conf.URL,
			DBName:  fmt.Sprintf("%s_sublemmas", groupedName),
		},
		db:          db,
		groupedName: groupedName,
		jobActions:  jobActions,
	}
}
