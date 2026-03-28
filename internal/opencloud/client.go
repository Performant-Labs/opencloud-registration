package opencloud

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Performant-Labs/opencloud-registration/internal/config"
)

type Client struct {
	baseURL    string
	adminUser  string
	adminPass  string
	httpClient *http.Client
}

type CreateUserRequest struct {
	DisplayName              string          `json:"displayName"`
	Mail                     string          `json:"mail"`
	OnPremisesSamAccountName string          `json:"onPremisesSamAccountName"`
	PasswordProfile          PasswordProfile `json:"passwordProfile"`
}

type PasswordProfile struct {
	Password string `json:"password"`
}

func NewClient(cfg *config.Config) *Client {
	transport := &http.Transport{}
	if cfg.OCInsecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}
	return &Client{
		baseURL:   cfg.OCUrl,
		adminUser: cfg.OCAdminUser,
		adminPass: cfg.OCAdminPassword,
		httpClient: &http.Client{
			Timeout:   10 * time.Second,
			Transport: transport,
		},
	}
}

func (c *Client) CreateUser(ctx context.Context, req CreateUserRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/graph/v1.0/users", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.SetBasicAuth(c.adminUser, c.adminPass)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("call OpenCloud API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated {
		return nil
	}

	detail, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("OpenCloud returned HTTP %d: %s", resp.StatusCode, string(detail))
}
