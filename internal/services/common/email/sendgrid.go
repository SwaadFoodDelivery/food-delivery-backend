package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type SendGridProvider struct {
	apiKey      string
	endpoint    string
	fromAddress string
	fromName    string
	client      *http.Client
}

func NewSendGridProvider(apiKey, endpoint, fromAddress, fromName string) *SendGridProvider {
	return &SendGridProvider{
		apiKey:      strings.TrimSpace(apiKey),
		endpoint:    strings.TrimSpace(endpoint),
		fromAddress: strings.TrimSpace(fromAddress),
		fromName:    strings.TrimSpace(fromName),
		client:      &http.Client{},
	}
}

func (s *SendGridProvider) SendVerificationOTP(ctx context.Context, to string, code string) error {
	if s.apiKey == "" || s.endpoint == "" || s.fromAddress == "" {
		return fmt.Errorf("sendgrid credentials are incomplete")
	}

	payload := sendGridMailRequest{
		Personalizations: []sendGridPersonalization{
			{To: []sendGridEmail{{Email: to}}},
		},
		From: sendGridEmail{
			Email: s.fromAddress,
			Name:  s.fromName,
		},
		Subject: "Verify your email",
		Content: []sendGridContent{
			{
				Type:  "text/plain",
				Value: "Your email verification OTP is " + code + ". It is valid for 10 minutes.",
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("sendgrid send failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

type sendGridMailRequest struct {
	Personalizations []sendGridPersonalization `json:"personalizations"`
	From             sendGridEmail             `json:"from"`
	Subject          string                    `json:"subject"`
	Content          []sendGridContent         `json:"content"`
}

type sendGridPersonalization struct {
	To []sendGridEmail `json:"to"`
}

type sendGridEmail struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

type sendGridContent struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}
