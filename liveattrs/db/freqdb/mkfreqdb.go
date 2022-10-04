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

package freqdb

import (
	"database/sql"
	"fmt"
	"math"
	"time"

	"github.com/czcorpus/vert-tagextract/v2/ptcount/modders"
	"github.com/go-sql-driver/mysql"
	"github.com/rs/zerolog/log"
)

const (
	chunkSize = 50000
)

type NgramFreqGenerator struct {
	db          *sql.DB
	groupedName string
	corpusName  string
	posFn       *modders.StringTransformerChain
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
			if ok && terr.Number == 1062 {
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
				if ok && terr.Number == 1062 {
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
				if ok && terr.Number == 1 { // TODO !!!!! find proper error code
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
	row := nfg.db.QueryRow(fmt.Sprintf(
		"SELECT COUNT(*) "+
			"FROM %s_colcounts "+
			"WHERE col4 <> 'X@-------------' ", nfg.groupedName))
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

func (nfg *NgramFreqGenerator) run(tx *sql.Tx) error {

	total, err := nfg.findTotalNumLines()
	if err != nil {
		return err
	}
	log.Info().Msgf("about to process %d lines of raw n-grams for corpus %s", total, nfg.corpusName)
	chunkEmpty := false
	var numStop, offset int
	for !chunkEmpty {
		t0 := time.Now()
		rows, err := nfg.db.Query(fmt.Sprintf(
			"SELECT col0, col2, col3, col4, `count` AS abs, arf "+
				"FROM %s_colcounts "+
				"WHERE col4 <> 'X@-------------' "+
				"ORDER BY col2, col3, col4, col0 "+
				"LIMIT ? OFFSET ?", nfg.groupedName), chunkSize, offset)
		if err != nil {
			return err
		}
		var currLemma *ngRecord
		words := make([]*ngRecord, 0)
		sublemmas := make(map[string]int)
		chunkEmpty = true
		for rows.Next() {
			chunkEmpty = false
			rec := new(ngRecord)
			// 'word', 'lemma', 'sublemma', 'tag', 'abs', 'arf'
			err := rows.Scan(&rec.word, &rec.lemma, &rec.sublemma, &rec.tag, &rec.abs, &rec.arf)
			if err != nil {
				return err
			}
			if isStopNgram(rec.lemma) {
				numStop++
				continue
			}
			words, sublemmas, currLemma, err = nfg.procLine(tx, rec, currLemma, words, sublemmas)
			if err != nil {
				return err
			}
			sublemmas[rec.sublemma] += 1
		}
		// proc the last element
		lastRec := new(ngRecord)
		nfg.procLine(tx, lastRec, currLemma, words, sublemmas)
		log.Debug().Msgf(
			"Processed chunk %d - %d, processing speed: %d items / sec.",
			offset, offset+chunkSize, int(math.RoundToEven(chunkSize/time.Since(t0).Seconds())))
		offset += chunkSize
	}
	log.Info().Msgf("num stop words: %d", numStop)
	return nil
}

func (nfg *NgramFreqGenerator) Generate() (*genNgramsResult, error) {
	tx, err := nfg.db.Begin()
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	err = nfg.createTables(tx)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	err = nfg.run(tx)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	err = tx.Commit()
	return &genNgramsResult{}, err
}

func NewNgramFreqGenerator(
	db *sql.DB,
	groupedName string,
	corpusName string,
	posFn *modders.StringTransformerChain,
) *NgramFreqGenerator {
	return &NgramFreqGenerator{
		db:          db,
		groupedName: groupedName,
		corpusName:  corpusName,
		posFn:       posFn,
	}
}
