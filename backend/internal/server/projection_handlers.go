package server

import "context"

type healthOutput struct {
	Body struct {
		Ok bool `json:"ok"`
	}
}

func (s *Server) health(_ context.Context, _ *struct{}) (*healthOutput, error) {
	out := &healthOutput{}
	out.Body.Ok = true
	return out, nil
}

type rebuildProjectionOutput struct {
	Body struct {
		ProjectsRebuilt int `json:"projects_rebuilt"`
		CardsRebuilt    int `json:"cards_rebuilt"`
	}
}

func (s *Server) rebuildProjection(_ context.Context, _ *struct{}) (*rebuildProjectionOutput, error) {
	result, err := s.service.RebuildProjection()
	if err != nil {
		return nil, toHumaError(err)
	}

	out := &rebuildProjectionOutput{}
	out.Body.ProjectsRebuilt = result.ProjectsRebuilt
	out.Body.CardsRebuilt = result.CardsRebuilt
	return out, nil
}
