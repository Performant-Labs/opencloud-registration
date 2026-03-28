package handler

import (
	"context"
	"fmt"
	"html/template"
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
	tmpl     *template.Template
}

func NewRegisterHandler(cfg *config.Config, database *db.DB, oc *opencloud.Client, tmpl *template.Template) *RegisterHandler {
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
	h.renderForm(w, registerFormData{AppBaseURL: h.cfg.AppBaseURL})
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
		h.renderForm(w, formData)
		return
	}

	// Check uniqueness
	if existing, _ := h.db.ListRegistrationsByStatus("pending"); existing != nil {
		for _, reg := range existing {
			if reg.Username == username {
				formData.Error = "username is already taken"
				h.renderForm(w, formData)
				return
			}
			if reg.Email == email {
				formData.Error = "email is already registered"
				h.renderForm(w, formData)
				return
			}
		}
	}

	id := uuid.New().String()

	if h.cfg.RegistrationMode == "open" {
		err := h.ocClient.CreateUser(context.Background(), opencloud.CreateUserRequest{
			DisplayName:              displayName,
			Mail:                     email,
			OnPremisesSamAccountName: username,
			PasswordProfile:          opencloud.PasswordProfile{Password: password},
		})
		if err != nil {
			_ = h.db.AppendAuditLog(id, "oc_failed", err.Error())
			formData.Error = fmt.Sprintf("Could not create account: %v", err)
			h.renderForm(w, formData)
			return
		}
		_ = h.db.AppendAuditLog(id, "oc_created", "")
		htmxRedirect(w, r, "/success")
		return
	}

	// Approval mode — encrypt password and store
	encrypted, err := encryptPassword(password, h.cfg.AdminToken)
	if err != nil {
		formData.Error = "internal error, please try again"
		h.renderForm(w, formData)
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
	if err := h.db.CreateRegistration(reg); err != nil {
		formData.Error = "Could not save registration, please try again"
		h.renderForm(w, formData)
		return
	}
	_ = h.db.AppendAuditLog(id, "submitted", "")
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
	_ = h.tmpl.ExecuteTemplate(w, "success.html", map[string]string{"AppBaseURL": h.cfg.AppBaseURL})
}

func (h *RegisterHandler) ShowPending(w http.ResponseWriter, r *http.Request) {
	_ = h.tmpl.ExecuteTemplate(w, "pending.html", nil)
}

func (h *RegisterHandler) renderForm(w http.ResponseWriter, data registerFormData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.tmpl.ExecuteTemplate(w, "register.html", data)
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
