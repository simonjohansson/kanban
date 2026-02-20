package server

import (
	"context"
	"fmt"

	"github.com/danielgtaylor/huma/v2"
	"github.com/simonjohansson/kanban/backend/internal/model"
)

type createCardRequest struct {
	Title       string  `json:"title"`
	Description *string `json:"description,omitempty"`
	Branch      *string `json:"branch,omitempty"`
	Status      string  `json:"status"`
}

type createCardInput struct {
	Project string `path:"project"`
	Body    createCardRequest
}

type createCardOutput struct {
	Body model.Card
}

func (s *Server) createCard(_ context.Context, input *createCardInput) (*createCardOutput, error) {
	card, err := s.service.CreateCard(input.Project, input.Body.Title, stringOrEmpty(input.Body.Description), stringOrEmpty(input.Body.Branch), input.Body.Status)
	if err != nil {
		return nil, toHumaError(err)
	}

	out := &createCardOutput{Body: card}
	return out, nil
}

type listCardsInput struct {
	Project        string `path:"project"`
	IncludeDeleted bool   `query:"include_deleted"`
}

type listCardsOutput struct {
	Body struct {
		Cards []model.CardSummary `json:"cards"`
	}
}

func (s *Server) listCards(_ context.Context, input *listCardsInput) (*listCardsOutput, error) {
	cards, err := s.service.ListCards(input.Project, input.IncludeDeleted)
	if err != nil {
		return nil, toHumaError(err)
	}
	out := &listCardsOutput{}
	out.Body.Cards = cards
	return out, nil
}

type cardPathInput struct {
	Project string `path:"project"`
	Number  int    `path:"number"`
}

type getCardOutput struct {
	Body model.Card
}

func (s *Server) getCard(_ context.Context, input *cardPathInput) (*getCardOutput, error) {
	number, err := normalizeCardNumber(input.Number)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	card, err := s.service.GetCard(input.Project, number)
	if err != nil {
		return nil, toHumaError(err)
	}
	return &getCardOutput{Body: card}, nil
}

type moveCardRequest struct {
	Status string `json:"status"`
}

type moveCardInput struct {
	Project string `path:"project"`
	Number  int    `path:"number"`
	Body    moveCardRequest
}

type moveCardOutput struct {
	Body model.Card
}

func (s *Server) moveCard(_ context.Context, input *moveCardInput) (*moveCardOutput, error) {
	number, err := normalizeCardNumber(input.Number)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	card, err := s.service.MoveCard(input.Project, number, input.Body.Status)
	if err != nil {
		return nil, toHumaError(err)
	}
	return &moveCardOutput{Body: card}, nil
}

type textBodyRequest struct {
	Body string `json:"body"`
}

type commentCardInput struct {
	Project string `path:"project"`
	Number  int    `path:"number"`
	Body    textBodyRequest
}

type commentCardOutput struct {
	Body model.Card
}

func (s *Server) commentCard(_ context.Context, input *commentCardInput) (*commentCardOutput, error) {
	number, err := normalizeCardNumber(input.Number)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	card, err := s.service.CommentCard(input.Project, number, input.Body.Body)
	if err != nil {
		return nil, toHumaError(err)
	}
	return &commentCardOutput{Body: card}, nil
}

type appendDescriptionInput struct {
	Project string `path:"project"`
	Number  int    `path:"number"`
	Body    textBodyRequest
}

type appendDescriptionOutput struct {
	Body model.Card
}

func (s *Server) appendDescription(_ context.Context, input *appendDescriptionInput) (*appendDescriptionOutput, error) {
	number, err := normalizeCardNumber(input.Number)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	card, err := s.service.AppendDescription(input.Project, number, input.Body.Body)
	if err != nil {
		return nil, toHumaError(err)
	}
	return &appendDescriptionOutput{Body: card}, nil
}

type setCardBranchRequest struct {
	Branch string `json:"branch"`
}

type setCardBranchInput struct {
	Project string `path:"project"`
	Number  int    `path:"number"`
	Body    setCardBranchRequest
}

type setCardBranchOutput struct {
	Body model.Card
}

func (s *Server) setCardBranch(_ context.Context, input *setCardBranchInput) (*setCardBranchOutput, error) {
	number, err := normalizeCardNumber(input.Number)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	card, err := s.service.SetCardBranch(input.Project, number, input.Body.Branch)
	if err != nil {
		return nil, toHumaError(err)
	}
	return &setCardBranchOutput{Body: card}, nil
}

type deleteCardInput struct {
	Project string `path:"project"`
	Number  int    `path:"number"`
	Hard    bool   `query:"hard"`
}

type deleteCardOutput struct {
	Body model.Card
}

func (s *Server) deleteCard(_ context.Context, input *deleteCardInput) (*deleteCardOutput, error) {
	number, err := normalizeCardNumber(input.Number)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	card, err := s.service.DeleteCard(input.Project, number, input.Hard)
	if err != nil {
		return nil, toHumaError(err)
	}

	return &deleteCardOutput{Body: card}, nil
}

func normalizeCardNumber(number int) (int, error) {
	if number <= 0 {
		return 0, fmt.Errorf("invalid card number")
	}
	return number, nil
}
