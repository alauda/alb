// Copy from https://github.com/golang/mod/blob/master/sumdb/dirhash/hash.go
package dirhash

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DefaultHash is the default hash function used in new go.sum entries.
var DefaultHash Hash = Hash1

// A Hash is a directory hash function.
// It accepts a list of files along with a function that opens the content of each file.
// It opens, reads, hashes, and closes each file and returns the overall directory hash.
type Hash func(files []string, open func(string) (io.ReadCloser, error)) (string, error)

// Hash1 is the "h1:" directory hash function, using SHA-256.
//
// Hash1 is "h1:" followed by the base64-encoded SHA-256 hash of a summary
// prepared as if by the Unix command:
//
//	find . -type f | sort | sha256sum
//
// More precisely, the hashed summary contains a single line for each file in the list,
// ordered by sort.Strings applied to the file names, where each line consists of
// the hexadecimal SHA-256 hash of the file content,
// two spaces (U+0020), the file name, and a newline (U+000A).
//
// File names with newlines (U+000A) are disallowed.
func Hash1(files []string, open func(string) (io.ReadCloser, error)) (string, error) {
	h := sha256.New()
	files = append([]string(nil), files...)
	sort.Strings(files)
	for _, file := range files {
		if strings.Contains(file, "\n") {
			return "", errors.New("dirhash: filenames with newlines are not supported")
		}
		r, err := open(file)
		if err != nil {
			return "", err
		}
		hf := sha256.New()
		_, err = io.Copy(hf, r)
		r.Close()
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%x  %s\n", hf.Sum(nil), file)
	}
	return "h1:" + base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

// HashDir returns the hash of the local file system directory dir,
// replacing the directory name itself with prefix in the file names
// used in the hash function.
func HashDir(dir, suffix string, hash Hash) (string, error) {
	files, err := DirFiles(dir, suffix)
	if err != nil {
		return "", err
	}
	osOpen := func(name string) (io.ReadCloser, error) {
		return os.Open(filepath.Join(dir, name))
	}
	return hash(files, osOpen)
}

// DirFiles returns the list of files in the tree rooted at dir,
// replacing the directory name dir with prefix in each name.
// The resulting names always use forward slashes.
func DirFiles(dir, suffix string) ([]string, error) {
	var files []string
	dir = filepath.Clean(dir)
	err := filepath.Walk(dir, func(file string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel := file
		if dir != "." {
			rel = file[len(dir)+1:]
		}
		if strings.HasSuffix(rel, suffix) {
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}
