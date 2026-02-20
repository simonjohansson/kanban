package kanban

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCLIErrorAndHelpers(t *testing.T) {
	t.Parallel()

	require.Equal(t, "boom", (&cliError{message: "boom"}).Error())
	require.Equal(t, "fallback", extractErrorMessage([]byte(`{"error":"fallback"}`)))
	require.Equal(t, "", extractErrorMessage([]byte(`not-json`)))
	var target *cliError
	require.True(t, asCLIError(&cliError{message: "x"}, &target))
	require.False(t, asCLIError(errors.New("x"), &target))
}

func TestHandleResponseBranches(t *testing.T) {
	t.Parallel()

	t.Run("request error maps gateway", func(t *testing.T) {
		err := handleResponse(OutputJSON, io.Discard, nil, errors.New("network down"))
		require.Error(t, err)
		var cErr *cliError
		require.True(t, asCLIError(err, &cErr))
		require.Equal(t, http.StatusBadGateway, cErr.status)
	})

	t.Run("success json empty body emits empty object", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("   "))}
		var out bytes.Buffer
		err := handleResponse(OutputJSON, &out, resp, nil)
		require.NoError(t, err)
		require.Equal(t, "{}\n", out.String())
	})

	t.Run("success text empty body emits ok", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(""))}
		var out bytes.Buffer
		err := handleResponse(OutputText, &out, resp, nil)
		require.NoError(t, err)
		require.Equal(t, "ok\n", out.String())
	})

	t.Run("success json non-json body wraps as result", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("plain"))}
		var out bytes.Buffer
		err := handleResponse(OutputJSON, &out, resp, nil)
		require.NoError(t, err)
		require.Equal(t, "{\"result\":\"plain\"}\n", out.String())
	})

	t.Run("error status with json payload preserves raw json in cli error", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusUnprocessableEntity, Body: io.NopCloser(strings.NewReader(`{"detail":"bad input"}`))}
		err := handleResponse(OutputJSON, io.Discard, resp, nil)
		require.Error(t, err)
		var cErr *cliError
		require.True(t, asCLIError(err, &cErr))
		require.Equal(t, http.StatusUnprocessableEntity, cErr.status)
		require.Equal(t, "bad input", cErr.message)
		require.JSONEq(t, `{"detail":"bad input"}`, string(cErr.rawJSON))
	})

	t.Run("error status plain body uses body as message", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusBadRequest, Body: io.NopCloser(strings.NewReader("oops"))}
		err := handleResponse(OutputText, io.Discard, resp, nil)
		require.Error(t, err)
		var cErr *cliError
		require.True(t, asCLIError(err, &cErr))
		require.Equal(t, "oops", cErr.message)
	})
}

func TestFormatErrorTextFallbackStatus(t *testing.T) {
	t.Parallel()
	line := FormatError(OutputText, http.StatusNotFound, "")
	require.Contains(t, line, "Not Found")
}
