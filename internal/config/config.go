package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Database  DatabaseConfig  `toml:"database"`
	Injection InjectionConfig `toml:"injection"`
	Log       LogConfig       `toml:"log"`
}

type DatabaseConfig struct {
	Path string `toml:"path"`
}

type InjectionConfig struct {
	MaxCorrections int     `toml:"max_corrections"`
	MaxTokens      int     `toml:"max_tokens"`
	MinScore       float64 `toml:"min_score"`
}

type LogConfig struct {
	Level string `toml:"level"`
	File  string `toml:"file"`
}

func DefaultConfig() Config {
	return Config{
		Database: DatabaseConfig{
			Path: filepath.Join("~", ".local", "share", "engram", "engram.db"),
		},
		Injection: InjectionConfig{
			MaxCorrections: 10,
			MaxTokens:      300,
			MinScore:       0.0,
		},
		Log: LogConfig{
			Level: "warn",
			File:  "",
		},
	}
}

// ConfigDir returns the engram configuration directory.
func ConfigDir() string {
	if v := os.Getenv("ENGRAM_CONFIG_DIR"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "engram")
}

// ConfigPath returns the path to the config file.
func ConfigPath() string {
	if v := os.Getenv("ENGRAM_CONFIG"); v != "" {
		return v
	}
	return filepath.Join(ConfigDir(), "config.toml")
}

// Load reads the config file from disk. If the file does not exist,
// it returns the default config without error.
func Load() (Config, error) {
	cfg := DefaultConfig()

	path := ConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}

	cfg.Database.Path = expandHome(cfg.Database.Path)
	cfg.Log.File = expandHome(cfg.Log.File)

	return cfg, nil
}

// expandHome replaces a leading ~ with the user's home directory.
func expandHome(path string) string {
	if path == "" {
		return path
	}
	if path[0] != '~' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[1:])
}

// ResolveDatabasePath returns the absolute database path with ~ expanded.
func (c *Config) ResolveDatabasePath() string {
	return expandHome(c.Database.Path)
}
