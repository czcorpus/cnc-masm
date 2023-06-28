package mango

// #cgo CXXFLAGS: -std=c++11
// #cgo CFLAGS: -I${SRCDIR}/attrib -I${SRCDIR}/attrib/corp
// #cgo LDFLAGS:  -lmanatee -L${SRCDIR} -Wl,-rpath='$ORIGIN'
// #include <stdlib.h>
// #include "mango.h"
import "C"

type GoConcordance struct {
	conc C.ConcV
}
