package e2e_test

import (
	"encoding/json"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/Performant-Labs/opencloud-registration/internal/config"
	"github.com/Performant-Labs/opencloud-registration/internal/db"
	"github.com/Performant-Labs/opencloud-registration/internal/handler"
	"github.com/Performant-Labs/opencloud-registration/internal/opencloud"
)

// testEnv wires up the full server with a mock OpenCloud backend.
type testEnv struct {
	server     *httptest.Server
	ocServer   *httptest.Server
	ocRequests []*http.Request
	ocStatus   int
	db         *db.DB
	adminToken string
}

func newEnv(t *testing.T, mode string) *testEnv {
	t.Helper()

	env := &testEnv{adminToken: "e2e-token", ocStatus: http.StatusCreated}

	env.ocServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		env.ocRequests = append(env.ocRequests, r.Clone(r.Context()))
		w.WriteHeader(env.ocStatus)
	}))
	t.Cleanup(env.ocServer.Close)

	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	env.db = database
	t.Cleanup(func() { database.Close() })

	cfg := &config.Config{
		RegistrationMode: mode,
		AdminToken:       env.adminToken,
		OCUrl:            env.ocServer.URL,
		OCAdminUser:      "admin",
		OCAdminPassword:  "secret",
		AppBaseURL:       "https://cloud.example.com",
	}

	ocClient := opencloud.NewClient(cfg)
	tmpl := buildTemplates(t)

	regHandler := handler.NewRegisterHandler(cfg, database, ocClient, tmpl)
	adminHandler := handler.NewAdminHandler(cfg, database, ocClient, tmpl)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", regHandler.ShowForm)
	mux.HandleFunc("POST /register", regHandler.HandleSubmit)
	mux.HandleFunc("GET /success", regHandler.ShowSuccess)
	mux.HandleFunc("GET /pending", regHandler.ShowPending)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`)) //nolint:errcheck
	})

	adminMux := http.NewServeMux()
	adminMux.HandleFunc("GET /admin", adminHandler.List)
	adminMux.HandleFunc("POST /admin/approve/{id}", adminHandler.Approve)
	adminMux.HandleFunc("POST /admin/reject/{id}", adminHandler.Reject)
	mux.Handle("/admin", handler.AdminAuth(env.adminToken, adminMux))
	mux.Handle("/admin/", handler.AdminAuth(env.adminToken, adminMux))

	env.server = httptest.NewServer(mux)
	t.Cleanup(env.server.Close)

	return env
}

func (e *testEnv) get(t *testing.T, path string) *http.Response {
	t.Helper()
	resp, err := http.Get(e.server.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func (e *testEnv) htmxPost(t *testing.T, path string, form url.Values) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, e.server.URL+path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("htmx POST %s: %v", path, err)
	}
	return resp
}

func validForm() url.Values {
	return url.Values{
		"display_name":     {"E2E User"},
		"username":         {"e2euser"},
		"email":            {"e2e@example.com"},
		"password":         {"password123"},
		"password_confirm": {"password123"},
	}
}

func TestE2E_HealthCheck(t *testing.T) {
	env := newEnv(t, "open")
	resp := env.get(t, "/health")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d", resp.StatusCode)
	}
	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body) //nolint:errcheck
	if body["status"] != "ok" {
		t.Errorf("body: got %v", body)
	}
}

func TestE2E_OpenRegistration(t *testing.T) {
	env := newEnv(t, "open")

	resp := env.htmxPost(t, "/register", validForm())
	defer resp.Body.Close()

	if resp.Header.Get("HX-Redirect") != "/success" {
		t.Errorf("HX-Redirect: got %q", resp.Header.Get("HX-Redirect"))
	}
	if len(env.ocRequests) != 1 {
		t.Fatalf("expected 1 OC request, got %d", len(env.ocRequests))
	}
	if !strings.HasSuffix(env.ocRequests[0].URL.Path, "/graph/v1.0/users") {
		t.Errorf("OC path: got %s", env.ocRequests[0].URL.Path)
	}
}

func TestE2E_ApprovalFlow(t *testing.T) {
	env := newEnv(t, "approval")

	resp := env.htmxPost(t, "/register", validForm())
	resp.Body.Close()

	if resp.Header.Get("HX-Redirect") != "/pending" {
		t.Errorf("HX-Redirect: got %q", resp.Header.Get("HX-Redirect"))
	}
	if len(env.ocRequests) != 0 {
		t.Error("OC should not be called before approval")
	}

	regs, _ := env.db.ListRegistrationsByStatus("pending")
	if len(regs) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(regs))
	}
	id := regs[0].ID

	approveResp := env.htmxPost(t, "/admin/approve/"+id+"?token="+env.adminToken, url.Values{})
	approveResp.Body.Close()

	if len(env.ocRequests) != 1 {
		t.Fatalf("expected OC to be called on approval, got %d calls", len(env.ocRequests))
	}
	approved, _ := env.db.ListRegistrationsByStatus("approved")
	if len(approved) != 1 {
		t.Errorf("expected 1 approved, got %d", len(approved))
	}
}

func TestE2E_ApprovalReject(t *testing.T) {
	env := newEnv(t, "approval")

	resp := env.htmxPost(t, "/register", validForm())
	resp.Body.Close()

	regs, _ := env.db.ListRegistrationsByStatus("pending")
	id := regs[0].ID

	rejectResp := env.htmxPost(t, "/admin/reject/"+id+"?token="+env.adminToken, url.Values{})
	rejectResp.Body.Close()

	if len(env.ocRequests) != 0 {
		t.Error("OC should not be called on rejection")
	}
	rejected, _ := env.db.ListRegistrationsByStatus("rejected")
	if len(rejected) != 1 {
		t.Errorf("expected 1 rejected, got %d", len(rejected))
	}
}

func TestE2E_DuplicateRegistration(t *testing.T) {
	// In approval mode, pending registrations are stored in the DB, so
	// a second submission with the same username/email is caught before
	// touching OpenCloud.
	env := newEnv(t, "approval")

	r1 := env.htmxPost(t, "/register", validForm())
	r1.Body.Close()

	// Same username, different email
	form2 := validForm()
	form2.Set("email", "other@example.com")
	r2 := env.htmxPost(t, "/register", form2)
	io.Copy(io.Discard, r2.Body) //nolint:errcheck
	r2.Body.Close()

	if r2.Header.Get("HX-Redirect") == "/pending" {
		t.Error("duplicate username should not be accepted")
	}
	// OC must never be called in approval mode before an admin approves
	if len(env.ocRequests) != 0 {
		t.Errorf("OC should not be called in approval mode, got %d calls", len(env.ocRequests))
	}
	regs, _ := env.db.ListRegistrationsByStatus("pending")
	if len(regs) != 1 {
		t.Errorf("only 1 pending registration should exist, got %d", len(regs))
	}
}

func TestE2E_AdminAuthRequired(t *testing.T) {
	env := newEnv(t, "approval")

	cases := []struct{ name, path string }{
		{"no token", "/admin"},
		{"wrong token", "/admin?token=wrong"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := env.get(t, tc.path)
			resp.Body.Close()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("status: got %d, want 401", resp.StatusCode)
			}
		})
	}
}

func TestE2E_FieldValidation(t *testing.T) {
	env := newEnv(t, "open")

	cases := []struct {
		name   string
		mutate func(url.Values)
	}{
		{"blank display_name", func(v url.Values) { v.Del("display_name") }},
		{"blank username", func(v url.Values) { v.Del("username") }},
		{"bad email", func(v url.Values) { v.Set("email", "notanemail") }},
		{"short password", func(v url.Values) { v.Set("password", "short"); v.Set("password_confirm", "short") }},
		{"password mismatch", func(v url.Values) { v.Set("password_confirm", "different!") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			form := validForm()
			tc.mutate(form)
			resp := env.htmxPost(t, "/register", form)
			resp.Body.Close()
			if resp.Header.Get("HX-Redirect") == "/success" {
				t.Error("invalid form should not redirect to /success")
			}
		})
	}
	if len(env.ocRequests) != 0 {
		t.Errorf("OC should never be called for invalid submissions, got %d calls", len(env.ocRequests))
	}
}

func TestE2E_OCError_ShowsFormError(t *testing.T) {
	env := newEnv(t, "open")
	env.ocStatus = http.StatusInternalServerError

	resp := env.htmxPost(t, "/register", validForm())
	io.Copy(io.Discard, resp.Body) //nolint:errcheck
	resp.Body.Close()

	if resp.Header.Get("HX-Redirect") == "/success" {
		t.Error("OC error should not redirect to success")
	}
}

func buildTemplates(t *testing.T) *template.Template {
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

	// Prefer real templates when running from the project root
	if _, err := os.Stat("../templates"); err == nil {
		tmpl, err := template.New("").Funcs(funcMap).ParseGlob("../templates/*.html")
		if err == nil {
			return tmpl
		}
		t.Logf("could not load real templates (%v) — using inline stubs", err)
	}

	src := `
{{define "base"}}OK{{end}}
{{define "register.html"}}{{template "base" .}}{{end}}
{{define "success.html"}}{{template "base" .}}{{end}}
{{define "pending.html"}}{{template "base" .}}{{end}}
{{define "admin.html"}}{{template "base" .}}{{end}}
{{define "admin_row.html"}}{{if .Error}}<td class="error">{{.Error}}</td>{{else}}<td>{{.Status}}</td>{{end}}{{end}}
`
	tmpl, err := template.New("").Funcs(funcMap).Parse(src)
	if err != nil {
		t.Fatalf("parse inline templates: %v", err)
	}
	return tmpl
}
