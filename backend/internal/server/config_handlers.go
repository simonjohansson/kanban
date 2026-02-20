package server

import (
	"context"
	"os"
	"strings"

	"github.com/simonjohansson/kanban/backend/pkg/kanbanconfig"
)

type clientConfigOutput struct {
	Body struct {
		ServerURL string `json:"server_url"`
	}
}

func (s *Server) clientConfig(_ context.Context, _ *struct{}) (*clientConfigOutput, error) {
	out := &clientConfigOutput{}
	home, err := os.UserHomeDir()
	if err != nil {
		return out, nil
	}
	cfgPath := kanbanconfig.ConfigPath(home)
	cfg, err := kanbanconfig.LoadFile(cfgPath)
	if err != nil {
		return out, nil
	}
	out.Body.ServerURL = strings.TrimSpace(cfg.ServerURL)
	return out, nil
}
