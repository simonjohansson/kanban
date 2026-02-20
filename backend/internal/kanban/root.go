package kanban

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/simonjohansson/kanban/backend/internal/kanban/commands/cardcmd"
	"github.com/simonjohansson/kanban/backend/internal/kanban/commands/projectcmd"
	"github.com/spf13/cobra"
)

type globalFlags struct {
	serverURL string
	output    string
}

type commandRuntime struct {
	cfg *Config
}

func (r commandRuntime) ServerURL() string {
	return r.cfg.ServerURL
}

func (r commandRuntime) Output() string {
	return string(r.cfg.Output)
}

func NewRootCommand(initial Config, stdout, stderr io.Writer) *cobra.Command {
	cfg := initial
	flags := globalFlags{
		serverURL: initial.ServerURL,
		output:    string(initial.Output),
	}
	runtime := commandRuntime{cfg: &cfg}

	root := &cobra.Command{
		Use:   "kanban",
		Short: "Run the Kanban server and manage projects/cards over HTTP.",
		Long: strings.TrimSpace(`kanban is a unified binary for:
- starting the Kanban backend server
- managing projects and cards over the Kanban HTTP API

Use kanban help <command> for command-specific examples.

The CLI is intentionally transport-focused:
- --server-url selects the backend endpoint
- --output selects text/json formatting`),
		Example: strings.TrimSpace(`kanban --help
kanban serve
kanban project create --name "Alpha"
kanban proj ls
kanban card create -p alpha -t "Task" -s Todo
kanban cards rm -p alpha -i 1 --hard
kanban watch -p alpha
kanban --output json primer`),
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			return applyGlobalFlags(&cfg, flags)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	root.SetOut(stdout)
	root.SetErr(stderr)

	root.PersistentFlags().StringVar(&flags.serverURL, "server-url", flags.serverURL, "Backend API base URL (e.g. http://127.0.0.1:8080)")
	root.PersistentFlags().StringVar(&flags.output, "output", flags.output, "Output format: text or json")

	root.AddCommand(newServeCommand(&cfg))
	root.AddCommand(newPrimerCommand(&cfg, stdout))
	root.AddCommand(projectcmd.New(runtime, stdout, handleResponseFromString, wrapCLIError))
	root.AddCommand(cardcmd.New(runtime, stdout, handleResponseFromString, wrapCLIError))
	root.AddCommand(newWatchCommand(&cfg, stdout))

	return root
}

func applyGlobalFlags(cfg *Config, flags globalFlags) error {
	output := strings.TrimSpace(flags.output)
	if !isValidOutput(output) {
		return &cliError{status: http.StatusBadRequest, message: fmt.Sprintf("invalid --output: %s", output)}
	}

	cfg.ServerURL = strings.TrimSpace(flags.serverURL)
	cfg.Output = Output(output)

	if cfg.ServerURL == "" {
		return &cliError{status: http.StatusBadRequest, message: "--server-url cannot be empty"}
	}

	return nil
}

func handleResponseFromString(output string, stdout io.Writer, resp *http.Response, reqErr error) error {
	if !isValidOutput(output) {
		return &cliError{status: http.StatusBadRequest, message: fmt.Sprintf("invalid --output: %s", output)}
	}
	return handleResponse(Output(output), stdout, resp, reqErr)
}

func wrapCLIError(status int, message string) error {
	return &cliError{status: status, message: message}
}

func newPrimerCommand(cfg *Config, stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "primer",
		Short: "Print concise usage guidance.",
		Long:  "Prints quick command examples and usage conventions for scripting.",
		Example: strings.TrimSpace(`kanban primer
kanban --output json primer`),
		RunE: func(_ *cobra.Command, _ []string) error {
			return printPrimer(cfg.Output, stdout)
		},
	}
}

func newWatchCommand(cfg *Config, stdout io.Writer) *cobra.Command {
	watchCmd := &cobra.Command{
		Use:     "watch",
		Aliases: []string{"events", "stream"},
		Short:   "Stream realtime events over websocket.",
		Long:    "Connect to backend websocket and continuously print events until interrupted.",
		Example: strings.TrimSpace(`kanban watch
kanban watch --project alpha
kanban events -p alpha --output json`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			project, _ := cmd.Flags().GetString("project")
			wsURL, err := BuildWebsocketURL(cfg.ServerURL, strings.TrimSpace(project))
			if err != nil {
				return &cliError{status: http.StatusBadRequest, message: err.Error()}
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
			if err != nil {
				return &cliError{status: http.StatusBadGateway, message: err.Error()}
			}
			defer conn.Close()

			// Interrupts cancel context, but ReadJSON can still block until socket activity.
			// Close the connection when context is done to unblock reads immediately.
			go func() {
				<-ctx.Done()
				_ = conn.WriteControl(
					websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, "interrupt"),
					time.Now().Add(500*time.Millisecond),
				)
				_ = conn.Close()
			}()

			for {
				select {
				case <-ctx.Done():
					return nil
				default:
				}

				var event map[string]any
				if err := conn.ReadJSON(&event); err != nil {
					if ctx.Err() != nil {
						return nil
					}
					return &cliError{status: http.StatusBadGateway, message: err.Error()}
				}

				line, err := FormatWatchLine(cfg.Output, event)
				if err != nil {
					return &cliError{status: http.StatusInternalServerError, message: err.Error()}
				}
				if _, err := fmt.Fprintln(stdout, line); err != nil {
					return &cliError{status: http.StatusInternalServerError, message: err.Error()}
				}
			}
		},
	}

	watchCmd.Flags().StringP("project", "p", "", "Optional project slug filter")
	return watchCmd
}
