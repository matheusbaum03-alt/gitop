package snapshots

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type Entry struct {
	Type string `json:"type"`
	Path string `json:"path"`
}

func getCachePath(name string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("error while getting home dir returning $HOME with path")
		return filepath.Join(os.Getenv("HOME"), ".local", "share", "gitop", name)
	}
	return filepath.Join(home, ".local", "share", "gitop", name)
}

func logF(msg string) {
	fmt.Println(msg)
	err := os.Remove(getCachePath("lock.lock"))
	if err != nil {
		log.Fatal("Error removing check if is there")
	}
	os.Exit(1)
}

func GetMapDir(dir string) map[string]string {
	info, err := os.Stat(dir)
	if err != nil {
		logF("Error getting stat: " + err.Error())
	}
	if !info.IsDir() {
		logF("Error Tried to map a non dir path")
	}
	paths := make(map[string]string)
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == dir {
			return nil
		}
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if strings.Contains(relPath, "share/") {
			return nil
		}
		if d.IsDir() {
			paths[relPath] = "dir"
		} else {
			paths[relPath] = "file"
		}
		return nil
	})
	if err != nil {
		logF("Error walking went wrong: " + err.Error())
	}
	return paths
}

func GetDelta(oldV, newV map[string]string) []Entry {
	var entries []Entry
	for relPath, entryType := range newV {
		if _, existed := oldV[relPath]; !existed {
			entries = append(entries, Entry{
				Path: relPath,
				Type: entryType,
			})
		}
	}
	return entries
}
