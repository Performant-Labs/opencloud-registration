package db

import (
	"context"
	"database/sql"
	"fmt"
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
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return &DB{conn: conn}, nil
}

func (d *DB) Close() error {
	return d.conn.Close()
}

func (d *DB) Migrate(ctx context.Context) error {
	_, err := d.conn.ExecContext(ctx, schema)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

func (d *DB) CreateRegistration(ctx context.Context, r *Registration) error {
	_, err := d.conn.ExecContext(ctx,
		`INSERT INTO registrations (id, display_name, username, email, password, status)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		r.ID, r.DisplayName, r.Username, r.Email, r.Password, r.Status,
	)
	if err != nil {
		return fmt.Errorf("create registration: %w", err)
	}
	return nil
}

func (d *DB) GetRegistration(ctx context.Context, id string) (*Registration, error) {
	row := d.conn.QueryRowContext(ctx,
		`SELECT id, display_name, username, email, password, status, created_at, reviewed_at, reviewed_by
		 FROM registrations WHERE id = ?`, id,
	)
	r, err := scanRegistration(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("get registration: not found")
		}
		return nil, fmt.Errorf("get registration: %w", err)
	}
	return r, nil
}

func (d *DB) ListRegistrationsByStatus(ctx context.Context, status string) ([]*Registration, error) {
	rows, err := d.conn.QueryContext(ctx,
		`SELECT id, display_name, username, email, password, status, created_at, reviewed_at, reviewed_by
		 FROM registrations WHERE status = ? ORDER BY created_at ASC`, status,
	)
	if err != nil {
		return nil, fmt.Errorf("list registrations: query: %w", err)
	}
	defer rows.Close()

	var list []*Registration
	for rows.Next() {
		r, err := scanRegistration(rows)
		if err != nil {
			return nil, fmt.Errorf("list registrations: scan: %w", err)
		}
		list = append(list, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list registrations: rows error: %w", err)
	}
	return list, nil
}

func (d *DB) UpdateStatus(ctx context.Context, id, status, reviewedBy string) error {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err := d.conn.ExecContext(ctx,
		`UPDATE registrations SET status = ?, reviewed_at = ?, reviewed_by = ? WHERE id = ?`,
		status, now, reviewedBy, id,
	)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	return nil
}

func (d *DB) ExistsByEmailOrUsername(ctx context.Context, email, username string) (emailTaken, usernameTaken bool, err error) {
	rows, err := d.conn.QueryContext(ctx,
		`SELECT email, username FROM registrations WHERE email = ? OR username = ?`,
		email, username,
	)
	if err != nil {
		return false, false, fmt.Errorf("exists check: query: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var e, u string
		if err := rows.Scan(&e, &u); err != nil {
			return false, false, fmt.Errorf("exists check: scan: %w", err)
		}
		if e == email {
			emailTaken = true
		}
		if u == username {
			usernameTaken = true
		}
	}
	if err := rows.Err(); err != nil {
		return false, false, fmt.Errorf("exists check: rows error: %w", err)
	}
	return emailTaken, usernameTaken, nil
}

func (d *DB) AppendAuditLog(ctx context.Context, regID, action, detail string) error {
	_, err := d.conn.ExecContext(ctx,
		`INSERT INTO audit_log (reg_id, action, detail) VALUES (?, ?, ?)`,
		regID, action, detail,
	)
	if err != nil {
		return fmt.Errorf("append audit log: %w", err)
	}
	return nil
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
