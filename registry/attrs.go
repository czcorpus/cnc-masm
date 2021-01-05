// Copyright 2021 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2021 Institute of the Czech National Corpus,
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

package registry

type multisep struct {
	Value       string `json:"value"`
	Description string `json:"description"`
}

type boolValue struct {
	Value      string `json:"value"`
	TargetType bool   `json:"targetType"`
}

var availBoolValues = []boolValue{
	{Value: "y", TargetType: true},
	{Value: "n", TargetType: false},
	{Value: "yes", TargetType: true},
	{Value: "no", TargetType: false},
}
