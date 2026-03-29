package handler

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Performant-Labs/opencloud-registration/internal/config"
	"github.com/Performant-Labs/opencloud-registration/internal/db"
	"github.com/Performant-Labs/opencloud-registration/internal/opencloud"
)

// mockOCClient satisfies the interface expected by handlers via duck-typing.
// We wrap it in a thin adapter so handlers can accept it.
type mockOCClient struct {
	err      error
	captured *opencloud.CreateUserRequest
}

func (m *mockOCClient) CreateUser(_ context.Context, req opencloud.CreateUserRequest) error {
	m.captured = &req
	return m.err
}

// handlerOCClient is a local interface matching what handlers call on the client.
type handlerOCClient interface {
	CreateUser(context.Context, opencloud.CreateUserRequest) error
}

// We need the real opencloud.Client to satisfy the concrete type in handlers,
// so we use httptest to spin up a mock OC server instead.
func mockOCServer(t *testing.T, status int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newTestDeps(t *testing.T, mode string, ocStatus int) (*config.Config, *db.DB, *opencloud.Client, map[string]*template.Template) {
	t.Helper()

	srv := mockOCServer(t, ocStatus)

	cfg := &config.Config{
		RegistrationMode: mode,
		AdminToken:       "test-token",
		OCUrl:            srv.URL,
		OCAdminUser:      "admin",
		OCAdminPassword:  "secret",
		AppBaseURL:       "https://cloud.example.com",
	}

	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	ocClient := opencloud.NewClient(cfg)

	tmpl := mustParseTemplates(t)

	return cfg, database, ocClient, tmpl
}

func mustParseTemplates(t *testing.T) map[string]*template.Template {
	t.Helper()
	funcMap := template.FuncMap{
		"map": func(pairs ...any) map[string]any {
			m := make(map[string]any, len(pairs)/2)
			for i := 0; i+1 < len(pairs); i += 2 {
				if k, ok := pairs[i].(string); ok {
					m[k] = pairs[i+1]
				}
			}
			return m
		},
	}
	// Use minimal inline templates for unit tests
	src := `
{{define "register.html"}}{{template "base" .}}{{end}}
{{define "success.html"}}{{template "base" .}}{{end}}
{{define "pending.html"}}{{template "base" .}}{{end}}
{{define "admin.html"}}{{template "base" .}}{{end}}
{{define "admin_row.html"}}{{if .Error}}<td class="error">{{.Error}}</td>{{else}}<td>{{.Status}}</td>{{end}}{{end}}
{{define "base"}}OK{{end}}
`
	tmpl, err := template.New("").Funcs(funcMap).Parse(src)
	if err != nil {
		t.Fatalf("parse templates: %v", err)
	}
	return map[string]*template.Template{
		"register.html":  tmpl,
		"success.html":   tmpl,
		"pending.html":   tmpl,
		"admin.html":     tmpl,
		"admin_row.html": tmpl,
	}
}

func postForm(values url.Values) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(values.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func validForm() url.Values {
	return url.Values{
		"display_name":     {"Test User"},
		"username":         {"testuser"},
		"email":            {"test@example.com"},
		"password":         {"password123"},
		"password_confirm": {"password123"},
	}
}

// --- Register handler tests ---

func TestShowForm_GET(t *testing.T) {
	cfg, db, oc, tmpl := newTestDeps(t, "open", http.StatusCreated)
	h := NewRegisterHandler(cfg, db, oc, tmpl)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ShowForm(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d", w.Code)
	}
}

func TestHandleRegister_ValidationErrors(t *testing.T) {
	cfg, database, oc, tmpl := newTestDeps(t, "open", http.StatusCreated)
	h := NewRegisterHandler(cfg, database, oc, tmpl)

	cases := []struct {
		name   string
		mutate func(url.Values)
	}{
		{"missing display_name", func(v url.Values) { v.Del("display_name") }},
		{"missing username", func(v url.Values) { v.Del("username") }},
		{"bad email", func(v url.Values) { v.Set("email", "notanemail") }},
		{"short password", func(v url.Values) { v.Set("password", "short"); v.Set("password_confirm", "short") }},
		{"password mismatch", func(v url.Values) { v.Set("password_confirm", "different") }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			form := validForm()
			tc.mutate(form)

			w := httptest.NewRecorder()
			h.HandleSubmit(w, postForm(form))

			// On validation error the form is re-rendered (200), not redirected
			if w.Code != http.StatusOK {
				t.Errorf("status: got %d, want 200", w.Code)
			}

			regs, _ := database.ListRegistrationsByStatus(context.Background(), "pending")
			if len(regs) != 0 {
				t.Error("no registration should have been created")
			}
		})
	}
}

func TestHandleRegister_OpenMode_Success(t *testing.T) {
	cfg, database, oc, tmpl := newTestDeps(t, "open", http.StatusCreated)
	h := NewRegisterHandler(cfg, database, oc, tmpl)

	w := httptest.NewRecorder()
	r := postForm(validForm())
	r.Header.Set("HX-Request", "true")
	h.HandleSubmit(w, r)

	if w.Header().Get("HX-Redirect") != "/success" {
		t.Errorf("HX-Redirect: got %q", w.Header().Get("HX-Redirect"))
	}

	// In open mode, nothing should be stored in the DB
	regs, _ := database.ListRegistrationsByStatus(context.Background(), "pending")
	if len(regs) != 0 {
		t.Error("open mode should not write to DB")
	}
}

func TestHandleRegister_OpenMode_OCError(t *testing.T) {
	cfg, database, oc, tmpl := newTestDeps(t, "open", http.StatusInternalServerError)
	h := NewRegisterHandler(cfg, database, oc, tmpl)

	w := httptest.NewRecorder()
	h.HandleSubmit(w, postForm(validForm()))

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d", w.Code)
	}
}

