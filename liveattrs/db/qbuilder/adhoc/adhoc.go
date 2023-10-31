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

package adhoc

import (
	"fmt"
	"masm/v3/corpus"
	"masm/v3/liveattrs/request/query"
	"strings"
)

// SubcSize is a generator for an SQL query + args for obtaining a subcorpus based
// on ad-hoc selection of text types
type SubcSize struct {
	CorpusInfo          *corpus.DBInfo
	AttrMap             query.Attrs
	AlignedCorpora      []string
	EmptyValPlaceholder string
}

// Query generates the result
// Please note that this is largely similar to laquery.AttrArgs.ExportSQL()
func (ssize *SubcSize) Query() (ansSQL string, whereValues []any) {
	joinSQL := make([]string, 0, 10)
	whereSQL := []string{
		"t1.corpus_id = ?",
		"t1.poscount is NOT NULL",
	}
	whereValues = []any{ssize.CorpusInfo.Name}
	for i, item := range ssize.AlignedCorpora {
		iOffs := i + 2
		joinSQL = append(
			joinSQL,
			fmt.Sprintf(
				"JOIN `%s_liveattrs_entry` AS t%d ON t1.item_id = t%d.item_id",
				ssize.CorpusInfo.Name, iOffs, iOffs,
			),
		)
		whereSQL = append(
			whereSQL,
			fmt.Sprintf("t%d.corpus_id = ?", iOffs),
		)
		whereValues = append(whereValues, item)
	}

	aargs := PredicateArgs{
		data:                ssize.AttrMap,
		emptyValPlaceholder: ssize.EmptyValPlaceholder,
		bibLabel:            ssize.CorpusInfo.BibLabelAttr,
	}
	where2, args2 := aargs.ExportSQL("t1", ssize.CorpusInfo.Name)
	whereSQL = append(whereSQL, where2)
	whereValues = append(whereValues, args2...)
	ansSQL = fmt.Sprintf(
		"SELECT SUM(t1.poscount) FROM `%s_liveattrs_entry` AS t1 %s WHERE %s",
		ssize.CorpusInfo.GroupedName(),
		strings.Join(joinSQL, " "),
		strings.Join(whereSQL, " AND "),
	)
	return
}
