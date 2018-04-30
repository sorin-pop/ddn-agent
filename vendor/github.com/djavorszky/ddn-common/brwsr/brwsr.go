package brwsr

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var root string

const (
	kb = 1 << 10
	mb = 1 << 20
	gb = 1 << 30
	tb = 1 << 40
)

// FileList contains the list of files in a folder and bool to decide whether it's on root or not.
type FileList struct {
	OnRoot  bool
	Path    string
	Parent  string
	Entries []Entry
}

// Entry corresponds to an item on the path - can be either a file or a folder
type Entry struct {
	Name    string
	Path    string
	ModTime time.Time
	Size    int64
	Folder  bool
}

// Mount checks if the path is "mountable", meaning, it's accessible
// and writeable.
func Mount(path string) error {
	if path == "" {
		return fmt.Errorf("path is empty")
	}

	if path == ".." {
		return fmt.Errorf("tried mounting parent directory")
	}

	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("path does not exist")
		}

		if os.IsPermission(err) {
			return fmt.Errorf("permission issue: %s", err.Error())
		}

		return fmt.Errorf("unknown issue: %s", err.Error())
	}

	root = path

	return nil
}

// List returns the entries in a given path relative to the root path
func List(relPath string) (FileList, error) {
	var flist FileList

	if strings.HasPrefix(relPath, "..") {
		return flist, fmt.Errorf("relpath starting with '..'")
	}

	list, err := ioutil.ReadDir(fullPath(relPath))
	if err != nil {
		return flist, fmt.Errorf("failed reading dir: %s", err.Error())
	}

	var res []Entry
	for _, item := range list {
		if strings.HasPrefix(item.Name(), ".") {
			continue
		}

		entry := Entry{
			Name:    item.Name(),
			Path:    filepath.Join(relPath, item.Name()),
			Size:    item.Size(),
			ModTime: item.ModTime(),
			Folder:  item.IsDir(),
		}

		res = append(res, entry)
	}

	flist.Path = relPath
	flist.Entries = res

	switch relPath {
	case "", ".", "/":
		flist.OnRoot = true
	default:
		flist.OnRoot = false

		if strings.Contains(relPath, "/") {
			flist.Parent = relPath[0:strings.LastIndex(relPath, "/")]
		}
	}

	return flist, nil
}

//FriendlySize returns the size of the file in a friendly way
func (e Entry) FriendlySize() string {
	if e.Size < kb {
		return fmt.Sprintf("%d B", e.Size)
	}

	if e.Size < mb {
		return fmt.Sprintf("%.2f Kb", float64(e.Size)/kb)
	}

	if e.Size < gb {
		return fmt.Sprintf("%.2f Mb", float64(e.Size)/mb)
	}

	if e.Size < tb {
		return fmt.Sprintf("%.2f Gb", float64(e.Size)/gb)
	}

	return fmt.Sprintf("%.2f Tb", float64(e.Size)/tb)
}

// Importable returns whether a file is importable or not.
func (e Entry) Importable() bool {
	ext := filepath.Ext(e.Name)

	switch strings.ToLower(ext) {
	// dump endings
	case ".sql", ".dmp", ".dpdmp", ".bak":
		return true
	// supported archive settings
	case ".zip", ".tar", ".gz", ".bz2":
		return true
	}

	return false
}

func fullPath(relPath string) string {
	return filepath.Join(root, relPath)
}
