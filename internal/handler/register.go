package handler

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/mail"
	"regexp"
	"strings"

	"github.com/Performant-Labs/opencloud-registration/internal/config"
	"github.com/Performant-Labs/opencloud-registration/internal/db"
	"github.com/Performant-Labs/opencloud-registration/internal/opencloud"
	"github.com/google/uuid"
)

var usernameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9\-]{1,31}$`)

type RegisterHandler struct {
	cfg      *config.Config
	db       *db.DB
	ocClient *opencloud.Client
	tmpl     map[string]*template.Template
}

func NewRegisterHandler(cfg *config.Config, database *db.DB, oc *opencloud.Client, tmpl map[string]*template.Template) *RegisterHandler {
	return &RegisterHandler{cfg: cfg, db: database, ocClient: oc, tmpl: tmpl}
}

type registerFormData struct {
	DisplayName string
	Username    string
	Email       string
	Error       string
	AppBaseURL  string
}

func (h *RegisterHandler) ShowForm(w http.ResponseWriter, r *http.Request) {
	h.renderForm(w, r, registerFormData{AppBaseURL: h.cfg.AppBaseURL})
}

func (h *RegisterHandler) HandleSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	displayName := strings.TrimSpace(r.FormValue("display_name"))
	username := strings.TrimSpace(r.FormValue("username"))
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")
	passwordConfirm := r.FormValue("password_confirm")

	formData := registerFormData{
		DisplayName: displayName,
		Username:    username,
		Email:       email,
		AppBaseURL:  h.cfg.AppBaseURL,
	}

	if errMsg := validate(displayName, username, email, password, passwordConfirm); errMsg != "" {
		formData.Error = errMsg
		h.renderForm(w, r, formData)
		return
	}

	// Check uniqueness across all existing registrations
	emailTaken, usernameTaken, _ := h.db.ExistsByEmailOrUsername(r.Context(), email, username)
	if usernameTaken {
		formData.Error = "username is already taken"
		h.renderForm(w, r, formData)
		return
	}
	if emailTaken {
		formData.Error = "email is already registered"
		h.renderForm(w, r, formData)
		return
	}

	id := uuid.New().String()

	if h.cfg.RegistrationMode == "open" {
		err := h.ocClient.CreateUser(r.Context(), opencloud.CreateUserRequest{
			DisplayName:              displayName,
			Mail:                     email,
			OnPremisesSamAccountName: username,
			PasswordProfile:          opencloud.PasswordProfile{Password: password},
		})
		if err != nil {
			_ = h.db.AppendAuditLog(r.Context(), id, "oc_failed", err.Error())
			log.Printf("opencloud user creation failed: %v", err)
			formData.Error = "Could not create account, please try again"
			h.renderForm(w, r, formData)
			return
		}
		_ = h.db.CreateRegistration(r.Context(), &db.Registration{
			ID: id, DisplayName: displayName, Username: username, Email: email, Status: "approved",
		})
		_ = h.db.AppendAuditLog(r.Context(), id, "oc_created", "")
		htmxRedirect(w, r, "/success")
		return
	}

	// Approval mode — encrypt password and store
	encrypted, err := encryptPassword(password, h.cfg.AdminToken)
	if err != nil {
		log.Printf("password encryption failed: %v", err)
		formData.Error = "internal error, please try again"
		h.renderForm(w, r, formData)
		return
	}

	reg := &db.Registration{
		ID:          id,
		DisplayName: displayName,
		Username:    username,
		Email:       email,
		Password:    encrypted,
		Status:      "pending",
	}
	if err := h.db.CreateRegistration(r.Context(), reg); err != nil {
		log.Printf("save pending registration failed: %v", err)
		formData.Error = "Could not save registration, please try again"
		h.renderForm(w, r, formData)
		return
	}
	_ = h.db.AppendAuditLog(r.Context(), id, "submitted", "")
	htmxRedirect(w, r, "/pending")
}

// ValidateField handles per-field blur validation for htmx.
func (h *RegisterHandler) ValidateField(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		return
	}
	field := r.PathValue("field")
	var errMsg string
	switch field {
	case "username":
		u := strings.TrimSpace(r.FormValue("username"))
		if u == "" {
			errMsg = "username is required"
		} else if !usernameRe.MatchString(u) {
			errMsg = "username must be 3–32 lowercase letters, numbers, or hyphens"
		}
	case "email":
		e := strings.TrimSpace(r.FormValue("email"))
		if _, err := mail.ParseAddress(e); err != nil {
			errMsg = "invalid email address"
		}
	}
	w.Header().Set("Content-Type", "text/html")
	if errMsg != "" {
		fmt.Fprintf(w, `<span class="field-error">%s</span>`, template.HTMLEscapeString(errMsg))
	} else {
		fmt.Fprint(w, `<span class="field-error"></span>`)
	}
}

func (h *RegisterHandler) ShowSuccess(w http.ResponseWriter, r *http.Request) {
	_ = h.tmpl["success.html"].ExecuteTemplate(w, "success.html", map[string]string{
		"OCUrl":    h.cfg.OCUrl,
		"SigninURL": h.cfg.OCUrl + "/signin/v1/identifier",
	})
}

func (h *RegisterHandler) ShowPending(w http.ResponseWriter, r *http.Request) {
	_ = h.tmpl["pending.html"].ExecuteTemplate(w, "pending.html", nil)
}

func (h *RegisterHandler) renderForm(w http.ResponseWriter, r *http.Request, data registerFormData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Header.Get("HX-Request") == "true" {
		_ = h.tmpl["register.html"].ExecuteTemplate(w, "form-fragment", data)
		return
	}
	_ = h.tmpl["register.html"].ExecuteTemplate(w, "register.html", data)
}

// htmxRedirect sends an HX-Redirect for htmx requests and a normal 303 otherwise.
func htmxRedirect(w http.ResponseWriter, r *http.Request, url string) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", url)
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, url, http.StatusSeeOther)
}

func validate(displayName, username, email, password, passwordConfirm string) string {
	if displayName == "" {
		return "display name is required"
	}
	if username == "" {
		return "username is required"
	}
	if !usernameRe.MatchString(username) {
		return "username must be 3–32 lowercase letters, numbers, or hyphens"
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return "invalid email address"
	}
	if len(password) < 8 {
		return "password must be at least 8 characters"
	}
	if password != passwordConfirm {
		return "passwords do not match"
	}
	return ""
}
