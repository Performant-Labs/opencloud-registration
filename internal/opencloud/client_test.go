package opencloud

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Performant-Labs/opencloud-registration/internal/config"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	cfg := &config.Config{
		OCUrl:           srv.URL,
		OCAdminUser:     "admin",
		OCAdminPassword: "secret",
	}
	return NewClient(cfg), srv
}

func validReq() CreateUserRequest {
	return CreateUserRequest{
		DisplayName:              "Test User",
		Mail:                     "test@example.com",
		OnPremisesSamAccountName: "testuser",
		PasswordProfile:          PasswordProfile{Password: "password123"},
	}
}

func TestCreateUser_201(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method: got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/graph/v1.0/users") {
			t.Errorf("path: got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
	})

	if err := client.CreateUser(context.Background(), validReq()); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestCreateUser_400(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
	})

	err := client.CreateUser(context.Background(), validReq())
	if err == nil {
		t.Fatal("expected error for 400")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("expected 400 in error, got: %v", err)
	}
}

func TestCreateUser_409(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"user already exists"}`, http.StatusConflict)
	})

	err := client.CreateUser(context.Background(), validReq())
	if err == nil {
		t.Fatal("expected error for 409")
	}
	if !strings.Contains(err.Error(), "409") {
		t.Errorf("expected 409 in error, got: %v", err)
	}
}

func TestCreateUser_NetworkError(t *testing.T) {
	cfg := &config.Config{
		OCUrl:           "http://127.0.0.1:1", // nothing listening
		OCAdminUser:     "admin",
		OCAdminPassword: "secret",
	}
	client := NewClient(cfg)

	err := client.CreateUser(context.Background(), validReq())
	if err == nil {
		t.Fatal("expected network error")
	}
	var netErr *net.OpError
	if !isNetError(err) && netErr == nil {
		// just check the message contains something useful
		if !strings.Contains(err.Error(), "OpenCloud API") {
			t.Errorf("unexpected error: %v", err)
		}
	}
}

func isNetError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*net.OpError)
	return ok
}

func TestCreateUser_SendsBasicAuth(t *testing.T) {
	var gotUser, gotPass string
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotUser, gotPass, _ = r.BasicAuth()
		w.WriteHeader(http.StatusCreated)
	})

	_ = client.CreateUser(context.Background(), validReq())

	if gotUser != "admin" {
		t.Errorf("basic auth user: got %q", gotUser)
	}
	if gotPass != "secret" {
		t.Errorf("basic auth pass: got %q", gotPass)
	}
}
