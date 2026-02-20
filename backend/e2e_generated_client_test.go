package backend_test

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"

	genclient "github.com/simonjohansson/kanban/backend/gen/client"
	"github.com/simonjohansson/kanban/backend/internal/server"
	"github.com/stretchr/testify/require"
)

func TestE2EGeneratedClientFlow(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	sqlitePath := filepath.Join(dataDir, "projection.db")
	app, err := server.New(server.Options{
		DataDir:    dataDir,
		SQLitePath: sqlitePath,
		Logger:     newTestLogger(t, "backend"),
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = app.Close() })

	httpServer := httptest.NewServer(app.Handler())
	t.Cleanup(httpServer.Close)

	client, err := genclient.NewClientWithResponses(httpServer.URL)
	require.NoError(t, err)
	ctx := context.Background()

	health, err := client.GetHealthWithResponse(ctx)
	require.NoError(t, err)
	require.Equal(t, 200, health.StatusCode())
	require.NotNil(t, health.JSON200)
	require.True(t, health.JSON200.Ok)

	localPath := "/tmp/generated-client-demo"
	remoteURL := "https://example.com/generated-client-demo.git"
	createProject, err := client.CreateProjectWithResponse(ctx, genclient.CreateProjectRequest{
		Name:      "Generated Client Demo",
		LocalPath: &localPath,
		RemoteUrl: &remoteURL,
	})
	require.NoError(t, err)
	require.Equal(t, 201, createProject.StatusCode())
	require.NotNil(t, createProject.JSON201)
	require.Equal(t, "generated-client-demo", createProject.JSON201.Slug)

	listProjects, err := client.ListProjectsWithResponse(ctx)
	require.NoError(t, err)
	require.Equal(t, 200, listProjects.StatusCode())
	require.NotNil(t, listProjects.JSON200)
	require.Len(t, listProjects.JSON200.Projects, 1)
	require.Equal(t, "generated-client-demo", listProjects.JSON200.Projects[0].Slug)

	createCard, err := client.CreateCardWithResponse(ctx, "generated-client-demo", genclient.CreateCardRequest{
		Title:       "Set up generated client e2e test",
		Status:      "Todo",
		Description: ptr("Initial description"),
		Column:      ptr("Todo"),
	})
	require.NoError(t, err)
	require.Equal(t, 201, createCard.StatusCode())
	require.NotNil(t, createCard.JSON201)
	require.Equal(t, "generated-client-demo/card-1", createCard.JSON201.Id)

	getCard, err := client.GetCardWithResponse(ctx, "generated-client-demo", int64(1))
	require.NoError(t, err)
	require.Equal(t, 200, getCard.StatusCode())
	require.NotNil(t, getCard.JSON200)
	require.Equal(t, "Todo", getCard.JSON200.Status)
	require.Len(t, getCard.JSON200.Description, 1)
	require.Len(t, getCard.JSON200.Comments, 0)

	addComment, err := client.CommentCardWithResponse(ctx, "generated-client-demo", int64(1), genclient.TextBodyRequest{
		Body: "This is a generated client comment",
	})
	require.NoError(t, err)
	require.Equal(t, 200, addComment.StatusCode())
	require.NotNil(t, addComment.JSON200)
	require.Len(t, addComment.JSON200.Comments, 1)

	appendDescription, err := client.AppendDescriptionWithResponse(ctx, "generated-client-demo", int64(1), genclient.TextBodyRequest{
		Body: "Append description via generated client",
	})
	require.NoError(t, err)
	require.Equal(t, 200, appendDescription.StatusCode())
	require.NotNil(t, appendDescription.JSON200)
	require.Len(t, appendDescription.JSON200.Description, 2)

	moveCard, err := client.MoveCardWithResponse(ctx, "generated-client-demo", int64(1), genclient.MoveCardRequest{
		Status: "Doing",
		Column: ptr("Doing"),
	})
	require.NoError(t, err)
	require.Equal(t, 200, moveCard.StatusCode())
	require.NotNil(t, moveCard.JSON200)
	require.Equal(t, "Doing", moveCard.JSON200.Status)

	listActiveCards, err := client.ListCardsWithResponse(ctx, "generated-client-demo", nil)
	require.NoError(t, err)
	require.Equal(t, 200, listActiveCards.StatusCode())
	require.NotNil(t, listActiveCards.JSON200)
	require.Len(t, listActiveCards.JSON200.Cards, 1)

	softDelete, err := client.DeleteCardWithResponse(ctx, "generated-client-demo", int64(1), nil)
	require.NoError(t, err)
	require.Equal(t, 200, softDelete.StatusCode())
	require.NotNil(t, softDelete.JSON200)
	require.True(t, softDelete.JSON200.Deleted)

	listWithoutDeleted, err := client.ListCardsWithResponse(ctx, "generated-client-demo", nil)
	require.NoError(t, err)
	require.Equal(t, 200, listWithoutDeleted.StatusCode())
	require.NotNil(t, listWithoutDeleted.JSON200)
	require.Len(t, listWithoutDeleted.JSON200.Cards, 0)

	includeDeleted := true
	listWithDeleted, err := client.ListCardsWithResponse(ctx, "generated-client-demo", &genclient.ListCardsParams{
		IncludeDeleted: &includeDeleted,
	})
	require.NoError(t, err)
	require.Equal(t, 200, listWithDeleted.StatusCode())
	require.NotNil(t, listWithDeleted.JSON200)
	require.Len(t, listWithDeleted.JSON200.Cards, 1)

	hardDelete := true
	hardDeletedResp, err := client.DeleteCardWithResponse(ctx, "generated-client-demo", int64(1), &genclient.DeleteCardParams{
		Hard: &hardDelete,
	})
	require.NoError(t, err)
	require.Equal(t, 200, hardDeletedResp.StatusCode())
	require.NotNil(t, hardDeletedResp.JSON200)

	listAfterHardDelete, err := client.ListCardsWithResponse(ctx, "generated-client-demo", &genclient.ListCardsParams{
		IncludeDeleted: &includeDeleted,
	})
	require.NoError(t, err)
	require.Equal(t, 200, listAfterHardDelete.StatusCode())
	require.NotNil(t, listAfterHardDelete.JSON200)
	require.Len(t, listAfterHardDelete.JSON200.Cards, 0)

	rebuild, err := client.RebuildProjectionWithResponse(ctx)
	require.NoError(t, err)
	require.Equal(t, 200, rebuild.StatusCode())
	require.NotNil(t, rebuild.JSON200)
	require.Equal(t, int64(1), rebuild.JSON200.ProjectsRebuilt)
}

func ptr[T any](v T) *T {
	return &v
}
