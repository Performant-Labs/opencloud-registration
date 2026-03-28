package db

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Registration struct {
	ID          string
	DisplayName string
	Username    string
	Email       string
	Password    string // encrypted ciphertext (approval mode) or empty (open mode)
	Status      string // pending | approved | rejected
	CreatedAt   string
	ReviewedAt  *string
	ReviewedBy  *string
}

type DB struct {
	conn *sql.DB
}

func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, err
	}
	if err := conn.Ping(); err != nil {
		return nil, err
	}
	return &DB{conn: conn}, nil
}

func (d *DB) Close() error {
	return d.conn.Close()
}

func (d *DB) Migrate() error {
	_, err := d.conn.Exec(schema)
	return err
}

func (d *DB) CreateRegistration(r *Registration) error {
	_, err := d.conn.Exec(
		`INSERT INTO registrations (id, display_name, username, email, password, status)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		r.ID, r.DisplayName, r.Username, r.Email, r.Password, r.Status,
	)
	return err
}

func (d *DB) GetRegistration(id string) (*Registration, error) {
	row := d.conn.QueryRow(
		`SELECT id, display_name, username, email, password, status, created_at, reviewed_at, reviewed_by
		 FROM registrations WHERE id = ?`, id,
	)
	return scanRegistration(row)
}

func (d *DB) ListRegistrationsByStatus(status string) ([]*Registration, error) {
	rows, err := d.conn.Query(
		`SELECT id, display_name, username, email, password, status, created_at, reviewed_at, reviewed_by
		 FROM registrations WHERE status = ? ORDER BY created_at ASC`, status,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*Registration
	for rows.Next() {
		r, err := scanRegistration(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, rows.Err()
}

func (d *DB) UpdateStatus(id, status, reviewedBy string) error {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err := d.conn.Exec(
		`UPDATE registrations SET status = ?, reviewed_at = ?, reviewed_by = ? WHERE id = ?`,
		status, now, reviewedBy, id,
	)
	return err
}

func (d *DB) AppendAuditLog(regID, action, detail string) error {
	_, err := d.conn.Exec(
		`INSERT INTO audit_log (reg_id, action, detail) VALUES (?, ?, ?)`,
		regID, action, detail,
	)
	return err
}

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanRegistration(s scanner) (*Registration, error) {
	var r Registration
	err := s.Scan(
		&r.ID, &r.DisplayName, &r.Username, &r.Email,
		&r.Password, &r.Status, &r.CreatedAt, &r.ReviewedAt, &r.ReviewedBy,
	)
	if err != nil {
		return nil, err
	}
	return &r, nil
}
