package otp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type TwilioProvider struct {
	accountSID string
	authToken  string
	from       string
	client     *http.Client
}

func NewTwilioProvider(accountSID, authToken, from string) *TwilioProvider {
	return &TwilioProvider{
		accountSID: strings.TrimSpace(accountSID),
		authToken:  strings.TrimSpace(authToken),
		from:       strings.TrimSpace(from),
		client:     &http.Client{},
	}
}

func (t *TwilioProvider) Send(ctx context.Context, phone string, code string) error {
	if t.accountSID == "" || t.authToken == "" || t.from == "" {
		return fmt.Errorf("twilio credentials are incomplete")
	}

	form := url.Values{}
	form.Set("To", phone)
	form.Set("From", t.from)
	form.Set("Body", "Your OTP is "+code)

	endpoint := "https://api.twilio.com/2010-04-01/Accounts/" + t.accountSID + "/Messages.json"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth(t.accountSID, t.authToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("twilio send failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}
