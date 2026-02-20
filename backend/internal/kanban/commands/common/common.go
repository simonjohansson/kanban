package common

import (
	"io"
	"net/http"

	apiclient "github.com/simonjohansson/kanban/backend/gen/client"
)

type Runtime interface {
	ServerURL() string
	Output() string
}

type HandleResponseFunc func(output string, stdout io.Writer, resp *http.Response, reqErr error) error

type WrapErrorFunc func(status int, message string) error

func NewClient(runtime Runtime) (*apiclient.Client, error) {
	return apiclient.NewClient(runtime.ServerURL())
}
