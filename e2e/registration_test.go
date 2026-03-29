// Integration tests — run against the live stack (./occtl start).
// Skipped automatically when the stack is not reachable.
//
// Defaults match pl-opencloud-server/.env; override with env vars:
//
//	REGISTRATION_URL   https://register.opencloud.test
//	OC_URL             https://cloud.opencloud.test
//	ADMIN_TOKEN        localtest
//	OC_ADMIN_USER      admin
//	OC_ADMIN_PASSWORD  admin
package e2e_test

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// ── Config ────────────────────────────────────────────────────────────────────

var (
	regURL   = envOr("REGISTRATION_URL", "https://register.opencloud.test")
	ocURL    = envOr("OC_URL", "https://cloud.opencloud.test")
	regToken = envOr("ADMIN_TOKEN", "localtest")
	ocUser   = envOr("OC_ADMIN_USER", "admin")
	ocPass   = envOr("OC_ADMIN_PASSWORD", "admin")
)

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ── HTTP client ───────────────────────────────────────────────────────────────

// newClient returns an HTTPS client that accepts self-signed certs and does
// NOT follow redirects, so tests can assert on redirect targets directly.
func newClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// requireStack skips the test if the registration service is not reachable.
func requireStack(t *testing.T, client *http.Client) {
	t.Helper()
	resp, err := client.Get(regURL + "/health")
	if err != nil {
		t.Skipf("stack not reachable (%v) — run ./occtl start", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Skipf("health check returned %d — stack may be starting up", resp.StatusCode)
	}
}

// ── Test user helpers ─────────────────────────────────────────────────────────

// runID returns a short suffix unique to this test run so users created in
// parallel or repeated runs don't collide.
var runID = fmt.Sprintf("%d", time.Now().UnixMilli()%1_000_000)

func testUsername(name string) string { return fmt.Sprintf("t%s-%s", runID, name) }
func testEmail(name string) string    { return fmt.Sprintf("t%s-%s@test.example", runID, name) }

// deleteOCUser removes a user from OpenCloud by username, failing silently if
// the user doesn't exist (idempotent cleanup).
func deleteOCUser(t *testing.T, client *http.Client, username string) {
	t.Helper()

	req, _ := http.NewRequest(http.MethodGet, ocURL+"/graph/v1.0/users", nil)
	req.SetBasicAuth(ocUser, ocPass)
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return
	}
	defer resp.Body.Close()

	var result struct {
		Value []struct {
			ID                       string `json:"id"`
			OnPremisesSamAccountName string `json:"onPremisesSamAccountName"`
		} `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return
	}
	for _, u := range result.Value {
		if u.OnPremisesSamAccountName == username {
			del, _ := http.NewRequest(http.MethodDelete, ocURL+"/graph/v1.0/users/"+u.ID, nil)
			del.SetBasicAuth(ocUser, ocPass)
			client.Do(del) //nolint:errcheck
			return
		}
	}
}

// ── Request helpers ───────────────────────────────────────────────────────────

func get(t *testing.T, client *http.Client, path string) *http.Response {
	t.Helper()
	resp, err := client.Get(regURL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func htmxPost(t *testing.T, client *http.Client, path string, form url.Values) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, regURL+path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(b)
}

// register submits the registration form and returns the response.
// It registers cleanup of the OC user so the test is repeatable.
func register(t *testing.T, client *http.Client, username, email string) *http.Response {
	t.Helper()
	t.Cleanup(func() { deleteOCUser(t, client, username) })
	form := url.Values{
		"display_name":     {username},
		"username":         {username},
		"email":            {email},
		"password":         {"Password123!"},
		"password_confirm": {"Password123!"},
	}
	return htmxPost(t, client, "/register", form)
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestLive_HealthCheck(t *testing.T) {
	client := newClient()
	requireStack(t, client)

	resp := get(t, client, "/health")
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d", resp.StatusCode)
	}
	if !strings.Contains(body, `"ok"`) {
		t.Errorf("expected {\"status\":\"ok\"}, got: %s", body)
	}
}

// Regression: template set shared across pages caused success.html's content
// block to win alphabetically, so GET / showed the success page instead of
// the registration form.
func TestLive_RegistrationFormAtRoot(t *testing.T) {
	client := newClient()
	requireStack(t, client)

	body := readBody(t, get(t, client, "/"))

	if !strings.Contains(body, "form-container") {
		t.Errorf("GET / should render the registration form; got:\n%s", body)
	}
	if strings.Contains(body, "Go to OpenCloud") {
		t.Error("GET / must not render the success page content")
	}
}

// Regression: success page linked to AppBaseURL (registration app) instead of
// the OpenCloud URL. Also verifies it links to the sign-in page, not the root,
// so a returning admin session doesn't mask the new user's login.
func TestLive_SuccessPageLinksToOpenCloud(t *testing.T) {
	client := newClient()
	requireStack(t, client)

	body := readBody(t, get(t, client, "/success"))

	signinURL := ocURL + "/signin/v1/identifier"
	if !strings.Contains(body, signinURL) {
		t.Errorf("success page should link to sign-in page (%s); body:\n%s", signinURL, body)
	}
	if strings.Contains(body, regURL) {
		t.Errorf("success page must not link back to the registration app (%s)", regURL)
	}
}

// Full open-mode flow: submit form → redirect to /success → every field the
// user typed in the form is present in the OpenCloud user record.
func TestLive_OpenRegistration_CreatesUser(t *testing.T) {
	client := newClient()
	requireStack(t, client)

	username := testUsername("reg")
	displayName := "Reg Test User"
	email := testEmail("reg")

	t.Cleanup(func() { deleteOCUser(t, client, username) })

	form := url.Values{
		"display_name":     {displayName},
		"username":         {username},
		"email":            {email},
		"password":         {"TestPass123"},
		"password_confirm": {"TestPass123"},
	}
	resp := htmxPost(t, client, "/register", form)
	defer resp.Body.Close()

	if resp.Header.Get("HX-Redirect") != "/success" {
		t.Fatalf("expected HX-Redirect to /success, got %q", resp.Header.Get("HX-Redirect"))
	}

	// Verify every submitted field landed in the OpenCloud user record.
	req, _ := http.NewRequest(http.MethodGet, ocURL+"/graph/v1.0/users", nil)
	req.SetBasicAuth(ocUser, ocPass)
	ocResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("query OC users: %v", err)
	}
	defer ocResp.Body.Close()

	var result struct {
		Value []struct {
			DisplayName              string `json:"displayName"`
			OnPremisesSamAccountName string `json:"onPremisesSamAccountName"`
			Mail                     string `json:"mail"`
		} `json:"value"`
	}
	if err := json.NewDecoder(ocResp.Body).Decode(&result); err != nil {
		t.Fatalf("decode OC response: %v", err)
	}
	for _, u := range result.Value {
		if u.OnPremisesSamAccountName == username {
			if u.DisplayName != displayName {
				t.Errorf("displayName: got %q, want %q", u.DisplayName, displayName)
			}
			if u.Mail != email {
				t.Errorf("mail: got %q, want %q", u.Mail, email)
			}
			return
		}
	}
	t.Errorf("user %q not found in OpenCloud after registration", username)
}

// Full open-mode flow including credential verification: the registered user
// must be able to authenticate against OpenCloud, not just exist in the user list.
func TestLive_OpenRegistration_UserCanLogin(t *testing.T) {
	client := newClient()
	requireStack(t, client)

	username := testUsername("login")
	email := testEmail("login")
	password := "TestPass123"

	form := url.Values{
		"display_name":     {username},
		"username":         {username},
		"email":            {email},
		"password":         {password},
		"password_confirm": {password},
	}
	t.Cleanup(func() { deleteOCUser(t, client, username) })

	resp := htmxPost(t, client, "/register", form)
	resp.Body.Close()
	if resp.Header.Get("HX-Redirect") != "/success" {
		t.Fatalf("registration failed — no redirect to /success")
	}

	// Verify credentials work against OpenCloud WebDAV.
	req, _ := http.NewRequest(http.MethodGet, ocURL+"/dav/files/"+username+"/", nil)
	req.SetBasicAuth(username, password)
	davResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("WebDAV request failed: %v", err)
	}
	davResp.Body.Close()

	if davResp.StatusCode != http.StatusOK {
		t.Errorf("new user could not log in: WebDAV returned %d (expected 200)", davResp.StatusCode)
	}
}

// Regression: duplicate email was not caught in open mode because the DB
// was never written to, so the uniqueness check always found nothing.
func TestLive_DuplicateEmail_Rejected(t *testing.T) {
	client := newClient()
	requireStack(t, client)

	username := testUsername("dupe-email")
	email := testEmail("dupe-email")

	r1 := register(t, client, username, email)
	r1.Body.Close()
	if r1.Header.Get("HX-Redirect") != "/success" {
		t.Fatal("first registration should succeed")
	}

	// Same email, different username.
	r2 := register(t, client, testUsername("dupe-email-b"), email)
	body := readBody(t, r2)

	if r2.Header.Get("HX-Redirect") == "/success" {
		t.Error("duplicate email should be rejected")
	}
	if !strings.Contains(body, "email is already registered") {
		t.Errorf("expected 'email is already registered' error; got:\n%s", body)
	}
}

func TestLive_DuplicateUsername_Rejected(t *testing.T) {
	client := newClient()
	requireStack(t, client)

	username := testUsername("dupe-user")

	r1 := register(t, client, username, testEmail("dupe-user"))
	r1.Body.Close()
	if r1.Header.Get("HX-Redirect") != "/success" {
		t.Fatal("first registration should succeed")
	}

	// Same username, different email.
	r2 := register(t, client, username, testEmail("dupe-user-b"))
	body := readBody(t, r2)

	if r2.Header.Get("HX-Redirect") == "/success" {
		t.Error("duplicate username should be rejected")
	}
	if !strings.Contains(body, "username is already taken") {
		t.Errorf("expected 'username is already taken' error; got:\n%s", body)
	}
}

func TestLive_AdminAuth_Required(t *testing.T) {
	client := newClient()
	requireStack(t, client)

	cases := []struct{ name, path string }{
		{"no token", "/admin"},
		{"wrong token", "/admin?token=wrong"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := get(t, client, tc.path)
			resp.Body.Close()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("expected 401, got %d", resp.StatusCode)
			}
		})
	}
}

func TestLive_FieldValidation_Rejected(t *testing.T) {
	client := newClient()
	requireStack(t, client)

	base := url.Values{
		"display_name":     {testUsername("val")},
		"username":         {testUsername("val")},
		"email":            {testEmail("val")},
		"password":         {"Password123!"},
		"password_confirm": {"Password123!"},
	}

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
			form := url.Values{}
			for k, v := range base {
				form[k] = v
			}
			tc.mutate(form)
			resp := htmxPost(t, client, "/register", form)
			resp.Body.Close()
			if resp.Header.Get("HX-Redirect") == "/success" {
				t.Errorf("invalid input (%s) should not redirect to /success", tc.name)
			}
		})
	}
}

// Regression: htmx form error responses returned the full page (base template +
// header + card) inside the existing page, causing nested rendering. The fix
// renders only the #form-container fragment for htmx requests. This test ensures
// that regression stays caught.
func TestLive_HtmxErrorResponse_IsFragment(t *testing.T) {
	client := newClient()
	requireStack(t, client)

	// Submit invalid form via htmx — blank display_name triggers validation error.
	form := url.Values{
		"display_name":     {""},
		"username":         {testUsername("frag")},
		"email":            {testEmail("frag")},
		"password":         {"TestPass123"},
		"password_confirm": {"TestPass123"},
	}
	resp := htmxPost(t, client, "/register", form)
	body := readBody(t, resp)

	// The response must be just the form fragment, not a full HTML page.
	if strings.Contains(body, "<html") {
		t.Error("htmx error response contains <html> — full page returned instead of fragment")
	}
	if strings.Contains(body, "<header") {
		t.Error("htmx error response contains <header> — full page returned instead of fragment")
	}
	if !strings.Contains(body, `id="form-container"`) {
		t.Errorf("htmx error response missing #form-container; got:\n%s", body)
	}
	if !strings.Contains(body, "error-banner") {
		t.Errorf("htmx error response should contain the error banner; got:\n%s", body)
	}
}

// Walks the complete registration flow as a connected sequence:
//  1. GET /             → see the form
//  2. POST /register    → htmx redirect to /success
//  3. GET /success      → see sign-in link
//  4. GET sign-in link  → OpenCloud login page loads
//  5. WebDAV auth       → new credentials work
func TestLive_FullFlow_FormToLogin(t *testing.T) {
	client := newClient()
	requireStack(t, client)

	username := testUsername("flow")
	email := testEmail("flow")
	password := "TestPass123"
	t.Cleanup(func() { deleteOCUser(t, client, username) })

	// 1. GET / — registration form
	formPage := readBody(t, get(t, client, "/"))
	if !strings.Contains(formPage, `id="form-container"`) {
		t.Fatalf("step 1: GET / did not return the registration form")
	}

	// 2. POST /register — submit via htmx
	form := url.Values{
		"display_name":     {username},
		"username":         {username},
		"email":            {email},
		"password":         {password},
		"password_confirm": {password},
	}
	regResp := htmxPost(t, client, "/register", form)
	regResp.Body.Close()
	redirect := regResp.Header.Get("HX-Redirect")
	if redirect != "/success" {
		t.Fatalf("step 2: expected HX-Redirect /success, got %q", redirect)
	}

	// 3. GET /success — follow the redirect, verify content
	successBody := readBody(t, get(t, client, redirect))
	signinURL := ocURL + "/signin/v1/identifier"
	if !strings.Contains(successBody, signinURL) {
		t.Fatalf("step 3: success page missing sign-in link (%s)", signinURL)
	}

	// 4. Follow the sign-in link — OpenCloud login page loads
	signinResp := get(t, client, "")
	// signinURL is absolute, not relative to registration server
	signinReq, _ := http.NewRequest(http.MethodGet, signinURL, nil)
	signinResp, err := client.Do(signinReq)
	if err != nil {
		t.Fatalf("step 4: sign-in page request failed: %v", err)
	}
	signinBody := readBody(t, signinResp)
	if signinResp.StatusCode != http.StatusOK {
		t.Fatalf("step 4: sign-in page returned %d", signinResp.StatusCode)
	}
	if !strings.Contains(signinBody, "Sign in") {
		t.Errorf("step 4: sign-in page doesn't contain 'Sign in'")
	}

	// 5. Authenticate as the new user via WebDAV
	davReq, _ := http.NewRequest(http.MethodGet, ocURL+"/dav/files/"+username+"/", nil)
	davReq.SetBasicAuth(username, password)
	davResp, err := client.Do(davReq)
	if err != nil {
		t.Fatalf("step 5: WebDAV request failed: %v", err)
	}
	davResp.Body.Close()
	if davResp.StatusCode != http.StatusOK {
		t.Errorf("step 5: new user login failed — WebDAV returned %d", davResp.StatusCode)
	}
}

// Verifies that a registration in open mode shows up in the admin panel.
func TestLive_AdminPanel_ShowsRegistration(t *testing.T) {
	client := newClient()
	requireStack(t, client)

	username := testUsername("admpanel")
	email := testEmail("admpanel")
	t.Cleanup(func() { deleteOCUser(t, client, username) })

	// Register a user.
	resp := register(t, client, username, email)
	resp.Body.Close()
	if resp.Header.Get("HX-Redirect") != "/success" {
		t.Fatalf("registration failed")
	}

	// Check the admin panel lists the registration.
	adminResp := get(t, client, "/admin?token="+regToken+"&status=approved")
	body := readBody(t, adminResp)

	if adminResp.StatusCode != http.StatusOK {
		t.Fatalf("admin panel returned %d", adminResp.StatusCode)
	}
	if !strings.Contains(body, username) {
		t.Errorf("admin panel does not list registered user %q; body:\n%s", username, body)
	}
	if !strings.Contains(body, email) {
		t.Errorf("admin panel does not show email %q", email)
	}
}
