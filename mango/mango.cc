// Copyright 2019 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2019 Institute of the Czech National Corpus,
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


#include "corp/corpus.hh"
#include "mango.h"
#include <string.h>
#include <stdio.h>
#include <iostream>

using namespace std;

// a bunch of wrapper functions we need to get data
// from Manatee


CorpusRetval open_corpus(const char* corpusPath) {
    string tmp(corpusPath);
    CorpusRetval ans;
    try {
        ans.value = new Corpus(tmp);
        return ans;

    } catch (std::exception &e) {
        ans.err = strdup(e.what());
        return ans;
    }
}

void close_corpus(CorpusV corpus) {
    delete (Corpus *)corpus;
}

CorpusSizeRetrval get_corpus_size(CorpusV corpus) {
    CorpusSizeRetrval ans;
    try {
        ans.value = ((Corpus*)corpus)->size();
        return ans;

    } catch (std::exception &e) {
        ans.err = strdup(e.what());
        return ans;
    }
}
