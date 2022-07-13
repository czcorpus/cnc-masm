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

package laconf

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"masm/v3/corpus"
	"masm/v3/fsops"
	"masm/v3/general/collections"
	"masm/v3/liveattrs/utils"
	"os"
	"path"
	"strings"

	"github.com/rs/zerolog/log"

	vteconf "github.com/czcorpus/vert-tagextract/v2/cnf"
	vtedb "github.com/czcorpus/vert-tagextract/v2/db"
)

var (
	ErrorNoSuchConfig = errors.New("no such configuration (corpus not installed)")
)

// Create creates a new live attribute extraction configuration based
// on provided args.
// note: bibIdAttr and mergeAttrs use dot notation (e.g. "doc.author")
func Create(
	masmConf *corpus.Conf,
	corpusInfo *corpus.Info,
	corpusDBInfo *corpus.DBInfo,
	atomStructure string,
	bibIdAttr string,
	mergeAttrs []string,
	mergeFn string,
) (*vteconf.VTEConf, error) {
	newConf := vteconf.VTEConf{
		Corpus:              corpusInfo.ID,
		ParallelCorpus:      corpusDBInfo.ParallelCorpus,
		AtomParentStructure: "",
		StackStructEval:     false,
		MaxNumErrors:        100000, // TODO should not be hardcoded here
		Ngrams:              vteconf.NgramConf{},
		Encoding:            "UTF-8",
		IndexedCols:         []string{},
		VerticalFile:        corpusInfo.RegistryConf.Vertical.Path,
	}
	newConf.Structures = corpusInfo.RegistryConf.SubcorpAttrs
	if bibIdAttr != "" {
		bibView := vtedb.BibViewConf{}
		bibView.IDAttr = utils.ImportKey(bibIdAttr)
		for stru, attrs := range corpusInfo.RegistryConf.SubcorpAttrs {
			for _, attr := range attrs {
				bibView.Cols = append(bibView.Cols, fmt.Sprintf("%s_%s", stru, attr))
			}
		}
		newConf.BibView = bibView
		bibIdElms := strings.Split(bibIdAttr, ".")
		tmp, ok := newConf.Structures[bibIdElms[0]]
		if ok {
			if !collections.SliceContains(tmp, bibIdElms[1]) {
				newConf.Structures[bibIdElms[0]] = append(newConf.Structures[bibIdElms[0]], bibIdElms[1])
			}

		} else {
			newConf.Structures[bibIdElms[0]] = []string{bibIdElms[1]}
		}
	}
	if atomStructure == "" {
		if len(newConf.Structures) == 1 {
			for k := range newConf.Structures {
				newConf.AtomStructure = k
				break
			}
			log.Info().Msgf("no atomStructure, inferred value: %s", newConf.AtomStructure)

		} else {
			return nil, fmt.Errorf("no atomStructure specified and the value cannot be inferred due to multiple involved structures")
		}

	} else {
		newConf.AtomStructure = atomStructure
	}
	atomExists := false
	for _, st := range corpusInfo.IndexedStructs {
		if st == newConf.AtomStructure {
			atomExists = true
			break
		}
	}
	if !atomExists {
		return nil, fmt.Errorf("atom structure '%s' does not exist in corpus %s", newConf.AtomStructure, corpusInfo.ID)
	}

	if len(mergeAttrs) > 0 {
		newConf.SelfJoin.ArgColumns = make([]string, len(mergeAttrs))
		for i, argCol := range mergeAttrs {
			tmp := strings.Split(argCol, ".")
			if len(tmp) != 2 {
				return nil, fmt.Errorf("invalid mergeAttr format: %s", argCol)
			}
			newConf.SelfJoin.ArgColumns[i] = tmp[0] + "_" + tmp[1]
			_, ok := newConf.Structures[tmp[0]]
			if ok {
				if !collections.SliceContains(newConf.Structures[tmp[0]], tmp[1]) {
					newConf.Structures[tmp[0]] = append(newConf.Structures[tmp[0]], tmp[1])
				}

			} else {
				newConf.Structures[tmp[0]] = []string{tmp[1]}
			}
		}
		newConf.SelfJoin.GeneratorFn = mergeFn
	}
	newConf.DB = vtedb.Conf{
		Type:           "mysql",
		Host:           masmConf.LiveAttrs.DB.Host,
		User:           masmConf.LiveAttrs.DB.User,
		Password:       masmConf.LiveAttrs.DB.Password,
		PreconfQueries: masmConf.LiveAttrs.DB.PreconfQueries,
	}
	if corpusDBInfo.ParallelCorpus != "" {
		newConf.DB.Name = corpusDBInfo.ParallelCorpus

	} else {
		newConf.DB.Name = corpusInfo.ID
	}
	return &newConf, nil
}

type LiveAttrsBuildConfProvider struct {
	confDirPath  string
	globalDBConf *vtedb.Conf
	data         map[string]*vteconf.VTEConf
}

// Get returns an existing liveattrs configuration file. In case the file
// does not exist the method will not create it for you (as it requires additional
// arguments to determine specific properties).
// In case there is no other error but the configuration does not exist,
// the method returns ErrorNoSuchConfig error
func (lcache *LiveAttrsBuildConfProvider) Get(corpname string) (*vteconf.VTEConf, error) {
	if v, ok := lcache.data[corpname]; ok {
		return v, nil
	}
	confPath := path.Join(lcache.confDirPath, corpname+".json")
	if fsops.IsFile(confPath) {
		v, err := LoadConf(confPath)
		if err != nil {
			return nil, err
		}
		lcache.data[corpname] = v
		v.DB = *lcache.globalDBConf
		return v, nil
	}
	return nil, ErrorNoSuchConfig
}

// Save saves a provided configuration to a file for later use
func (lcache *LiveAttrsBuildConfProvider) Save(data *vteconf.VTEConf) error {
	rawData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	confPath := path.Join(lcache.confDirPath, data.Corpus+".json")
	err = ioutil.WriteFile(confPath, rawData, 0777)
	if err != nil {
		return err
	}
	return nil
}

// Clear removes a configuration from memory and from filesystem
func (lcache *LiveAttrsBuildConfProvider) Clear(corpusID string) error {
	delete(lcache.data, corpusID)
	confPath := path.Join(lcache.confDirPath, corpusID+".json")
	if fsops.IsFile(confPath) {
		return os.Remove(confPath)
	}
	return nil
}

func NewLiveAttrsBuildConfProvider(confDirPath string, globalDBConf *vtedb.Conf) *LiveAttrsBuildConfProvider {
	return &LiveAttrsBuildConfProvider{
		confDirPath:  confDirPath,
		globalDBConf: globalDBConf,
		data:         make(map[string]*vteconf.VTEConf),
	}
}
