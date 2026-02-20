package kb

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
	apiclient "github.com/simonjohansson/kanban/backend/gen/client"
	"github.com/spf13/cobra"
)

type globalFlags struct {
	serverURL string
	output    string
}

func NewRootCommand(initial Config, stdout, stderr io.Writer) *cobra.Command {
	cfg := initial
	flags := globalFlags{
		serverURL: initial.ServerURL,
		output:    string(initial.Output),
	}

	root := &cobra.Command{
		Use:   "kb",
		Short: "Manage kanban projects and cards over HTTP.",
		Long: strings.TrimSpace(`kb is a non-interactive CLI for managing kanban projects and cards
against the Kanban backend API.

Use kb help <command> for command-specific examples.

The CLI is intentionally transport-focused:
- --server-url selects the backend endpoint
- --output selects text/json formatting`),
		Example: strings.TrimSpace(`kb --help
kb project create --name "Alpha"
kb proj ls
kb card create -p alpha -t "Task" -s Todo
kb cards rm -p alpha -i 1 --hard
kb watch -p alpha
kb --output json primer`),
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

	root.AddCommand(newPrimerCommand(&cfg, stdout))
	root.AddCommand(newProjectCommand(&cfg, stdout))
	root.AddCommand(newCardCommand(&cfg, stdout))
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

func newPrimerCommand(cfg *Config, stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "primer",
		Short: "Print concise usage guidance.",
		Long:  "Prints quick command examples and usage conventions for scripting.",
		Example: strings.TrimSpace(`kb primer
kb --output json primer`),
		RunE: func(_ *cobra.Command, _ []string) error {
			return printPrimer(cfg.Output, stdout)
		},
	}
}

func newProjectCommand(cfg *Config, stdout io.Writer) *cobra.Command {
	projectCmd := &cobra.Command{
		Use:     "project",
		Aliases: []string{"projects", "proj"},
		Short:   "Manage projects.",
		Long:    "Create, list, and delete projects.",
	}

	createCmd := &cobra.Command{
		Use:     "create",
		Aliases: []string{"new"},
		Short:   "Create a project.",
		Long:    "Create a project with optional repository metadata.",
		Example: strings.TrimSpace(`kb project create --name "Alpha"
kb proj new -n "Alpha" --local-path /work/alpha --remote-url git@github.com:org/alpha.git`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := apiclient.NewClient(cfg.ServerURL)
			if err != nil {
				return &cliError{status: http.StatusBadRequest, message: err.Error()}
			}

			name, _ := cmd.Flags().GetString("name")
			localPath, _ := cmd.Flags().GetString("local-path")
			remoteURL, _ := cmd.Flags().GetString("remote-url")

			body := apiclient.CreateProjectRequest{Name: strings.TrimSpace(name)}
			if value := strings.TrimSpace(localPath); value != "" {
				body.LocalPath = &value
			}
			if value := strings.TrimSpace(remoteURL); value != "" {
				body.RemoteUrl = &value
			}

			resp, reqErr := client.CreateProject(context.Background(), body)
			return handleResponse(cfg.Output, stdout, resp, reqErr)
		},
	}
	createCmd.Flags().StringP("name", "n", "", "Project display name")
	createCmd.Flags().String("local-path", "", "Local repository path")
	createCmd.Flags().String("remote-url", "", "Remote repository URL")
	_ = createCmd.MarkFlagRequired("name")

	listCmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List projects.",
		Long:    "List all projects known by the backend.",
		Example: strings.TrimSpace(`kb project list
kb proj ls`),
		RunE: func(_ *cobra.Command, _ []string) error {
			client, err := apiclient.NewClient(cfg.ServerURL)
			if err != nil {
				return &cliError{status: http.StatusBadRequest, message: err.Error()}
			}

			resp, reqErr := client.ListProjects(context.Background())
			return handleResponse(cfg.Output, stdout, resp, reqErr)
		},
	}

	deleteCmd := &cobra.Command{
		Use:     "delete <project-slug>",
		Aliases: []string{"rm", "remove"},
		Short:   "Delete a project.",
		Long:    "Delete a project by slug.",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(`kb project delete alpha
kb proj rm alpha`),
		RunE: func(_ *cobra.Command, args []string) error {
			client, err := apiclient.NewClient(cfg.ServerURL)
			if err != nil {
				return &cliError{status: http.StatusBadRequest, message: err.Error()}
			}

			resp, reqErr := client.DeleteProject(context.Background(), strings.TrimSpace(args[0]))
			return handleResponse(cfg.Output, stdout, resp, reqErr)
		},
	}

	projectCmd.AddCommand(createCmd, listCmd, deleteCmd)
	return projectCmd
}

