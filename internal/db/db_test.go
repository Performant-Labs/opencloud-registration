package db

import (
	"strings"
	"testing"
)

func openMemDB(t *testing.T) *DB {
	t.Helper()
	d, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := d.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func newReg(id, username, email string) *Registration {
	return &Registration{
		ID:          id,
		DisplayName: "Test User",
		Username:    username,
		Email:       email,
		Password:    "encrypted-blob",
		Status:      "pending",
	}
}

func TestMigrate_Idempotent(t *testing.T) {
	d := openMemDB(t)
	if err := d.Migrate(); err != nil {
		t.Fatalf("second Migrate failed: %v", err)
	}
}

func TestCreateAndGetRegistration(t *testing.T) {
	d := openMemDB(t)

	reg := newReg("id-1", "alice", "alice@example.com")
	if err := d.CreateRegistration(reg); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := d.GetRegistration("id-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Username != "alice" {
		t.Errorf("username: got %q", got.Username)
	}
	if got.Email != "alice@example.com" {
		t.Errorf("email: got %q", got.Email)
	}
	if got.Status != "pending" {
		t.Errorf("status: got %q", got.Status)
	}
}

func TestListRegistrationsByStatus(t *testing.T) {
	d := openMemDB(t)

	for i, u := range []string{"bob", "carol", "dave"} {
		_ = d.CreateRegistration(newReg("id-"+u, u, u+"-"+string(rune('a'+i))+"@example.com"))
	}
	approved := newReg("id-eve", "eve", "eve@example.com")
	approved.Status = "approved"
	_ = d.CreateRegistration(approved)

	pending, err := d.ListRegistrationsByStatus("pending")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(pending) != 3 {
		t.Errorf("pending count: got %d, want 3", len(pending))
	}

	approvedList, _ := d.ListRegistrationsByStatus("approved")
	if len(approvedList) != 1 {
		t.Errorf("approved count: got %d, want 1", len(approvedList))
	}
}

func TestUpdateStatus(t *testing.T) {
	d := openMemDB(t)
	_ = d.CreateRegistration(newReg("id-1", "frank", "frank@example.com"))

	if err := d.UpdateStatus("id-1", "approved", "admin"); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, _ := d.GetRegistration("id-1")
	if got.Status != "approved" {
		t.Errorf("status: got %q", got.Status)
	}
	if got.ReviewedAt == nil {
		t.Error("reviewed_at should be set")
	}
	if got.ReviewedBy == nil || *got.ReviewedBy != "admin" {
		t.Errorf("reviewed_by: got %v", got.ReviewedBy)
	}
}

func TestDuplicateUsername(t *testing.T) {
	d := openMemDB(t)
	_ = d.CreateRegistration(newReg("id-1", "grace", "grace@example.com"))

	err := d.CreateRegistration(newReg("id-2", "grace", "other@example.com"))
	if err == nil {
		t.Fatal("expected UNIQUE constraint error")
	}
	if !strings.Contains(err.Error(), "UNIQUE") {
		t.Errorf("expected UNIQUE error, got: %v", err)
	}
}

func TestDuplicateEmail(t *testing.T) {
	d := openMemDB(t)
	_ = d.CreateRegistration(newReg("id-1", "henry", "shared@example.com"))

	err := d.CreateRegistration(newReg("id-2", "irene", "shared@example.com"))
	if err == nil {
		t.Fatal("expected UNIQUE constraint error")
	}
}

func TestAuditLog(t *testing.T) {
	d := openMemDB(t)
	_ = d.CreateRegistration(newReg("id-1", "judy", "judy@example.com"))

	if err := d.AppendAuditLog("id-1", "submitted", ""); err != nil {
		t.Fatalf("audit log: %v", err)
	}

	var count int
	_ = d.conn.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE reg_id = 'id-1'`).Scan(&count)
	if count != 1 {
		t.Errorf("audit log count: got %d, want 1", count)
	}
}