func TestHandleRegister_ApprovalMode(t *testing.T) {
	cfg, database, oc, tmpl := newTestDeps(t, "approval", http.StatusCreated)
	h := NewRegisterHandler(cfg, database, oc, tmpl)

	w := httptest.NewRecorder()
	r := postForm(validForm())
	r.Header.Set("HX-Request", "true")
	h.HandleSubmit(w, r)

	if w.Header().Get("HX-Redirect") != "/pending" {
		t.Errorf("HX-Redirect: got %q", w.Header().Get("HX-Redirect"))
	}

	regs, _ := database.ListRegistrationsByStatus(context.Background(), "pending")
	if len(regs) != 1 {
		t.Fatalf("expected 1 pending registration, got %d", len(regs))
	}
	if regs[0].Username != "testuser" {
		t.Errorf("username: got %q", regs[0].Username)
	}
}

func TestHandleRegister_DuplicateUsername(t *testing.T) {
	cfg, database, oc, tmpl := newTestDeps(t, "approval", http.StatusCreated)
	h := NewRegisterHandler(cfg, database, oc, tmpl)

	// First registration
	r1 := postForm(validForm())
	r1.Header.Set("HX-Request", "true")
	h.HandleSubmit(httptest.NewRecorder(), r1)

	// Second registration with same username
	form2 := validForm()
	form2.Set("email", "other@example.com")
	w := httptest.NewRecorder()
	h.HandleSubmit(w, postForm(form2))

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d", w.Code)
	}

	regs, _ := database.ListRegistrationsByStatus(context.Background(), "pending")
	if len(regs) != 1 {
		t.Errorf("should still have exactly 1 pending registration, got %d", len(regs))
	}
}

// --- Admin middleware tests ---

func TestAdminAuth_Unauthorized(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	protected := AdminAuth("secret", next)

	cases := []struct {
		name string
		req  *http.Request
	}{
		{"no token", httptest.NewRequest("GET", "/admin", nil)},
		{"wrong token", func() *http.Request {
			r := httptest.NewRequest("GET", "/admin?token=wrong", nil)
			return r
		}()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			protected.ServeHTTP(w, tc.req)
			if w.Code != http.StatusUnauthorized {
				t.Errorf("status: got %d, want 401", w.Code)
			}
		})
	}
}

func TestAdminAuth_Authorized(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	protected := AdminAuth("secret", next)

	cases := []struct {
		name string
		req  *http.Request
	}{
		{"query param", httptest.NewRequest("GET", "/admin?token=secret", nil)},
		{"bearer header", func() *http.Request {
			r := httptest.NewRequest("GET", "/admin", nil)
			r.Header.Set("Authorization", "Bearer secret")
			return r
		}()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			protected.ServeHTTP(w, tc.req)
			if w.Code != http.StatusOK {
				t.Errorf("status: got %d, want 200", w.Code)
			}
		})
	}
}

// --- Crypto tests ---

func TestEncryptDecrypt(t *testing.T) {
	token := "my-admin-token"
	plain := "supersecretpassword"

	encrypted, err := encryptPassword(plain, token)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	decrypted, err := decryptPassword(encrypted, token)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if decrypted != plain {
		t.Errorf("got %q, want %q", decrypted, plain)
	}
}

func TestEncryptDecrypt_WrongToken(t *testing.T) {
	encrypted, _ := encryptPassword("secret", "token-a")
	_, err := decryptPassword(encrypted, "token-b")
	if err == nil {
		t.Fatal("expected error when decrypting with wrong token")
	}
}

func TestEncrypt_UniqueEachTime(t *testing.T) {
	e1, _ := encryptPassword("same", "token")
	e2, _ := encryptPassword("same", "token")
	if e1 == e2 {
		t.Error("two encryptions of the same value should differ (random nonce)")
	}
}