func newCardCommand(cfg *Config, stdout io.Writer) *cobra.Command {
	cardCmd := &cobra.Command{
		Use:     "card",
		Aliases: []string{"cards"},
		Short:   "Manage cards.",
		Long:    "Create, list, get, move, comment, describe, and delete cards.",
	}

	createCmd := &cobra.Command{
		Use:     "create",
		Aliases: []string{"new"},
		Short:   "Create a card.",
		Long:    "Create a card in a project with required title and status.",
		Example: strings.TrimSpace(`kb card create --project alpha --title "Task" --status Todo
kb cards new -p alpha -t "Task" -s Doing -c Doing`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := apiclient.NewClient(cfg.ServerURL)
			if err != nil {
				return &cliError{status: http.StatusBadRequest, message: err.Error()}
			}

			project, _ := cmd.Flags().GetString("project")
			title, _ := cmd.Flags().GetString("title")
			status, _ := cmd.Flags().GetString("status")
			description, _ := cmd.Flags().GetString("description")
			column, _ := cmd.Flags().GetString("column")

			body := apiclient.CreateCardRequest{
				Title:  strings.TrimSpace(title),
				Status: strings.TrimSpace(status),
			}
			if value := strings.TrimSpace(description); value != "" {
				body.Description = &value
			}
			if value := strings.TrimSpace(column); value != "" {
				body.Column = &value
			}

			resp, reqErr := client.CreateCard(context.Background(), strings.TrimSpace(project), body)
			return handleResponse(cfg.Output, stdout, resp, reqErr)
		},
	}
	createCmd.Flags().StringP("project", "p", "", "Project slug")
	createCmd.Flags().StringP("title", "t", "", "Card title")
	createCmd.Flags().StringP("description", "d", "", "Initial description text")
	createCmd.Flags().StringP("status", "s", "", "Card status (Todo|Doing|Review|Done)")
	createCmd.Flags().StringP("column", "c", "", "Board column override")
	_ = createCmd.MarkFlagRequired("project")
	_ = createCmd.MarkFlagRequired("title")
	_ = createCmd.MarkFlagRequired("status")

	listCmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List cards.",
		Long:    "List cards in a project.",
		Example: strings.TrimSpace(`kb card list --project alpha
kb cards ls -p alpha --include-deleted`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := apiclient.NewClient(cfg.ServerURL)
			if err != nil {
				return &cliError{status: http.StatusBadRequest, message: err.Error()}
			}

			project, _ := cmd.Flags().GetString("project")
			includeDeleted, _ := cmd.Flags().GetBool("include-deleted")

			params := &apiclient.ListCardsParams{IncludeDeleted: &includeDeleted}
			resp, reqErr := client.ListCards(context.Background(), strings.TrimSpace(project), params)
			return handleResponse(cfg.Output, stdout, resp, reqErr)
		},
	}
	listCmd.Flags().StringP("project", "p", "", "Project slug")
	listCmd.Flags().Bool("include-deleted", false, "Include soft-deleted cards")
	_ = listCmd.MarkFlagRequired("project")

	getCmd := &cobra.Command{
		Use:     "get",
		Aliases: []string{"show"},
		Short:   "Get one card.",
		Long:    "Fetch one card by number from a project.",
		Example: strings.TrimSpace(`kb card get --project alpha --id 1
kb cards show -p alpha -i 1 --output json`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := apiclient.NewClient(cfg.ServerURL)
			if err != nil {
				return &cliError{status: http.StatusBadRequest, message: err.Error()}
			}

			project, _ := cmd.Flags().GetString("project")
			id, _ := cmd.Flags().GetInt64("id")

			resp, reqErr := client.GetCard(context.Background(), strings.TrimSpace(project), id)
			return handleResponse(cfg.Output, stdout, resp, reqErr)
		},
	}
	getCmd.Flags().StringP("project", "p", "", "Project slug")
	getCmd.Flags().Int64P("id", "i", 0, "Card number")
	_ = getCmd.MarkFlagRequired("project")
	_ = getCmd.MarkFlagRequired("id")

	moveCmd := &cobra.Command{
		Use:   "move",
		Short: "Move a card.",
		Long:  "Update card status and optionally column.",
		Example: strings.TrimSpace(`kb card move --project alpha --id 1 --status Doing
kb cards move -p alpha -i 1 -s Review -c Review`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := apiclient.NewClient(cfg.ServerURL)
			if err != nil {
				return &cliError{status: http.StatusBadRequest, message: err.Error()}
			}

			project, _ := cmd.Flags().GetString("project")
			id, _ := cmd.Flags().GetInt64("id")
			status, _ := cmd.Flags().GetString("status")
			column, _ := cmd.Flags().GetString("column")

			body := apiclient.MoveCardRequest{Status: strings.TrimSpace(status)}
			if value := strings.TrimSpace(column); value != "" {
				body.Column = &value
			}

			resp, reqErr := client.MoveCard(context.Background(), strings.TrimSpace(project), id, body)
			return handleResponse(cfg.Output, stdout, resp, reqErr)
		},
	}
	moveCmd.Flags().StringP("project", "p", "", "Project slug")
	moveCmd.Flags().Int64P("id", "i", 0, "Card number")
	moveCmd.Flags().StringP("status", "s", "", "Target status (Todo|Doing|Review|Done)")
	moveCmd.Flags().StringP("column", "c", "", "Target column")
	_ = moveCmd.MarkFlagRequired("project")
	_ = moveCmd.MarkFlagRequired("id")
	_ = moveCmd.MarkFlagRequired("status")

	commentCmd := &cobra.Command{
		Use:     "comment",
		Aliases: []string{"note"},
		Short:   "Append a comment.",
		Long:    "Add a comment event to a card.",
		Example: strings.TrimSpace(`kb card comment --project alpha --id 1 --body "Need review"
kb cards note -p alpha -i 1 -b "LGTM"`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := apiclient.NewClient(cfg.ServerURL)
			if err != nil {
				return &cliError{status: http.StatusBadRequest, message: err.Error()}
			}

			project, _ := cmd.Flags().GetString("project")
			id, _ := cmd.Flags().GetInt64("id")
			bodyRaw, _ := cmd.Flags().GetString("body")

			resp, reqErr := client.CommentCard(context.Background(), strings.TrimSpace(project), id, apiclient.TextBodyRequest{Body: strings.TrimSpace(bodyRaw)})
			return handleResponse(cfg.Output, stdout, resp, reqErr)
		},
	}
	commentCmd.Flags().StringP("project", "p", "", "Project slug")
	commentCmd.Flags().Int64P("id", "i", 0, "Card number")
	commentCmd.Flags().StringP("body", "b", "", "Comment body")
	_ = commentCmd.MarkFlagRequired("project")
	_ = commentCmd.MarkFlagRequired("id")
	_ = commentCmd.MarkFlagRequired("body")

	describeCmd := &cobra.Command{
		Use:     "describe",
		Aliases: []string{"desc"},
		Short:   "Append description text.",
		Long:    "Append text to the card description event log.",
		Example: strings.TrimSpace(`kb card describe --project alpha --id 1 --body "Investigated root cause"
kb cards desc -p alpha -i 1 -b "Added acceptance criteria"`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := apiclient.NewClient(cfg.ServerURL)
			if err != nil {
				return &cliError{status: http.StatusBadRequest, message: err.Error()}
			}

			project, _ := cmd.Flags().GetString("project")
			id, _ := cmd.Flags().GetInt64("id")
			bodyRaw, _ := cmd.Flags().GetString("body")

			resp, reqErr := client.AppendDescription(context.Background(), strings.TrimSpace(project), id, apiclient.TextBodyRequest{Body: strings.TrimSpace(bodyRaw)})
			return handleResponse(cfg.Output, stdout, resp, reqErr)
		},
	}
	describeCmd.Flags().StringP("project", "p", "", "Project slug")
	describeCmd.Flags().Int64P("id", "i", 0, "Card number")
	describeCmd.Flags().StringP("body", "b", "", "Description text to append")
	_ = describeCmd.MarkFlagRequired("project")
	_ = describeCmd.MarkFlagRequired("id")
	_ = describeCmd.MarkFlagRequired("body")

	deleteCmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm", "remove"},
		Short:   "Delete a card.",
		Long:    "Soft-delete by default; set --hard for permanent delete.",
		Example: strings.TrimSpace(`kb card delete --project alpha --id 1
kb cards rm -p alpha -i 1 --hard`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := apiclient.NewClient(cfg.ServerURL)
			if err != nil {
				return &cliError{status: http.StatusBadRequest, message: err.Error()}
			}

			project, _ := cmd.Flags().GetString("project")
			id, _ := cmd.Flags().GetInt64("id")
			hard, _ := cmd.Flags().GetBool("hard")

			params := &apiclient.DeleteCardParams{Hard: &hard}
			resp, reqErr := client.DeleteCard(context.Background(), strings.TrimSpace(project), id, params)
			return handleResponse(cfg.Output, stdout, resp, reqErr)
		},
	}
	deleteCmd.Flags().StringP("project", "p", "", "Project slug")
	deleteCmd.Flags().Int64P("id", "i", 0, "Card number")
	deleteCmd.Flags().Bool("hard", false, "Permanently delete instead of soft delete")
	_ = deleteCmd.MarkFlagRequired("project")
	_ = deleteCmd.MarkFlagRequired("id")

	cardCmd.AddCommand(createCmd, listCmd, getCmd, moveCmd, commentCmd, describeCmd, deleteCmd)
	return cardCmd
}

func newWatchCommand(cfg *Config, stdout io.Writer) *cobra.Command {
	watchCmd := &cobra.Command{
		Use:     "watch",
		Aliases: []string{"events", "stream"},
		Short:   "Stream realtime events over websocket.",
		Long:    "Connect to backend websocket and continuously print events until interrupted.",
		Example: strings.TrimSpace(`kb watch
kb watch --project alpha
kb events -p alpha --output json`),
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
