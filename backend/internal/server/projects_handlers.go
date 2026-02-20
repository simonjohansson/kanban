package server

import (
	"context"

	"github.com/simonjohansson/kanban/backend/internal/model"
)

type createProjectRequest struct {
	Name      string  `json:"name"`
	LocalPath *string `json:"local_path,omitempty"`
	RemoteURL *string `json:"remote_url,omitempty"`
}

type createProjectInput struct {
	Body createProjectRequest
}

type createProjectOutput struct {
	Body model.Project
}

func (s *Server) createProject(_ context.Context, input *createProjectInput) (*createProjectOutput, error) {
	project, err := s.service.CreateProject(input.Body.Name, stringOrEmpty(input.Body.LocalPath), stringOrEmpty(input.Body.RemoteURL))
	if err != nil {
		return nil, toHumaError(err)
	}

	out := &createProjectOutput{Body: project}
	return out, nil
}

type listProjectsOutput struct {
	Body struct {
		Projects []model.Project `json:"projects"`
	}
}

func (s *Server) listProjects(_ context.Context, _ *struct{}) (*listProjectsOutput, error) {
	projects, err := s.service.ListProjects()
	if err != nil {
		return nil, toHumaError(err)
	}
	out := &listProjectsOutput{}
	out.Body.Projects = projects
	return out, nil
}

type deleteProjectInput struct {
	Project string `path:"project"`
}

type deleteProjectOutput struct {
	Body struct {
		Project string `json:"project"`
		Deleted bool   `json:"deleted"`
	}
}

func (s *Server) deleteProject(_ context.Context, input *deleteProjectInput) (*deleteProjectOutput, error) {
	if err := s.service.DeleteProject(input.Project); err != nil {
		return nil, toHumaError(err)
	}

	out := &deleteProjectOutput{}
	out.Body.Project = input.Project
	out.Body.Deleted = true
	return out, nil
}
