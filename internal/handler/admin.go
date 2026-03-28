package handler

import (
	"context"
	"fmt"
	"html/template"
	"net/http"

	"github.com/Performant-Labs/opencloud-registration/internal/config"
	"github.com/Performant-Labs/opencloud-registration/internal/db"
	"github.com/Performant-Labs/opencloud-registration/internal/opencloud"
)

type AdminHandler struct {
	cfg      *config.Config
	db       *db.DB
	ocClient *opencloud.Client
	tmpl     *template.Template
}

func NewAdminHandler(cfg *config.Config, database *db.DB, oc *opencloud.Client, tmpl *template.Template) *AdminHandler {
	return &AdminHandler{cfg: cfg, db: database, ocClient: oc, tmpl: tmpl}
}

type adminPageData struct {
	Registrations []*db.Registration
	Status        string
	AdminToken    string
}

func (h *AdminHandler) List(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "pending"
	}
	regs, err := h.db.ListRegistrationsByStatus(status)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	token := r.URL.Query().Get("token")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.tmpl.ExecuteTemplate(w, "admin.html", adminPageData{
		Registrations: regs,
		Status:        status,
		AdminToken:    token,
	})
}

func (h *AdminHandler) Approve(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	reg, err := h.db.GetRegistration(id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	plainPassword, err := decryptPassword(reg.Password, h.cfg.AdminToken)
	if err != nil {
		h.respondError(w, r, id, fmt.Sprintf("could not decrypt password: %v", err))
		return
	}

	err = h.ocClient.CreateUser(context.Background(), opencloud.CreateUserRequest{
		DisplayName:              reg.DisplayName,
		Mail:                     reg.Email,
		OnPremisesSamAccountName: reg.Username,
		PasswordProfile:          opencloud.PasswordProfile{Password: plainPassword},
	})
	if err != nil {
		_ = h.db.AppendAuditLog(id, "oc_failed", err.Error())
		h.respondError(w, r, id, fmt.Sprintf("OpenCloud error: %v", err))
		return
	}

	_ = h.db.UpdateStatus(id, "approved", "admin")
	_ = h.db.AppendAuditLog(id, "oc_created", "")

	h.respondRow(w, r, reg, "approved", "")
}

func (h *AdminHandler) Reject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	reg, err := h.db.GetRegistration(id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	_ = h.db.UpdateStatus(id, "rejected", "admin")
	_ = h.db.AppendAuditLog(id, "rejected", "")

	h.respondRow(w, r, reg, "rejected", "")
}

// respondRow returns either an htmx fragment or a full-page redirect.
func (h *AdminHandler) respondRow(w http.ResponseWriter, r *http.Request, reg *db.Registration, status, errMsg string) {
	if r.Header.Get("HX-Request") != "true" {
		token := r.URL.Query().Get("token")
		http.Redirect(w, r, "/admin?token="+token, http.StatusSeeOther)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.tmpl.ExecuteTemplate(w, "admin_row.html", map[string]any{
		"Reg":    reg,
		"Status": status,
		"Error":  errMsg,
	})
}

func (h *AdminHandler) respondError(w http.ResponseWriter, r *http.Request, id, errMsg string) {
	if r.Header.Get("HX-Request") != "true" {
		http.Error(w, errMsg, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<tr id="reg-%s"><td colspan="5" class="error">%s</td></tr>`,
		template.HTMLEscapeString(id), template.HTMLEscapeString(errMsg))
}
