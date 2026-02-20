package service

import "errors"

type Code string

const (
	CodeValidation Code = "validation"
	CodeNotFound   Code = "not_found"
	CodeConflict   Code = "conflict"
	CodeInternal   Code = "internal"
)

type Error struct {
	Code    Code
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return string(e.Code)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func newError(code Code, msg string, err error) *Error {
	return &Error{
		Code:    code,
		Message: msg,
		Err:     err,
	}
}

func CodeOf(err error) Code {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return CodeInternal
}

func MessageOf(err error) string {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.Error()
	}
	if err == nil {
		return ""
	}
	return err.Error()
}
