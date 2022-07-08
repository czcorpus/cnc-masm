// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
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

package db

import (
	"database/sql"
	"fmt"
	"masm/v3/cnf"
	"masm/v3/corpus"
	"masm/v3/liveattrs/request/biblio"
	"strings"

	vteconf "github.com/czcorpus/vert-tagextract/v2/cnf"
)

func GetBibliography(
	db *sql.DB,
	corpusInfo *corpus.DBInfo,
	laConf *vteconf.VTEConf,
	qry biblio.Payload,
) (map[string]string, error) {
	if corpusInfo.BibIDAttr == "" {
		return map[string]string{}, fmt.Errorf("cannot get bibliography for %s - bibIdAttr/Label not defined", corpusInfo.Name)
	}
	subcorpAttrs := cnf.GetSubcorpAttrs(laConf)
	selAttrs := make([]string, len(subcorpAttrs))
	for i, attr := range subcorpAttrs {
		selAttrs[i] = ImportKey(attr)
	}

	sql1 := fmt.Sprintf(
		"SELECT %s FROM `%s_liveattrs_entry` WHERE %s = ? LIMIT 1",
		strings.Join(selAttrs, ", "),
		corpusInfo.GroupedName(),
		ImportKey(corpusInfo.BibIDAttr),
	)

	rows := db.QueryRow(sql1, qry.ItemID)
	ans := make(map[string]string)
	ansVals := make([]sql.NullString, len(selAttrs))
	ansPvals := make([]any, len(selAttrs))
	for i := range ansVals {
		ansPvals[i] = &ansVals[i]
	}
	err := rows.Scan(ansPvals...)
	if err == sql.ErrNoRows {
		return map[string]string{}, ErrorEmptyResult
	}
	if err != nil {
		return map[string]string{}, err
	}
	for i, val := range ansVals {
		if val.Valid {
			ans[ExportKey(selAttrs[i])] = val.String
		}
	}
	return ans, nil
}

func FindBibTitles(
	db *sql.DB,
	corpusInfo *corpus.DBInfo,
	laConf *vteconf.VTEConf,
	qry biblio.PayloadList,
) (map[string]string, error) {
	subcorpAttrs := cnf.GetSubcorpAttrs(laConf)
	selAttrs := make([]string, len(subcorpAttrs))
	for i, attr := range subcorpAttrs {
		selAttrs[i] = ImportKey(attr)
	}

	valuesPlaceholders := make([]string, len(qry.ItemIDs))
	for i := range qry.ItemIDs {
		valuesPlaceholders[i] = "?"
	}
	sql1 := fmt.Sprintf(
		"SELECT %s, %s FROM `%s_liveattrs_entry` WHERE %s IN (%s)",
		ImportKey(corpusInfo.BibIDAttr),
		ImportKey(corpusInfo.BibLabelAttr),
		corpusInfo.GroupedName(),
		ImportKey(corpusInfo.BibIDAttr),
		strings.Join(valuesPlaceholders, ", "),
	)
	sqlVals := make([]any, len(qry.ItemIDs))
	for i, v := range qry.ItemIDs {
		sqlVals[i] = v
	}

	rows, err := db.Query(sql1, sqlVals...)
	ans := make(map[string]string)
	if err == sql.ErrNoRows {
		return ans, ErrorEmptyResult

	} else if err != nil {
		return map[string]string{}, err
	}

	for rows.Next() {
		var bibIdVal, bibLabelVal string
		err = rows.Scan(&bibIdVal, &bibLabelVal)
		if err != nil {
			return nil, err
		}
		ans[bibIdVal] = bibLabelVal
	}

	return ans, nil
}
