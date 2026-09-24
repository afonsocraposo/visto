package pushover

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const messagesEndpoint = "https://api.pushover.net/1/messages.json"

type Client struct {
	appToken string
	endpoint string
	http     *http.Client
}

func NewClient(appToken string, client *http.Client) (*Client, error) {
	appToken = strings.TrimSpace(appToken)
	if appToken == "" {
		return nil, errors.New("Pushover application token is required")
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{appToken: appToken, endpoint: messagesEndpoint, http: client}, nil
}

func (client *Client) Send(ctx context.Context, userKey, title, message string) error {
	form := url.Values{
		"token":   {client.appToken},
		"user":    {userKey},
		"title":   {title},
		"message": {message},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("create Pushover request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := client.http.Do(request)
	if err != nil {
		return fmt.Errorf("send Pushover notification: %w", err)
	}
	defer response.Body.Close()
	var result struct {
		Status int      `json:"status"`
		Errors []string `json:"errors"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return fmt.Errorf("read Pushover response: %w", err)
	}
	if response.StatusCode != http.StatusOK || result.Status != 1 {
		return fmt.Errorf("Pushover rejected notification (HTTP %d): %s", response.StatusCode, strings.Join(result.Errors, ", "))
	}
	return nil
}
