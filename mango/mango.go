package mango

// #cgo LDFLAGS:  -lmanatee -L${SRCDIR} -Wl,-rpath='$ORIGIN'
// #include <stdlib.h>
// #include "mango.h"
import "C"

import (
	"fmt"
	"unsafe"
)

// GoCorpus is a Go wrapper for Manatee Corpus instance
type GoCorpus struct {
	corp C.CorpusV
}

func (gc *GoCorpus) Close() {
	C.close_corpus(gc.corp)
}

type GoConc struct {
	conc C.ConcV
}

type GoVector struct {
	v C.MVector
}

type GoFreqs struct {
	words C.MVector
	freqs C.MVector
	norms C.MVector
}

func (gc *GoConc) Size() int64 {
	return int64(C.concordance_size(gc.conc))
}

// OpenCorpus is a factory function creating
// a Manatee corpus wrapper.
func OpenCorpus(path string) (GoCorpus, error) {
	ret := GoCorpus{}
	var err error
	ans := C.open_corpus(C.CString(path))

	if ans.err != nil {
		err = fmt.Errorf(C.GoString(ans.err))
		defer C.free(unsafe.Pointer(ans.err))
		return ret, err
	}
	ret.corp = ans.value
	if ret.corp == nil {
		return ret, fmt.Errorf("Corpus %s not found", path)
	}
	return ret, nil
}

// CloseCorpus closes all the resources accompanying
// the corpus. The instance should become unusable.
func CloseCorpus(corpus GoCorpus) error {
	C.close_corpus(corpus.corp)
	return nil
}

// GetCorpusSize returns corpus size in tokens
func GetCorpusSize(corpus GoCorpus) (int64, error) {
	ans := (C.get_corpus_size(corpus.corp))
	if ans.err != nil {
		err := fmt.Errorf(C.GoString(ans.err))
		defer C.free(unsafe.Pointer(ans.err))
		return -1, err
	}
	return int64(ans.value), nil
}

// GetCorpusConf returns a corpus configuration item
// stored in a corpus configuration file (aka "registry file")
func GetCorpusConf(corpus *GoCorpus, prop string) (string, error) {
	ans := (C.get_corpus_conf(corpus.corp, C.CString(prop)))
	if ans.err != nil {
		err := fmt.Errorf(C.GoString(ans.err))
		defer C.free(unsafe.Pointer(ans.err))
		return "", err
	}
	return C.GoString(ans.value), nil
}

func CreateConcordance(corpus *GoCorpus, query string) (*GoConc, error) {
	var ret GoConc
	ans := (C.create_concordance(corpus.corp, C.CString(query)))
	if ans.err != nil {
		err := fmt.Errorf(C.GoString(ans.err))
		defer C.free(unsafe.Pointer(ans.err))
		return &ret, err
	}
	ret.conc = ans.value
	return &ret, nil
}

func CalcFreqDist(corpus *GoCorpus, conc *GoConc, fcrit string) (*GoFreqs, error) {
	var ret GoFreqs
	C.freq_dist(corpus, conc, C.CString(fcrit))
	return &ret, nil
}

func StrVectorToSlice(vector GoVector) []string {
	size := int(C.str_vector_get_size(vector.v))
	slice := make([]string, size)
	for i := 0; i < size; i++ {
		cstr := C.str_vector_get_element(vector.v, C.int(i))
		slice[i] = C.GoString(cstr)
	}
	return slice
}
