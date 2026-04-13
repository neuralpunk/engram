package project

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type engramFile struct {
	Project string `toml:"project"`
}

// Detect walks up the directory tree from startDir looking for a .engram file.
// Returns the project name and true if found, or empty string and false otherwise.
func Detect(startDir string) (string, bool) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", false
	}

	for {
		path := filepath.Join(dir, ".engram")
		data, err := os.ReadFile(path)
		if err == nil {
			var ef engramFile
			if err := toml.Unmarshal(data, &ef); err == nil && ef.Project != "" {
				return ef.Project, true
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}
