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

/*
Implementation note:
The current implementation is not general enough as it expects specific
tagset and positional attribute types and order.
*/

package freqdb

import (
	"database/sql"
	"fmt"
	"masm/v3/jobs"
	"masm/v3/liveattrs/db"
	"math"
	"time"

	"github.com/czcorpus/vert-tagextract/v2/ptcount/modders"
	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

const (
	reportEachNthItem   = 50000
	duplicateRowErrNo   = 1062
	NonWordCSCNC2020Tag = "X@-------------"
)

type NgramFreqGenerator struct {
	db          *sql.DB
	groupedName string
	corpusName  string
	posFn       *modders.StringTransformerChain
	jobActions  *jobs.Actions
}

func (nfg *NgramFreqGenerator) createTables(tx *sql.Tx) error {
	_, err := tx.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s_word", nfg.groupedName))
	if err != nil {
		return err
	}
	_, err = tx.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s_sublemma", nfg.groupedName))
	if err != nil {
		return err
	}
	_, err = tx.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s_lemma", nfg.groupedName))
	if err != nil {
		return err
	}
	_, err = tx.Exec(fmt.Sprintf(
		`CREATE TABLE %s_lemma (value VARCHAR(80), pos VARCHAR(80), count INTEGER, arf FLOAT,
			is_pname TINYINT, PRIMARY KEY(value, pos))`,
		nfg.groupedName))
	if err != nil {
		return err
	}
	_, err = tx.Exec(fmt.Sprintf(
		`CREATE TABLE %s_sublemma (value VARCHAR(80), lemma VARCHAR(80), pos VARCHAR(80), count INTEGER,
			PRIMARY KEY (value, lemma, pos),
			FOREIGN KEY (lemma, pos) REFERENCES %s_lemma(value, pos))`,
		nfg.groupedName, nfg.groupedName))
	if err != nil {
		return err
	}
	_, err = tx.Exec(fmt.Sprintf(
		`CREATE TABLE %s_word (value VARCHAR(80), lemma VARCHAR(80), sublemma VARCHAR(80), pos VARCHAR(80),
			count INTEGER, arf FLOAT, PRIMARY KEY (value, lemma, sublemma, pos),
			FOREIGN KEY (sublemma, lemma, pos) REFERENCES %s_sublemma(value, lemma, pos))`,
		nfg.groupedName, nfg.groupedName))
	if err != nil {
		return err
	}
	return nil
}

// procLine processes current ngRecord item (= vertical file line containing a token data)
// with respect to currently processed currLemma and collected sublemmas.
// Please note that should the method to work as expected, it is critical to process
// the token data ordered by word, sublemma, lemma. Otherwise, the procLine method
// won't be able to detect end of the current lemma forms (and sublemmas).
func (nfg *NgramFreqGenerator) procLine(
	tx *sql.Tx,
	item *ngRecord,
	currLemma *ngRecord,
	words []*ngRecord,
	sublemmas map[string]int,
) ([]*ngRecord, map[string]int, *ngRecord, error) {
	if currLemma == nil || item.lemma != currLemma.lemma || item.tag != currLemma.tag {
		// note: we take advantage of the fact that  currLemma == nil <=> len(words) == 0
		// (see currLemma.* accesses below)
		if len(words) > 0 {
			_, err := tx.Exec(
				fmt.Sprintf(
					"INSERT INTO %s_lemma (value, pos, count, arf, is_pname) VALUES (?, ?, ?, ?, ?)",
					nfg.groupedName,
				),
				currLemma.lemma, nfg.posFn.Transform(currLemma.tag), getLemmaTotal(words), getLemmaArf(words),
				startsWithUpcase(currLemma.lemma),
			)
			terr, ok := err.(*mysql.MySQLError)
			if ok && terr.Number == duplicateRowErrNo {
				tx.Exec(fmt.Sprintf(
					`UPDATE %s_lemma
					SET count = count + ?, arf = arf + ?
					WHERE value = ? AND pos = ?`, nfg.groupedName),
					getLemmaTotal(words), getLemmaArf(words), currLemma.lemma,
					nfg.posFn.Transform(currLemma.tag))

			} else if err != nil {
				return nil, map[string]int{}, nil, err
			}
			for subl := range sublemmas {
				_, err := tx.Exec(fmt.Sprintf(
					`INSERT INTO %s_sublemma (value, lemma, pos, count) VALUES (?, ?, ?, ?)`,
					nfg.groupedName),
					subl, currLemma.lemma, nfg.posFn.Transform(currLemma.tag), sublemmas[subl])
				terr, ok := err.(*mysql.MySQLError)
				if ok && terr.Number == duplicateRowErrNo {
					_, err := tx.Exec(fmt.Sprintf(
						`UPDATE %s_sublemma SET count = count + ?
						WHERE value = ? AND lemma = ? AND pos = ?`, nfg.groupedName),
						sublemmas[subl], subl, currLemma.lemma, nfg.posFn.Transform(currLemma.tag))
					if err != nil {
						return nil, map[string]int{}, nil, err
					}
				} else if err != nil {
					return nil, map[string]int{}, nil, err
				}
			}
			for _, word := range words {
				_, err := tx.Exec(fmt.Sprintf(
					`INSERT INTO %s_word (value, lemma, sublemma, pos, count, arf)
					VALUES (?, ?, ?, ?, ?, ?)`, nfg.groupedName),
					word.word, word.lemma, word.sublemma, nfg.posFn.Transform(word.tag), word.abs, word.arf)
				terr, ok := err.(*mysql.MySQLError)
				if ok && terr.Number == duplicateRowErrNo {
					_, err := tx.Exec(fmt.Sprintf(
						`UPDATE %s_word
						SET count = count + ?, arf = arf + ?
						WHERE value = ? AND lemma = ? AND pos = ?`, nfg.groupedName),
						word.abs, word.arf, word.word, word.lemma, nfg.posFn.Transform(word.tag))
					if err != nil {
						return nil, map[string]int{}, nil, err
					}
				}
			}
		}
		currLemma = item
		words = []*ngRecord{}
		sublemmas = make(map[string]int)
	}
	words = append(words, item)
	return words, sublemmas, currLemma, nil
}

func (nfg *NgramFreqGenerator) findTotalNumLines() (int, error) {
	// TODO the following query is not general enough
	row := nfg.db.QueryRow(fmt.Sprintf(
		"SELECT COUNT(*) "+
			"FROM %s_colcounts "+
			"WHERE col4 <> ? ", nfg.groupedName), NonWordCSCNC2020Tag)
	if row.Err() != nil {
		return -1, row.Err()
	}
	var ans int
	err := row.Scan(&ans)
	if err != nil {
		return -1, err
	}
	return ans, nil
}

// run generates n-grams (structured into 'word', 'lemma', 'sublemma') into intermediate database
// An existing database transaction must be provided along with current calculation status (which is
// progressively updated) and a status channel where the status is sent each time some significant
// update is encountered (typically - a chunk of items is finished or an error occurs)
func (nfg *NgramFreqGenerator) run(tx *sql.Tx, currStatus *genNgramsStatus, statusChan chan<- genNgramsStatus) error {

	total, err := nfg.findTotalNumLines()
	if err != nil {
		return fmt.Errorf("failed to run n-gram generator: %w", err)
	}
	estim, err := db.EstimateProcTimeSecs(nfg.db, "ngrams", total)
	if err == db.ErrorEstimationNotAvail {
		log.Warn().Msgf("processing estimation not (yet) available for %s", nfg.corpusName)
		estim = -1
	} else if err != nil {
		return fmt.Errorf("failed to run n-gram generator: %w", err)
	}
	if estim > 0 {
		currStatus.TimeEstimationSecs = estim
		statusChan <- *currStatus
	}
	log.Info().Msgf(
		"About to process %d lines of raw n-grams for corpus %s. Time estimation (seconds): %d",
		total, nfg.corpusName, estim)
	var numStop int
	t0 := time.Now()
	// TODO the following query is not general enough
	rows, err := nfg.db.Query(fmt.Sprintf(
		"SELECT col0, col2, col3, col4, `count` AS abs, arf "+
			"FROM %s_colcounts "+
			"WHERE col4 <> ? "+
			"ORDER BY col2, col3, col4, col0 ", nfg.groupedName), NonWordCSCNC2020Tag)
	if err != nil {
		return fmt.Errorf("failed to run n-gram generator: %w", err)
	}
	var currLemma *ngRecord
	words := make([]*ngRecord, 0)
	sublemmas := make(map[string]int)
	var numProcessed int
	for rows.Next() {
		rec := new(ngRecord)
		// 'word', 'lemma', 'sublemma', 'tag', 'abs', 'arf'
		err := rows.Scan(&rec.word, &rec.lemma, &rec.sublemma, &rec.tag, &rec.abs, &rec.arf)
		if err != nil {
			return fmt.Errorf("failed to run n-gram generator: %w", err)
		}
		if isStopNgram(rec.lemma) {
			numStop++
			continue
		}
		words, sublemmas, currLemma, err = nfg.procLine(tx, rec, currLemma, words, sublemmas)
		if err != nil {
			return fmt.Errorf("failed to run n-gram generator: %w", err)
		}
		sublemmas[rec.sublemma] += 1
		numProcessed++
		if numProcessed%reportEachNthItem == 0 {
			procTime := time.Since(t0).Seconds()
			currStatus.NumProcLines = numProcessed
			currStatus.AvgSpeedItemsPerSec = int(math.RoundToEven(float64(numProcessed) / procTime))
			statusChan <- *currStatus
		}
	}
	// proc the last element
	lastRec := new(ngRecord)
	nfg.procLine(tx, lastRec, currLemma, words, sublemmas)
	numProcessed++
	currStatus.NumProcLines = numProcessed
	procTime := time.Since(t0).Seconds()
	currStatus.AvgSpeedItemsPerSec = int(math.RoundToEven(float64(numProcessed) / procTime))
	err = db.AddProcTimeEntry(
		nfg.db,
		"ngrams",
		total,
		numProcessed,
		procTime,
	)
	if err != nil {
		log.Err(err).Msg("failed to write proc_time statistics")
	}
	statusChan <- *currStatus
	log.Info().Msgf("num stop words: %d", numStop)
	return nil
}

// generate (synchronously) generates n-grams from raw liveattrs data
// provided statusChan is closed by the method once
// the operation finishes
func (nfg *NgramFreqGenerator) generate(statusChan chan<- genNgramsStatus) error {
	defer close(statusChan)
	var status genNgramsStatus
	tx, err := nfg.db.Begin()
	if err != nil {
		tx.Rollback()
		status.Error = err
		statusChan <- status
		return err
	}
	err = nfg.createTables(tx)
	status.TablesReady = true
	statusChan <- status
	if err != nil {
		tx.Rollback()
		status.Error = err
		statusChan <- status
		return err
	}
	err = nfg.run(tx, &status, statusChan)
	if err != nil {
		tx.Rollback()
		status.Error = err
		statusChan <- status
		return err
	}
	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		status.Error = err
		statusChan <- status
		return err
	}
	return nil
}

func (nfg *NgramFreqGenerator) GenerateAsync(corpusID string) (NgramJobInfo, chan jobs.GeneralJobInfo, error) {
	jobID, err := uuid.NewUUID()
	if err != nil {
		return NgramJobInfo{}, nil, err
	}
	status := NgramJobInfo{
		ID:       jobID.String(),
		Type:     "ngram-generating",
		CorpusID: corpusID,
		Start:    jobs.CurrentDatetime(),
		Update:   jobs.CurrentDatetime(),
		Finished: false,
		Args:     NgramJobInfoArgs{},
	}
	statusChan := make(chan genNgramsStatus)
	updateJobChan := nfg.jobActions.AddJobInfo(&status)
	go func(runStatus NgramJobInfo) {
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
		nfg.generate(statusChan)
	}()
	return status, updateJobChan, nil
}

func NewNgramFreqGenerator(
	db *sql.DB,
	jobActions *jobs.Actions,
	groupedName string,
	corpusName string,
	posFn *modders.StringTransformerChain,
) *NgramFreqGenerator {
	return &NgramFreqGenerator{
		db:          db,
		jobActions:  jobActions,
		groupedName: groupedName,
		corpusName:  corpusName,
		posFn:       posFn,
	}
}
