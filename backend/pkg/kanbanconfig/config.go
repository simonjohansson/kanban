package kanbanconfig

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultServerURL = "http://127.0.0.1:8080"
	DefaultOutput    = "text"
)

type Config struct {
	ServerURL string        `yaml:"server_url"`
	Backend   BackendConfig `yaml:"backend"`
	CLI       CLIConfig     `yaml:"cli"`
}

type BackendConfig struct {
	SQLitePath string `yaml:"sqlite_path"`
	CardsPath  string `yaml:"cards_path"`
}

type CLIConfig struct {
	Output string `yaml:"output"`
}

func Default(home string) Config {
	stateDir := filepath.Join(home, ".local", "state", "kanban")
	cardsPath := filepath.Join(stateDir, "cards")

	return Config{
		ServerURL: DefaultServerURL,
		Backend: BackendConfig{
			SQLitePath: filepath.Join(stateDir, "projection.db"),
			CardsPath:  cardsPath,
		},
		CLI: CLIConfig{
			Output: DefaultOutput,
		},
	}
}

func ConfigPath(home string) string {
	return filepath.Join(home, ".config", "kanban", "config.yaml")
}

func LoadOrInit(home string) (Config, error) {
	path := ConfigPath(home)
	defaults := Default(home)

	cfg, err := LoadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := SaveFile(path, defaults); err != nil {
				return Config{}, err
			}
			return defaults, nil
		}
		return Config{}, err
	}

	merged := Merge(defaults, cfg)
	if merged != cfg {
		if err := SaveFile(path, merged); err != nil {
			return Config{}, err
		}
	}

	return merged, nil
}

func LoadFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}

	return normalize(cfg), nil
}

func SaveFile(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := yaml.Marshal(normalize(cfg))
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

func Merge(defaults Config, user Config) Config {
	out := normalize(defaults)
	in := normalize(user)

	if in.ServerURL != "" {
		out.ServerURL = in.ServerURL
	}

	if in.Backend.SQLitePath != "" {
		out.Backend.SQLitePath = in.Backend.SQLitePath
	}
	if in.Backend.CardsPath != "" {
		out.Backend.CardsPath = in.Backend.CardsPath
	}

	if in.CLI.Output != "" {
		out.CLI.Output = in.CLI.Output
	}

	return out
}

func normalize(cfg Config) Config {
	cfg.ServerURL = strings.TrimSpace(cfg.ServerURL)
	cfg.Backend.SQLitePath = strings.TrimSpace(cfg.Backend.SQLitePath)
	cfg.Backend.CardsPath = strings.TrimSpace(cfg.Backend.CardsPath)
	cfg.CLI.Output = strings.TrimSpace(cfg.CLI.Output)
	return cfg
}
