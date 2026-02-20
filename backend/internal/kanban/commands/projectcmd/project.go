package projectcmd

import (
	"context"
	"io"
	"net/http"
	"strings"

	apiclient "github.com/simonjohansson/kanban/backend/gen/client"
	"github.com/simonjohansson/kanban/backend/internal/kanban/commands/common"
	"github.com/spf13/cobra"
)

func New(runtime common.Runtime, stdout io.Writer, handle common.HandleResponseFunc, wrapErr common.WrapErrorFunc) *cobra.Command {
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
		Example: strings.TrimSpace(`kanban project create --name "Alpha"
kanban proj new -n "Alpha" --local-path /work/alpha --remote-url git@github.com:org/alpha.git`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := common.NewClient(runtime)
			if err != nil {
				return wrapErr(http.StatusBadRequest, err.Error())
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
			return handle(runtime.Output(), stdout, resp, reqErr)
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
		Example: strings.TrimSpace(`kanban project list
kanban proj ls`),
		RunE: func(_ *cobra.Command, _ []string) error {
			client, err := common.NewClient(runtime)
			if err != nil {
				return wrapErr(http.StatusBadRequest, err.Error())
			}

			resp, reqErr := client.ListProjects(context.Background())
			return handle(runtime.Output(), stdout, resp, reqErr)
		},
	}

	deleteCmd := &cobra.Command{
		Use:     "delete <project-slug>",
		Aliases: []string{"rm", "remove"},
		Short:   "Delete a project.",
		Long:    "Delete a project by slug.",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(`kanban project delete alpha
kanban proj rm alpha`),
		RunE: func(_ *cobra.Command, args []string) error {
			client, err := common.NewClient(runtime)
			if err != nil {
				return wrapErr(http.StatusBadRequest, err.Error())
			}

			resp, reqErr := client.DeleteProject(context.Background(), strings.TrimSpace(args[0]))
			return handle(runtime.Output(), stdout, resp, reqErr)
		},
	}

	projectCmd.AddCommand(createCmd, listCmd, deleteCmd)
	return projectCmd
}
