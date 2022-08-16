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
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
)

type CategoryTreeNode struct {
	NodeID            int
	ParentID          Maybe[int]
	Ratio             float64
	MetadataCondition []AbstractExpression
	Size              int
	ComputedBounds    any // TODO type
	Children          []*CategoryTreeNode
}

func (ctn *CategoryTreeNode) String() string {
	return fmt.Sprintf(
		"CategoryTreeNode(id: %d, parent: %v, ratio: %01.3f, metadata: %v, size: %d, num children: %d)",
		ctn.NodeID, ctn.ParentID, ctn.Ratio, ctn.MetadataCondition, ctn.Size, len(ctn.Children),
	)
}

func (ctn *CategoryTreeNode) printTree(node *CategoryTreeNode, indent int) {
	fmt.Printf("%s%v\n", strings.Repeat(" ", indent), node)
	for _, child := range node.Children {
		ctn.printTree(child, indent+4)
	}
}

func (ctn *CategoryTreeNode) PrintTree() {
	ctn.printTree(ctn, 0)
}

func (ctn *CategoryTreeNode) HasChildren() bool {
	return len(ctn.Children) > 0
}

// ---

type ExpressionJoin struct {
	Items []AbstractExpression
	Op    string
}

func (ej *ExpressionJoin) Add(item AbstractExpression) {
	ej.Items = append(ej.Items, item)
}

func (ej *ExpressionJoin) IsComposed() bool {
	return true
}

func collectAtomsRecursive(current any) []any {
	switch tCurrent := current.(type) {
	case ExpressionJoin:
		for _, item := range tCurrent.Items {
			var ans []any
			ans = append(ans, collectAtomsRecursive(item)...)
			return ans
		}
	case CategoryExpression:
		return []any{&tCurrent}
	}
	log.Debug().Msg("possibly invalid expression encoutered")
	return []any{}
}

func (ej *ExpressionJoin) GetAtoms() []AbstractAtomicExpression {
	tmp := collectAtomsRecursive(ej)
	ans := make([]AbstractAtomicExpression, len(tmp))
	for i, v := range tmp {
		t, ok := v.(CategoryExpression)
		if ok {
			ans[i] = &t

		} else {
			log.Debug().Msg("possibly invalid expression")
		}
	}
	return ans
}

func (ej *ExpressionJoin) IsEmpty() bool {
	return len(ej.Items) == 0 && ej.Op == ""
}

func (ej *ExpressionJoin) Negate() AbstractExpression {
	var newOp string
	if ej.Op == "OR" {
		newOp = "AND"

	} else if ej.Op == "AND" {
		newOp = "OR"
	}
	expr := &ExpressionJoin{
		Op:    newOp,
		Items: make([]AbstractExpression, len(ej.Items)),
	}
	copy(expr.Items, ej.Items)
	return expr
}

func (ej *ExpressionJoin) OpSQL() string {
	return ej.Op
}

func (ej *ExpressionJoin) Attr() string {
	return ""
}
