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

package dictionary

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"masm/v3/common"
	"masm/v3/jobs"
	"masm/v3/liveattrs/db/freqdb"
	"regexp"
	"strings"
)

const (
	maxExpectedNumMatchingLemmas = 30
)

var (
	keyAlphabet       = []byte{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z', 'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z'}
	validMVWordRegexp = regexp.MustCompile(`^[\sA-Za-z0-9áÁéÉěĚšŠčČřŘžŽýÝíÍúÚůťŤďĎňŇóÓ\-\|]+$`)
	validWordRegexp   = regexp.MustCompile(`^[\sA-Za-z0-9áÁéÉěĚšŠčČřŘžŽýÝíÍúÚůťŤďĎňŇóÓ\-]+$`)
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
	db                 *sql.DB
	groupedName        string
	jobActions         *jobs.Actions
	multiValuesEnabled bool
	readAccessUsers    []string
}

func isValidWord(w string, enableMultivalues bool) bool {
	if enableMultivalues {
		return validMVWordRegexp.MatchString(w)
	}
	return validWordRegexp.MatchString(w)
}

func processRowsSync(rows *sql.Rows, enableMultivalues bool) ([]Lemma, error) {

	var idBase, procRecords int
	matchingLemmas := make([]Lemma, 0, maxExpectedNumMatchingLemmas)
	var currLemma *Lemma
	sublemmas := make(map[string]int)

	for rows.Next() {
		var lemmaValue, sublemmaValue, wordValue, lemmaPos, wordPos string
		var sublemmaCount, lemmaCount, wordCount int
		var lemmaArf, wordArf float64
		var isPname bool
		err := rows.Scan(
			&wordValue, &lemmaValue, &sublemmaValue, &wordCount,
			&wordPos, &wordArf)
		if err != nil {
			return []Lemma{}, fmt.Errorf("failed to process dictionary rows: %w", err)
		}
		if isValidWord(lemmaValue, enableMultivalues) {
			newLemma := lemmaValue
			newPos := lemmaPos
			if currLemma == nil || newLemma != currLemma.Lemma || newPos != currLemma.PoS {
				if currLemma != nil {
					matchingLemmas = append(matchingLemmas, *currLemma)
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

		}
		procRecords++

	}
	if procRecords == 0 {
		return []Lemma{}, fmt.Errorf("there were no dictionary rcords to process")
	}
	if currLemma != nil {
		matchingLemmas = append(matchingLemmas, *currLemma)
	}
	return matchingLemmas, nil
}

type SearchOptions struct {
	Lemma            string
	Sublemma         string
	Word             string
	AnyValue         string
	AllowMultivalues bool
}

func SearchWithSublemma(v string) SearchOption {
	return func(c *SearchOptions) {
		c.Sublemma = v
	}
}

func SearchWithLemma(v string) SearchOption {
	return func(c *SearchOptions) {
		c.Lemma = v
	}
}

func SearchWithWord(v string) SearchOption {
	return func(c *SearchOptions) {
		c.Word = v
	}
}

func SearchWithAnyValue(v string) SearchOption {
	return func(c *SearchOptions) {
		c.AnyValue = v
	}
}

func SearchWithMultivalues() SearchOption {
	return func(c *SearchOptions) {
		c.AllowMultivalues = true
	}
}

type SearchOption func(c *SearchOptions)

func Search(
	ctx context.Context,
	db *sql.DB,
	groupedName string,
	opts ...SearchOption,
) ([]Lemma, error) {

	status := exporterStatus{}
	status.TablesReady = true
	whereSQL := make([]string, 0, 5)
	whereArgs := make([]any, 0, 5)
	whereSQL = append(whereSQL, "w.pos != ?")
	whereArgs = append(whereArgs, freqdb.NonWordCSCNC2020Tag)
	var srchOpts SearchOptions
	for _, opt := range opts {
		opt(&srchOpts)
	}
	if srchOpts.Lemma != "" {
		whereSQL = append(whereSQL, "m.value = ?")
		whereArgs = append(whereArgs, srchOpts.Lemma)
	}
	if srchOpts.Sublemma != "" {
		whereSQL = append(whereSQL, "s.value = ?")
		whereArgs = append(whereArgs, srchOpts.Sublemma)
	}
	if srchOpts.Word != "" {
		whereSQL = append(whereSQL, "w.value = ?")
		whereArgs = append(whereArgs, srchOpts.Word)
	}
	if srchOpts.AnyValue != "" {
		whereSQL = append(whereSQL, "s.value = ?")
		whereArgs = append(whereArgs, srchOpts.AnyValue)
	}
	rows, err := db.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT w.value, w.lemma, w.sublemma, w.count, "+
				"w.pos, w.arf "+
				"FROM %s_word AS w "+
				"JOIN %s_term_search AS s ON s.word_id = w.id "+
				"WHERE %s "+
				"ORDER BY w.lemma, w.pos, w.sublemma, w.value",
			groupedName,
			groupedName,
			strings.Join(whereSQL, " AND "),
		),
		whereArgs...,
	)
	if err != nil {
		return []Lemma{}, fmt.Errorf("failed to search dict. values: %w", err)
	}
	return processRowsSync(rows, srchOpts.AllowMultivalues)
}
