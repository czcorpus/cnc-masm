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

package subcmixer

import (
	"database/sql"
	"fmt"
	"masm/v3/general/collections"
	"masm/v3/liveattrs/utils"
	"math"
	"strings"

	"github.com/rs/zerolog/log"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/optimize/convex/lp"
)

type CorpusComposition struct {
	Error         string `json:"error,omitempty"`
	DocIDs        []int  `json:"docIds"`
	SizeAssembled int    `json:"sizeAssembled"`
	CategorySizes []int  `json:"categorySizes"`
}

type MetadataModel struct {
	db        *sql.DB
	tableName string
	cTree     *CategoryTree
	idAttr    string
	textSizes []int
	idMap     map[int]int
	numTexts  int
	b         []float64
	a         mat.Mutable
}

func (mm *MetadataModel) getAllConditions(node *CategoryTreeNode) [][2]string {
	sqlArgs := [][2]string{}
	for _, subl := range node.MetadataCondition {
		for _, mc := range subl.GetAtoms() {
			sqlArgs = append(sqlArgs, [2]string{mc.Attr(), mc.Value()})
		}
	}
	for _, child := range node.Children {
		sqlArgs = append(sqlArgs, mm.getAllConditions(child)...)
	}
	return sqlArgs
}

// List all the texts matching main corpus. This will be the
// base for the 'A' matrix in the optimization problem.
// In case we work with aligned corpora we still want
// the same result here as the non-aligned items from
// the primary corpus will not be selected in
// _init_ab() due to applied self JOIN
// (append_aligned_corp_sql())

// Also generate a map "db_ID -> row index" to be able
// to work with db-fetched subsets of the texts and
// matching them with the 'A' matrix (i.e. in a filtered
// result a record has a different index then in
// all the records list).
func (mm *MetadataModel) getTextSizes() ([]int, map[int]int, error) {
	allCond := mm.getAllConditions(mm.cTree.RootNode)
	allCondSQL := make([]string, len(allCond))
	allCondArgsSQL := make([]any, len(allCond))
	for i, v := range allCond {
		allCondSQL[i] = fmt.Sprintf("%s = ?", v[0])
		allCondArgsSQL[i] = v[1]
	}
	var sqle strings.Builder
	sqle.WriteString(fmt.Sprintf(
		"SELECT MIN(m1.id) AS db_id, SUM(poscount) FROM %s AS m1 ",
		mm.tableName,
	))
	args := []any{}
	sqle.WriteString(fmt.Sprintf(
		" WHERE m1.corpus_id = ? AND (%s) GROUP BY %s ORDER BY db_id",
		strings.Join(allCondSQL, " OR "),
		utils.ImportKey(mm.idAttr),
	))
	args = append(args, mm.cTree.CorpusID)
	args = append(args, allCondArgsSQL...)
	sizes := []int{}
	idMap := make(map[int]int)
	rows, err := mm.db.Query(sqle.String(), args...)
	if err != nil {
		return []int{}, map[int]int{}, err
	}
	i := 0
	for rows.Next() {
		var minID, minCount int
		err := rows.Scan(&minID, &minCount)
		if err != nil {
			return []int{}, map[int]int{}, err
		}
		sizes = append(sizes, minCount)
		idMap[minID] = i
		i++
	}
	return sizes, idMap, nil
}

func (mm *MetadataModel) initABNonalign(usedIDs *collections.Set[int]) {
	// Now we process items with no aligned counterparts.
	// In this case we must define a condition which will be
	// fulfilled iff X[i] == 0
	for k, v := range mm.idMap {
		if !usedIDs.Contains(k) {
			for i := 1; i < len(mm.b); i++ {
				mult := 10000.0
				if mm.b[i] > 0 {
					mult = 2.0
				}
				mm.a.Set(i, v, mm.b[i]*mult)
			}
		}
	}
}

func (mm *MetadataModel) PrintA(m mat.Matrix) {
	fa := mat.Formatted(m, mat.Prefix(""), mat.Squeeze())
	fmt.Println(fa)
}

func (mm *MetadataModel) initAB(node *CategoryTreeNode, usedIDs *collections.Set[int]) error {
	if len(node.MetadataCondition) > 0 {
		sqlItems := []string{}
		for _, subl := range node.MetadataCondition {
			for _, mc := range subl.GetAtoms() {
				sqlItems = append(
					sqlItems,
					fmt.Sprintf("m1.%s %s ?", mc.Attr(), mc.OpSQL()),
				)
			}
		}
		sqlArgs := []any{}
		var sqle strings.Builder
		sqle.WriteString(fmt.Sprintf(
			"SELECT MIN(m1.id) AS db_id, SUM(m1.poscount) FROM %s AS m1 ",
			mm.tableName,
		))
		mm.cTree.appendAlignedCorpSQL(sqle, &sqlArgs)
		sqle.WriteString(fmt.Sprintf(
			"WHERE %s AND m1.corpus_id = ? GROUP BY %s ORDER BY db_id",
			strings.Join(sqlItems, " AND "), utils.ImportKey(mm.idAttr),
		))
		// mc.value for subl in node.metadata_condition for mc in subl
		for _, subl := range node.MetadataCondition {
			for _, mc := range subl.GetAtoms() {
				sqlArgs = append(sqlArgs, mc.Value())
			}
		}
		sqlArgs = append(sqlArgs, mm.cTree.CorpusID)
		rows, err := mm.db.Query(sqle.String(), sqlArgs...)
		if err != nil {
			return err
		}
		for rows.Next() {
			var minID, minCount int
			err := rows.Scan(&minID, &minCount)
			if err != nil {
				return err
			}
			mcf := float64(minCount)
			mm.a.Set(node.NodeID-1, mm.idMap[minID], mcf)
			usedIDs.Add(minID)
			mm.b[node.NodeID-1] = float64(node.Size)
		}
	}
	if len(node.Children) > 0 {
		for _, child := range node.Children {
			mm.initAB(child, usedIDs)
		}
	}
	return nil
}

