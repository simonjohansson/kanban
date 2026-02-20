package kanban

import (
	"fmt"
	"net/url"
	"strings"
)

func BuildWebsocketURL(serverURL string, project string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(serverURL))
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid server url")
	}

	wsScheme := "ws"
	if parsed.Scheme == "https" {
		wsScheme = "wss"
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("server url must start with http:// or https://")
	}

	wsURL := &url.URL{
		Scheme: wsScheme,
		Host:   parsed.Host,
		Path:   "/ws",
	}

	if value := strings.TrimSpace(project); value != "" {
		q := wsURL.Query()
		q.Set("project", value)
		wsURL.RawQuery = q.Encode()
	}

	return wsURL.String(), nil
}
