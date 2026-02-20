package kb

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

func Run(args []string, stdout, stderr io.Writer, env []string) int {
	home, err := os.UserHomeDir()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, FormatError(OutputText, http.StatusInternalServerError, err.Error()))
		return 1
	}

	fileCfg, err := LoadOrInitConfig(home)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, FormatError(OutputText, http.StatusInternalServerError, err.Error()))
		return 1
	}

	cfg := MergeConfig(DefaultConfig(home), fileCfg, ParseEnvConfig(env), Config{})
	if !isValidOutput(string(cfg.Output)) {
		cfg.Output = OutputText
	}

	root := NewRootCommand(cfg, stdout, stderr)
	root.SetArgs(args)

	if err := root.Execute(); err != nil {
		output := cfg.Output
		if current, flagErr := root.PersistentFlags().GetString("output"); flagErr == nil && isValidOutput(current) {
			output = Output(current)
		}

		var cErr *cliError
		if ok := asCLIError(err, &cErr); ok {
			if output == OutputJSON && len(cErr.rawJSON) > 0 {
				_, _ = fmt.Fprintln(stderr, string(cErr.rawJSON))
			} else {
				_, _ = fmt.Fprintln(stderr, FormatError(output, cErr.status, cErr.message))
			}
			return 1
		}

		_, _ = fmt.Fprintln(stderr, FormatError(output, http.StatusInternalServerError, err.Error()))
		return 1
	}

	return 0
}
