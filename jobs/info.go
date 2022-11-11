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

package jobs

import (
	"golang.org/x/text/message"
)

func extractJobDescription(printer *message.Printer, info GeneralJobInfo) string {
	desc := "??"
	switch info.GetType() {
	case "ngram-and-qs-generating":
		desc = printer.Sprintf("N-grams and query suggestion data generation")
	case "liveattrs":
		desc = printer.Sprintf("Live attributes data extraction and generation")
	case "dummy-job":
		desc = printer.Sprintf("Testing and debugging empty job")
	default:
		desc = printer.Sprintf("Unknown job")
	}
	return desc
}

func localizedStatus(printer *message.Printer, info GeneralJobInfo) string {
	if info.GetError() == nil {
		return printer.Sprintf("Job finished without errors")
	}
	return printer.Sprintf("Job finished with error: %s", info.GetError())
}
