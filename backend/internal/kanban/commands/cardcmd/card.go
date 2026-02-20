package cardcmd

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
		Example: strings.TrimSpace(`kanban card create --project alpha --title "Task" --status Todo
kanban cards new -p alpha -t "Task" -s Doing`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := common.NewClient(runtime)
			if err != nil {
				return wrapErr(http.StatusBadRequest, err.Error())
			}

			project, _ := cmd.Flags().GetString("project")
			title, _ := cmd.Flags().GetString("title")
			status, _ := cmd.Flags().GetString("status")
			description, _ := cmd.Flags().GetString("description")
			branch, _ := cmd.Flags().GetString("branch")

			body := apiclient.CreateCardRequest{Title: strings.TrimSpace(title), Status: strings.TrimSpace(status)}
			if value := strings.TrimSpace(description); value != "" {
				body.Description = &value
			}
			if value := strings.TrimSpace(branch); value != "" {
				body.Branch = &value
			}

			resp, reqErr := client.CreateCard(context.Background(), strings.TrimSpace(project), body)
			return handle(runtime.Output(), stdout, resp, reqErr)
		},
	}
	createCmd.Flags().StringP("project", "p", "", "Project slug")
	createCmd.Flags().StringP("title", "t", "", "Card title")
	createCmd.Flags().StringP("description", "d", "", "Initial description text")
	createCmd.Flags().String("branch", "", "Optional git branch metadata")
	createCmd.Flags().StringP("status", "s", "", "Card status (Todo|Doing|Review|Done)")
	_ = createCmd.MarkFlagRequired("project")
	_ = createCmd.MarkFlagRequired("title")
	_ = createCmd.MarkFlagRequired("status")

	listCmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List cards.",
		Long:    "List cards in a project.",
		Example: strings.TrimSpace(`kanban card list --project alpha
kanban cards ls -p alpha --include-deleted`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := common.NewClient(runtime)
			if err != nil {
				return wrapErr(http.StatusBadRequest, err.Error())
			}

			project, _ := cmd.Flags().GetString("project")
			includeDeleted, _ := cmd.Flags().GetBool("include-deleted")
			params := &apiclient.ListCardsParams{IncludeDeleted: &includeDeleted}
			resp, reqErr := client.ListCards(context.Background(), strings.TrimSpace(project), params)
			return handle(runtime.Output(), stdout, resp, reqErr)
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
		Example: strings.TrimSpace(`kanban card get --project alpha --id 1
kanban cards show -p alpha -i 1 --output json`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := common.NewClient(runtime)
			if err != nil {
				return wrapErr(http.StatusBadRequest, err.Error())
			}

			project, _ := cmd.Flags().GetString("project")
			id, _ := cmd.Flags().GetInt64("id")
			resp, reqErr := client.GetCard(context.Background(), strings.TrimSpace(project), id)
			return handle(runtime.Output(), stdout, resp, reqErr)
		},
	}
	getCmd.Flags().StringP("project", "p", "", "Project slug")
	getCmd.Flags().Int64P("id", "i", 0, "Card number")
	_ = getCmd.MarkFlagRequired("project")
	_ = getCmd.MarkFlagRequired("id")

	moveCmd := &cobra.Command{
		Use:   "move",
		Short: "Move a card.",
		Long:  "Update card status.",
		Example: strings.TrimSpace(`kanban card move --project alpha --id 1 --status Doing
kanban cards move -p alpha -i 1 -s Review`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := common.NewClient(runtime)
			if err != nil {
				return wrapErr(http.StatusBadRequest, err.Error())
			}

			project, _ := cmd.Flags().GetString("project")
			id, _ := cmd.Flags().GetInt64("id")
			status, _ := cmd.Flags().GetString("status")
			body := apiclient.MoveCardRequest{Status: strings.TrimSpace(status)}
			resp, reqErr := client.MoveCard(context.Background(), strings.TrimSpace(project), id, body)
			return handle(runtime.Output(), stdout, resp, reqErr)
		},
	}
	moveCmd.Flags().StringP("project", "p", "", "Project slug")
	moveCmd.Flags().Int64P("id", "i", 0, "Card number")
	moveCmd.Flags().StringP("status", "s", "", "Target status (Todo|Doing|Review|Done)")
	_ = moveCmd.MarkFlagRequired("project")
	_ = moveCmd.MarkFlagRequired("id")
	_ = moveCmd.MarkFlagRequired("status")

	commentCmd := &cobra.Command{
		Use:     "comment",
		Aliases: []string{"note"},
		Short:   "Append a comment.",
		Long:    "Add a comment event to a card.",
		Example: strings.TrimSpace(`kanban card comment --project alpha --id 1 --body "Need review"
kanban cards note -p alpha -i 1 -b "LGTM"`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := common.NewClient(runtime)
			if err != nil {
				return wrapErr(http.StatusBadRequest, err.Error())
			}

			project, _ := cmd.Flags().GetString("project")
			id, _ := cmd.Flags().GetInt64("id")
			bodyRaw, _ := cmd.Flags().GetString("body")

			resp, reqErr := client.CommentCard(context.Background(), strings.TrimSpace(project), id, apiclient.TextBodyRequest{Body: strings.TrimSpace(bodyRaw)})
			return handle(runtime.Output(), stdout, resp, reqErr)
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
		Example: strings.TrimSpace(`kanban card describe --project alpha --id 1 --body "Investigated root cause"
kanban cards desc -p alpha -i 1 -b "Added acceptance criteria"`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := common.NewClient(runtime)
			if err != nil {
				return wrapErr(http.StatusBadRequest, err.Error())
			}

			project, _ := cmd.Flags().GetString("project")
			id, _ := cmd.Flags().GetInt64("id")
			bodyRaw, _ := cmd.Flags().GetString("body")

			resp, reqErr := client.AppendDescription(context.Background(), strings.TrimSpace(project), id, apiclient.TextBodyRequest{Body: strings.TrimSpace(bodyRaw)})
			return handle(runtime.Output(), stdout, resp, reqErr)
		},
	}
	describeCmd.Flags().StringP("project", "p", "", "Project slug")
	describeCmd.Flags().Int64P("id", "i", 0, "Card number")
	describeCmd.Flags().StringP("body", "b", "", "Description text to append")
	_ = describeCmd.MarkFlagRequired("project")
	_ = describeCmd.MarkFlagRequired("id")
	_ = describeCmd.MarkFlagRequired("body")

	branchCmd := &cobra.Command{
		Use:   "branch",
		Short: "Set card branch metadata.",
		Long:  "Set or update the card branch value.",
		Example: strings.TrimSpace(`kanban card branch --project alpha --id 1 --branch feature/task
kanban cards branch -p alpha -i 1 -b feature/task-v2`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := common.NewClient(runtime)
			if err != nil {
				return wrapErr(http.StatusBadRequest, err.Error())
			}

			project, _ := cmd.Flags().GetString("project")
			id, _ := cmd.Flags().GetInt64("id")
			branch, _ := cmd.Flags().GetString("branch")

			body := apiclient.SetCardBranchRequest{Branch: strings.TrimSpace(branch)}
			resp, reqErr := client.SetCardBranch(context.Background(), strings.TrimSpace(project), id, body)
			return handle(runtime.Output(), stdout, resp, reqErr)
		},
	}
	branchCmd.Flags().StringP("project", "p", "", "Project slug")
	branchCmd.Flags().Int64P("id", "i", 0, "Card number")
	branchCmd.Flags().StringP("branch", "b", "", "Git branch metadata")
	_ = branchCmd.MarkFlagRequired("project")
	_ = branchCmd.MarkFlagRequired("id")
	_ = branchCmd.MarkFlagRequired("branch")

	deleteCmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm", "remove"},
		Short:   "Delete a card.",
		Long:    "Soft-delete by default; set --hard for permanent delete.",
		Example: strings.TrimSpace(`kanban card delete --project alpha --id 1
kanban cards rm -p alpha -i 1 --hard`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := common.NewClient(runtime)
			if err != nil {
				return wrapErr(http.StatusBadRequest, err.Error())
			}

			project, _ := cmd.Flags().GetString("project")
			id, _ := cmd.Flags().GetInt64("id")
			hard, _ := cmd.Flags().GetBool("hard")

			params := &apiclient.DeleteCardParams{Hard: &hard}
			resp, reqErr := client.DeleteCard(context.Background(), strings.TrimSpace(project), id, params)
			return handle(runtime.Output(), stdout, resp, reqErr)
		},
	}
	deleteCmd.Flags().StringP("project", "p", "", "Project slug")
	deleteCmd.Flags().Int64P("id", "i", 0, "Card number")
	deleteCmd.Flags().Bool("hard", false, "Permanently delete instead of soft delete")
	_ = deleteCmd.MarkFlagRequired("project")
	_ = deleteCmd.MarkFlagRequired("id")

	cardCmd.AddCommand(createCmd, listCmd, getCmd, moveCmd, commentCmd, describeCmd, branchCmd, deleteCmd)
	return cardCmd
}
