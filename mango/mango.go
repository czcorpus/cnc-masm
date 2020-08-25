package mango

// #cgo CXXFLAGS: -std=c++11
// #cgo CFLAGS: -I${SRCDIR}/attrib -I${SRCDIR}/attrib/corp
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
func GetCorpusConf(corpus GoCorpus, prop string) (string, error) {
	ans := (C.get_corpus_conf(corpus.corp, C.CString(prop)))
	if ans.err != nil {
		err := fmt.Errorf(C.GoString(ans.err))
		defer C.free(unsafe.Pointer(ans.err))
		return "", err
	}
	return C.GoString(ans.value), nil
}
