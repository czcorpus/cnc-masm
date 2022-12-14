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

#ifdef __cplusplus
extern "C" {
#endif

typedef void* PosAttrV;
typedef void* CorpusV;
typedef void* StructV;

/**
 * CorpusRetval wraps both
 * a returned Manatee corpus object
 * and possible error
 */
typedef struct CorpusRetval {
    CorpusV value;
    const char * err;
} CorpusRetval;


typedef struct CorpusSizeRetrval {
    long long int value;
    const char * err;
} CorpusSizeRetrval;


typedef struct CorpusStringRetval {
    const char * value;
    const char * err;
} CorpusStringRetval;

/**
 * Create a Manatee corpus instance
 */
CorpusRetval open_corpus(const char* corpusPath);

void close_corpus(CorpusV corpus);

CorpusSizeRetrval get_corpus_size(CorpusV corpus);

CorpusStringRetval get_corpus_conf(CorpusV corpus, const char* prop);

#ifdef __cplusplus
}
#endif