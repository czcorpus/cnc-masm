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
	"masm/v3/corpus"
	"masm/v3/liveattrs/laconf"
	"masm/v3/liveattrs/request/biblio"
	"masm/v3/liveattrs/utils"
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
	subcorpAttrs := laconf.GetSubcorpAttrs(laConf)
	selAttrs := make([]string, len(subcorpAttrs))
	for i, attr := range subcorpAttrs {
		selAttrs[i] = utils.ImportKey(attr)
	}

	sql1 := fmt.Sprintf(
		"SELECT %s FROM `%s_liveattrs_entry` WHERE %s = ? LIMIT 1",
		strings.Join(selAttrs, ", "),
		corpusInfo.GroupedName(),
		utils.ImportKey(corpusInfo.BibIDAttr),
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
			ans[utils.ExportKey(selAttrs[i])] = val.String
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
	subcorpAttrs := laconf.GetSubcorpAttrs(laConf)
	selAttrs := make([]string, len(subcorpAttrs))
	for i, attr := range subcorpAttrs {
		selAttrs[i] = utils.ImportKey(attr)
	}

	valuesPlaceholders := make([]string, len(qry.ItemIDs))
	for i := range qry.ItemIDs {
		valuesPlaceholders[i] = "?"
	}
	sql1 := fmt.Sprintf(
		"SELECT %s, %s FROM `%s_liveattrs_entry` WHERE %s IN (%s)",
		utils.ImportKey(corpusInfo.BibIDAttr),
		utils.ImportKey(corpusInfo.BibLabelAttr),
		corpusInfo.GroupedName(),
		utils.ImportKey(corpusInfo.BibIDAttr),
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

type DocumentRow struct {
	Idx    int               `json:"idx"`
	ID     string            `json:"id"`
	Label  string            `json:"label"`
	Attrs  map[string]string `json:"attrs"`
	NumPos int               `json:"numOfPos"`
}

type PageInfo struct {
	Page     int
	PageSize int
	MaxItems int
}

func (pinfo PageInfo) NumItems() int {
	if pinfo.PageSize == 0 {
		return pinfo.MaxItems
	}
	return pinfo.Page*pinfo.PageSize + 1
}

func (pinfo PageInfo) Offset() int {
	return (pinfo.Page - 1) * pinfo.PageSize
}

func (pinfo PageInfo) ToSQL() string {
	if pinfo.PageSize == 0 {
		return ""
	}
	return fmt.Sprintf(
		"LIMIT %d OFFSET %d",
		pinfo.PageSize,
		pinfo.Offset(),
	)
}

func createAttrSQLChunk(attrs []string) string {
	sql := strings.Builder{}
	for _, attr := range attrs {
		sql.WriteString(", ")
		sql.WriteString(utils.ImportKey(attr))
	}
	return sql.String()
}

func GetNumOfDocuments(db *sql.DB, corpusInfo *corpus.DBInfo) (int, error) {
	sql := fmt.Sprintf(
		"SELECT COUNT(*) FROM `%s_liveattrs_entry` WHERE corpus_id = ? GROUP BY ?",
		corpusInfo.GroupedName(),
	)
	row := db.QueryRow(sql, corpusInfo.GroupedName(), utils.ImportKey(corpusInfo.BibIDAttr))
	var ans int
	err := row.Scan(&ans)
	if err != nil {
		return 0, err
	}
	return ans, nil
}

func GetDocuments(
	db *sql.DB,
	corpusInfo *corpus.DBInfo,
	attrs []string,
	page PageInfo,
) ([]*DocumentRow, error) {
	attrSQL := createAttrSQLChunk(attrs)
	sql := fmt.Sprintf(
		"SELECT %s, %s, SUM(poscount) AS poscount %s "+
			"FROM `%s_liveattrs_entry` "+
			"WHERE corpus_id = ? "+
			"GROUP BY %s "+
			"%s",
		utils.ImportKey(corpusInfo.BibIDAttr),
		utils.ImportKey(corpusInfo.BibLabelAttr),
		attrSQL,
		corpusInfo.GroupedName(),
		utils.ImportKey(corpusInfo.BibIDAttr),
		page.ToSQL(),
	)
	rows, err := db.Query(sql, corpusInfo.GroupedName())
	if err != nil {
		return []*DocumentRow{}, err
	}
	if page.MaxItems == 0 {
		var err error
		page.MaxItems, err = GetNumOfDocuments(db, corpusInfo)
		if err != nil {
			return []*DocumentRow{}, err
		}
	}
	ans := make([]*DocumentRow, 0, page.NumItems())
	attrVals := make([]string, len(attrs))
	scanVals := make([]any, 3+len(attrs))

	i := page.Offset()
	for rows.Next() {
		docEntry := &DocumentRow{Idx: i}
		docEntry.Attrs = make(map[string]string)
		scanVals[0] = &docEntry.ID
		scanVals[1] = &docEntry.Label
		scanVals[2] = &docEntry.NumPos

		for i := range attrVals {
			scanVals[3+i] = &attrVals[i]
		}
		err := rows.Scan(scanVals...)
		if err != nil {
			return []*DocumentRow{}, err
		}
		for i := 3; i < len(scanVals); i++ {
			if v, ok := scanVals[i].(*string); ok {
				docEntry.Attrs[attrs[i-3]] = *v
			}
		}
		ans = append(ans, docEntry)
		i++
	}
	return ans, nil
}
