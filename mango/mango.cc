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
#include "concord/concord.hh"
#include "concord/concord.hh"
#include "query/cqpeval.hh"
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
    ans.err = nullptr;
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
    ans.err = nullptr;
    try {
        ans.value = ((Corpus*)corpus)->size();
        return ans;

    } catch (std::exception &e) {
        ans.err = strdup(e.what());
        return ans;
    }
}

CorpusStringRetval get_corpus_conf(CorpusV corpus, const char* prop) {
    CorpusStringRetval ans;
    ans.err = nullptr;
    string tmp(prop);
    try {
        const char * s = ((Corpus*)corpus)->get_conf(tmp).c_str();
        ans.value = s;
        return ans;

    } catch (std::exception &e) {
        ans.err = strdup(e.what());
        return ans;
    }
}


ConcRetval create_concordance(CorpusV corpus, char* query) {
    string q(query);
    ConcRetval ans;
    ans.err = nullptr;
    Corpus* corpusObj = (Corpus*)corpus;

    try {
        ans.value = new Concordance(
            corpusObj, corpusObj->filter_query(eval_cqpquery(q.c_str(), (Corpus*)corpus)));
        ((Concordance*)ans.value)->sync();
    } catch (std::exception &e) {
        ans.err = strdup(e.what());
        return ans;
    }
    return ans;
}

long long int concordance_size(ConcV conc) {
    return ((Concordance *)conc)->size();
}
