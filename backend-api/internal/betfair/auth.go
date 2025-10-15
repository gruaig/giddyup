package betfair

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	AuthURL = "https://identitysso.betfair.com/api/login"
)

// Authenticator handles Betfair login
type Authenticator struct {
	appKey   string
	username string
	password string
}

// NewAuthenticator creates a new authenticator
func NewAuthenticator(appKey, username, password string) *Authenticator {
	return &Authenticator{
		appKey:   appKey,
		username: username,
		password: password,
	}
}

// Login authenticates with Betfair and returns a session token
func (a *Authenticator) Login() (string, error) {
	form := url.Values{}
	form.Set("username", a.username)
	form.Set("password", a.password)

	req, err := http.NewRequest(http.MethodPost, AuthURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("create login request: %w", err)
	}

	req.Header.Set("X-Application", a.appKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("perform login request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read login response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("login failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	type loginResponse struct {
		SessionToken string `json:"sessionToken"`
		Token        string `json:"token"`
		LoginStatus  string `json:"loginStatus"`
		Status       string `json:"status"`
		Error        string `json:"error"`
	}

	var lr loginResponse
	if err := json.Unmarshal(body, &lr); err != nil {
		return "", fmt.Errorf("decode login response: %w", err)
	}

	status := strings.ToUpper(firstNonEmpty(lr.LoginStatus, lr.Status))
	if status != "" && status != "SUCCESS" {
		return "", fmt.Errorf("login failed: %s - %s", status, lr.Error)
	}

	token := firstNonEmpty(lr.SessionToken, lr.Token)
	if token == "" {
		// Try to get from cookies
		for _, cookie := range resp.Cookies() {
			if strings.EqualFold(cookie.Name, "ssoid") {
				token = cookie.Value
				break
			}
		}
	}

	if token == "" {
		return "", fmt.Errorf("login response did not include a session token")
	}

	return token, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

