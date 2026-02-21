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

type addTodoRequest struct {
	Text string `json:"text"`
}

type addTodoInput struct {
	Project string `path:"project"`
	Number  int    `path:"number"`
	Body    addTodoRequest
}

type addTodoOutput struct {
	Body model.Todo
}

func (s *Server) addTodo(_ context.Context, input *addTodoInput) (*addTodoOutput, error) {
	number, err := normalizeCardNumber(input.Number)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	todo, err := s.service.AddTodo(input.Project, number, input.Body.Text)
	if err != nil {
		return nil, toHumaError(err)
	}
	return &addTodoOutput{Body: todo}, nil
}

type listTodosInput struct {
	Project string `path:"project"`
	Number  int    `path:"number"`
}

type listTodosOutput struct {
	Body struct {
		Todos []model.Todo `json:"todos"`
	}
}

func (s *Server) listTodos(_ context.Context, input *listTodosInput) (*listTodosOutput, error) {
	number, err := normalizeCardNumber(input.Number)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	todos, err := s.service.ListTodos(input.Project, number)
	if err != nil {
		return nil, toHumaError(err)
	}
	out := &listTodosOutput{}
	out.Body.Todos = todos
	return out, nil
}

type updateTodoRequest struct {
	Completed bool `json:"completed"`
}

type todoPathInput struct {
	Project string `path:"project"`
	Number  int    `path:"number"`
	TodoID  int    `path:"todo_id"`
}

type updateTodoInput struct {
	Project string `path:"project"`
	Number  int    `path:"number"`
	TodoID  int    `path:"todo_id"`
	Body    updateTodoRequest
}

type updateTodoOutput struct {
	Body model.Todo
}

func (s *Server) updateTodo(_ context.Context, input *updateTodoInput) (*updateTodoOutput, error) {
	number, err := normalizeCardNumber(input.Number)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	todoID, err := normalizeTodoID(input.TodoID)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	todo, err := s.service.SetTodoCompleted(input.Project, number, todoID, input.Body.Completed)
	if err != nil {
		return nil, toHumaError(err)
	}
	return &updateTodoOutput{Body: todo}, nil
}

type deleteTodoOutput struct {
	Body model.Todo
}

func (s *Server) deleteTodo(_ context.Context, input *todoPathInput) (*deleteTodoOutput, error) {
	number, err := normalizeCardNumber(input.Number)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	todoID, err := normalizeTodoID(input.TodoID)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	todo, err := s.service.DeleteTodo(input.Project, number, todoID)
	if err != nil {
		return nil, toHumaError(err)
	}
	return &deleteTodoOutput{Body: todo}, nil
}

func normalizeTodoID(todoID int) (int, error) {
	if todoID <= 0 {
		return 0, fmt.Errorf("invalid todo id")
	}
	return todoID, nil
}

type addAcceptanceCriterionRequest struct {
	Text string `json:"text"`
}

type addAcceptanceCriterionInput struct {
	Project string `path:"project"`
	Number  int    `path:"number"`
	Body    addAcceptanceCriterionRequest
}

type addAcceptanceCriterionOutput struct {
	Body model.AcceptanceCriterion
}

func (s *Server) addAcceptanceCriterion(_ context.Context, input *addAcceptanceCriterionInput) (*addAcceptanceCriterionOutput, error) {
	number, err := normalizeCardNumber(input.Number)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	criterion, err := s.service.AddAcceptanceCriterion(input.Project, number, input.Body.Text)
	if err != nil {
		return nil, toHumaError(err)
	}
	return &addAcceptanceCriterionOutput{Body: criterion}, nil
}

type listAcceptanceCriteriaInput struct {
	Project string `path:"project"`
	Number  int    `path:"number"`
}

type listAcceptanceCriteriaOutput struct {
	Body struct {
		AcceptanceCriteria []model.AcceptanceCriterion `json:"acceptance_criteria"`
	}
}

func (s *Server) listAcceptanceCriteria(_ context.Context, input *listAcceptanceCriteriaInput) (*listAcceptanceCriteriaOutput, error) {
	number, err := normalizeCardNumber(input.Number)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	criteria, err := s.service.ListAcceptanceCriteria(input.Project, number)
	if err != nil {
		return nil, toHumaError(err)
	}
	out := &listAcceptanceCriteriaOutput{}
	out.Body.AcceptanceCriteria = criteria
	return out, nil
}

type criterionPathInput struct {
	Project     string `path:"project"`
	Number      int    `path:"number"`
	CriterionID int    `path:"criterion_id"`
}

type updateAcceptanceCriterionRequest struct {
	Completed bool `json:"completed"`
}

type updateAcceptanceCriterionInput struct {
	Project     string `path:"project"`
	Number      int    `path:"number"`
	CriterionID int    `path:"criterion_id"`
	Body        updateAcceptanceCriterionRequest
}

type updateAcceptanceCriterionOutput struct {
	Body model.AcceptanceCriterion
}

func (s *Server) updateAcceptanceCriterion(_ context.Context, input *updateAcceptanceCriterionInput) (*updateAcceptanceCriterionOutput, error) {
	number, err := normalizeCardNumber(input.Number)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	criterionID, err := normalizeCriterionID(input.CriterionID)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	criterion, err := s.service.SetAcceptanceCriterionCompleted(input.Project, number, criterionID, input.Body.Completed)
	if err != nil {
		return nil, toHumaError(err)
	}
	return &updateAcceptanceCriterionOutput{Body: criterion}, nil
}

type deleteAcceptanceCriterionOutput struct {
	Body model.AcceptanceCriterion
}

func (s *Server) deleteAcceptanceCriterion(_ context.Context, input *criterionPathInput) (*deleteAcceptanceCriterionOutput, error) {
	number, err := normalizeCardNumber(input.Number)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	criterionID, err := normalizeCriterionID(input.CriterionID)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	criterion, err := s.service.DeleteAcceptanceCriterion(input.Project, number, criterionID)
	if err != nil {
		return nil, toHumaError(err)
	}
	return &deleteAcceptanceCriterionOutput{Body: criterion}, nil
}

func normalizeCriterionID(criterionID int) (int, error) {
	if criterionID <= 0 {
		return 0, fmt.Errorf("invalid criterion id")
	}
	return criterionID, nil
}
