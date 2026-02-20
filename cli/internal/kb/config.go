package kb

import (
	"strings"

	"github.com/simonjohansson/kanban/backend/pkg/kanbanconfig"
)

type Config struct {
	ServerURL string `yaml:"server_url"`
	Output    Output `yaml:"output"`
}

func DefaultConfig(home string) Config {
	shared := kanbanconfig.Default(home)
	return Config{
		ServerURL: shared.ServerURL,
		Output:    Output(shared.CLI.Output),
	}
}

func ParseEnvConfig(env []string) Config {
	cfg := Config{}

	for _, kv := range env {
		switch {
		case strings.HasPrefix(kv, "KB_SERVER_URL="):
			cfg.ServerURL = strings.TrimSpace(strings.TrimPrefix(kv, "KB_SERVER_URL="))
		case strings.HasPrefix(kv, "KB_OUTPUT="):
			value := strings.TrimSpace(strings.TrimPrefix(kv, "KB_OUTPUT="))
			if isValidOutput(value) {
				cfg.Output = Output(value)
			}
		}
	}

	return cfg
}

func MergeConfig(defaults, fileCfg, envCfg, flagCfg Config) Config {
	out := defaults
	applyConfig(&out, fileCfg)
	applyConfig(&out, envCfg)
	applyConfig(&out, flagCfg)
	return out
}

func applyConfig(dst *Config, src Config) {
	if value := strings.TrimSpace(src.ServerURL); value != "" {
		dst.ServerURL = value
	}
	if src.Output != "" {
		dst.Output = src.Output
	}
}

func LoadOrInitConfig(home string) (Config, error) {
	shared, err := kanbanconfig.LoadOrInit(home)
	if err != nil {
		return Config{}, err
	}
	return mapSharedToCLI(shared), nil
}

func ConfigPath(home string) string {
	return kanbanconfig.ConfigPath(home)
}

func LoadConfigFile(path string) (Config, error) {
	shared, err := kanbanconfig.LoadFile(path)
	if err != nil {
		return Config{}, err
	}
	return mapSharedToCLI(shared), nil
}

func SaveConfigFile(path string, cfg Config) error {
	shared, err := kanbanconfig.LoadFile(path)
	if err != nil {
		shared = kanbanconfig.Config{}
	}
	shared.ServerURL = strings.TrimSpace(cfg.ServerURL)
	shared.CLI.Output = strings.TrimSpace(string(cfg.Output))
	return kanbanconfig.SaveFile(path, shared)
}

func mapSharedToCLI(shared kanbanconfig.Config) Config {
	cfg := Config{
		ServerURL: strings.TrimSpace(shared.ServerURL),
		Output:    Output(strings.TrimSpace(shared.CLI.Output)),
	}
	if cfg.Output != "" && !isValidOutput(string(cfg.Output)) {
		cfg.Output = ""
	}
	return cfg
}
