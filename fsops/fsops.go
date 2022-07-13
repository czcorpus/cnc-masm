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

	"github.com/rs/zerolog/log"
)

// GetFileMtime returns file modification time as a ISO datetime.
// In case of an error the function returns an empty string and
// logs the error.
func GetFileMtime(filePath string) string {
	f, err := os.Open(filePath)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get file mtime")
		return ""
	}
	defer f.Close()
	finfo, err := f.Stat()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get file mtime")
		return ""
	}
	return finfo.ModTime().Format("2006-01-02T15:04:05-0700")
}

// IsFile tests whether the provided path represents a regular file.
// In case of an error the function returns false and logs the error.
func IsFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	finfo, err := f.Stat()
	if err != nil {
		return false
	}
	return finfo.Mode().IsRegular()
}

// PathExists tests whether  the provided path exists no matter what it
// is (file, dir, ...)
func PathExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// IsDir tests whether the provided path represents a directory.
// In case of an error the function returns false and logs the error.
func IsDir(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	finfo, err := f.Stat()
	if err != nil {
		return false
	}
	return finfo.Mode().IsDir()
}

// FileSize returns size of a provided file.
// In case of an error the function returns -1 and logs the error.
func FileSize(path string) int64 {
	f, err := os.Open(path)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get file size")
		return -1
	}
	defer f.Close()
	finfo, err := f.Stat()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get file size")
		return -1
	}
	return finfo.Size()
}

// ----

// FileList is an abstraction for list of files along with their
// modification time information. It supports sorting.
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

// First returns an item with the latest modification time.
func (f *FileList) First() os.FileInfo {
	return f.files[0]
}

func (f *FileList) ForEach(fn func(info os.FileInfo, idx int) bool) {
	for i, v := range f.files {
		if !fn(v, i) {
			break
		}
	}
}

// ListFilesInDir lists files according to their modification time
// (newest first).
func ListFilesInDir(path string, newestFirst bool) (FileList, error) {
	var ans FileList
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return ans, err
	}
	copy(files, ans.files)
	if newestFirst {
		sort.Sort(&ans)
	}
	return ans, nil
}

// ListDirsInDir lists directories in a directory (without recursion).
func ListDirsInDir(path string, newestFirst bool) (FileList, error) {
	var ans FileList
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return ans, err
	}
	ans.files = make([]os.FileInfo, 0, 200)
	for _, v := range files {
		if v.IsDir() {
			ans.files = append(ans.files, v)
		}
	}
	return ans, nil
}
