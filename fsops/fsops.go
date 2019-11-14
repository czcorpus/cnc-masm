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

package fsops

import (
	"io/ioutil"
	"os"
	"sort"
)

func GetFileMtime(filePath string) string {
	f, err := os.Open(filePath)
	if err != nil {
		return ""
	}
	finfo, err := f.Stat()
	if err == nil {
		return finfo.ModTime().Format("2006-01-02T15:04:05-0700")
	}
	return ""
}

func IsFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	finfo, err := f.Stat()
	if err != nil {
		return false
	}
	return finfo.Mode().IsRegular()
}

// ----

type FileList struct {
	files []os.FileInfo
}

func (f *FileList) Len() int {
	return len(f.files)
}

func (f *FileList) Less(i, j int) bool {
	return f.files[i].ModTime().After(f.files[j].ModTime())
}

func (f *FileList) Swap(i, j int) {
	f.files[i], f.files[j] = f.files[j], f.files[i]
}

func (f *FileList) First() os.FileInfo {
	return f.files[0]
}

func ListFilesInDir(path string, newestFirst bool) (FileList, error) {
	var ans FileList
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return ans, err
	}
	ans.files = make([]os.FileInfo, len(files))
	for i, v := range files {
		ans.files[i] = v
	}
	if newestFirst {
		sort.Sort(&ans)
	}
	return ans, nil
}