func (mm *MetadataModel) initXLimit() {
	for i := 0; i < mm.numTexts; i++ {
		mm.a.Set(i+mm.cTree.NumCategories()-1, i, 1.0)
	}
	for i := 0; i < mm.numTexts+mm.cTree.NumCategories()-1; i++ {
		mm.a.Set(i, mm.numTexts+i, 1)
	}
	for i := mm.cTree.NumCategories() - 1; i < len(mm.b); i++ {
		mm.b[i] = 1.0
	}
}

func (mm *MetadataModel) isZeroVector(m []float64) bool {
	for i := 0; i < len(m); i++ {
		if m[i] > 0 {
			return false
		}
	}
	return true
}

func (mm *MetadataModel) getCategorySize(results []float64, catID int) (float64, error) {
	if rva, ok := mm.a.(mat.RowViewer); ok {
		catIDRow := rva.RowView(catID)
		ans := mat.Dot(
			mat.NewVecDense(len(results), results),
			catIDRow,
		)
		return ans, nil
	}
	return -1, fmt.Errorf("cannot calculate category size - matrix is not a RowViewer")
}

func (mm *MetadataModel) getAssembledSize(results []float64) float64 {
	var ans float64
	for i := 0; i < mm.numTexts; i++ {
		ans += results[i] * float64(mm.textSizes[i])
	}
	return ans
}

func (mm *MetadataModel) Solve() *CorpusComposition {
	if mm.isZeroVector(mm.b) {
		return &CorpusComposition{}
	}
	c := make([]float64, 2*mm.numTexts+mm.cTree.NumCategories()-1)
	for i := 0; i < mm.numTexts; i++ {
		c[i] = -1.0
	}
	var simplexErr error
	_, variables, simplexErr := lp.Simplex(c, mm.a, mm.b, 0, nil)
	selections := mapSlice(
		variables,
		func(v float64) float64 { return math.RoundToEven(v) },
	)
	categorySizes := make([]float64, mm.cTree.NumCategories()-1)
	for c := 0; c < mm.cTree.NumCategories()-1; c++ {
		catSize, err := mm.getCategorySize(selections, c)
		if err != nil {
			log.Err(err).Msgf("Failed to get cat size")
		}
		categorySizes[c] = catSize
	}
	docIDs := make([]int, 0, len(selections))
	for docID, idx := range mm.idMap {
		if selections[idx] == 1 {
			docIDs = append(docIDs, docID)
		}
	}
	var errDesc string
	if simplexErr != nil {
		errDesc = simplexErr.Error()
	}
	return &CorpusComposition{
		Error:         errDesc,
		DocIDs:        docIDs,
		SizeAssembled: int(mm.getAssembledSize(selections)),
		CategorySizes: mapSlice(categorySizes, func(v float64) int { return int(v) }),
	}
}

func NewMetadataModel(
	metaDB *sql.DB,
	tableName string,
	cTree *CategoryTree,
	idAttr string,
) (*MetadataModel, error) {
	ans := &MetadataModel{
		db:        metaDB,
		tableName: tableName,
		cTree:     cTree,
		idAttr:    idAttr,
	}
	ts, idMap, err := ans.getTextSizes()
	if err != nil {
		return nil, err
	}
	ans.idMap = idMap
	ans.textSizes = ts
	ans.numTexts = len(ts)
	ans.b = make([]float64, ans.cTree.NumCategories()-1+ans.numTexts)
	usedIDs := collections.NewSet[int]()
	ans.a = mat.NewDense(
		ans.cTree.NumCategories()-1+ans.numTexts,
		ans.cTree.NumCategories()-1+2*ans.numTexts,
		make([]float64, (ans.numTexts+ans.cTree.NumCategories()-1)*(2*ans.numTexts+ans.cTree.NumCategories()-1)),
	)
	ans.initAB(cTree.RootNode, usedIDs)
	// for items without aligned counterparts we create
	// conditions fulfillable only for x[i] = 0
	ans.initABNonalign(usedIDs)
	ans.initXLimit()

	return ans, nil
}
