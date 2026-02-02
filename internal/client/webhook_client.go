package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type WebhookClient struct {
	url    string
	client *http.Client
}

func NewWebhookClient(url string) *WebhookClient {
	return &WebhookClient{
		url: url,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type sendRequest struct {
	PhoneNumber string `json:"phoneNumber"`
	Message     string `json:"message"`
}

type sendResponse struct {
	Message   string `json:"message"`
	MessageID string `json:"messageId"`
}

func (c *WebhookClient) Send(ctx context.Context, phoneNumber, message string) (string, error) {
	reqBody, err := json.Marshal(sendRequest{
		PhoneNumber: phoneNumber,
		Message:     message,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusAccepted {
		return "", fmt.Errorf("unexpected status code: %d body=%q", resp.StatusCode, string(body))
	}

	var sr sendResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		return "", fmt.Errorf("failed to decode json: %w body=%q", err, string(body))
	}
	if sr.MessageID == "" {
		return "", fmt.Errorf("missing messageId in response body=%q", string(body))
	}

	return sr.MessageID, nil
}
