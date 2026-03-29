# opencloud-registration

Self-registration portal for [OpenCloud](https://opencloud.eu). Users fill out a form and accounts are created via the OpenCloud Graph API. Supports **open** (instant) and **approval** (admin-gated) modes.

Built with Go 1.25 + [htmx 2.0.4](https://htmx.org), SQLite (WAL mode), and a single Docker container that plugs into any [opencloud-compose](https://github.com/opencloud-eu/opencloud-compose) stack via Traefik.

---

## Features

- Registration form with per-field inline validation on blur (username, email) via htmx
- Server-side validation of all fields (display name, username, email, password, password confirmation)
- **Open mode** — account created immediately via the Graph API; duplicate usernames and emails are rejected before the API call
- **Approval mode** — submissions queued in SQLite; admin approves or rejects via an htmx-powered dashboard with inline row updates
- Admin dashboard at `/admin` protected by a static token (`?token=` query param or `Authorization: Bearer` header)
- Passwords encrypted at rest with AES-256-GCM (key derived via PBKDF2 from `ADMIN_TOKEN`) in approval mode
- Full audit log of all registration events (submitted, approved, rejected, OC API errors)
- Single self-contained binary with embedded templates and CSS — no external assets at runtime
- Docker Compose add-on — joins `opencloud-net` and self-registers with Traefik

---

## Requirements

- Docker + Docker Compose (or [OrbStack](https://orbstack.dev))
- A running [opencloud-compose](https://github.com/opencloud-eu/opencloud-compose) stack (formerly `pl-opencloud-server`)
- Go 1.25+ (only needed to build locally or run tests)

---

## Installation & Deployment

Please refer to the [installation guide](docs/INSTALLATION.md) for detailed instructions on:
- Setting up the app alongside `pl-opencloud-server`
- Running the app in standalone mode
- Full environment variable reference

---

## Usage

### Open mode (default)

1. Go to **https://register.opencloud.test** — accept the self-signed cert warning.
2. Fill in display name, username, email, and password. Username and email are validated inline on blur.
3. Submit — the account is created immediately and you land on the success page.
4. Click **Sign in to OpenCloud** to go directly to the OpenCloud sign-in page.

Trying to register again with the same email or username shows an error — duplicates are caught in the local database *before* calling the Graph API.

### Approval mode

Set `OC_REG_REGISTRATION_MODE=approval` in `.env` (or pass it directly) and restart the registration container:

```bash
docker compose -f docker-compose.yml -f traefik/opencloud.yml \
  -f ../opencloud-registration/docker-compose.addon.yml \
  up -d registration
```

1. Users register and see a "Registration submitted" page with a pending icon — no account is created yet.
2. Open the admin dashboard: **https://register.opencloud.test/admin?token=your-token**
3. Approve a submission — the password is decrypted, the account is created in OpenCloud via the Graph API, and the row updates inline via htmx.
4. Reject a submission — no account is created; the row updates inline.

The admin dashboard has tab navigation for filtering by status: **Pending**, **Approved**, **Rejected**.

---


## Development

### Run locally without Docker

```bash
cp .env.example .env
# edit .env
source .env
go run ./cmd/server
```

### Testing

Please refer to the [Testing Instructions](docs/TESTING_INSTRUCTIONS.md) for details on running unit tests and the live integration test suite.

### Project structure

```
├── .env.example                     # template with all env vars and comments
├── .gitignore
├── assets.go                        # //go:embed templates static
├── cmd/server/main.go               # entry point: wires config, db, handlers, routes
├── internal/
│   ├── config/
│   │   ├── config.go                # env var loading + validation
│   │   └── config_test.go
│   ├── db/
│   │   ├── db.go                    # SQLite CRUD (WAL mode, foreign keys)
│   │   ├── schema.go                # DDL: registrations + audit_log tables
│   │   └── db_test.go
│   ├── opencloud/
│   │   ├── client.go                # POST /graph/v1.0/users (Basic auth)
│   │   └── client_test.go
│   └── handler/
│       ├── crypto.go                # AES-256-GCM password encrypt/decrypt (PBKDF2 key)
│       ├── middleware.go            # AdminAuth (Bearer header or ?token= query param)
│       ├── register.go              # GET /, POST /register, POST /register/validate/{field}
│       ├── admin.go                 # GET /admin, POST /admin/approve/{id}, /admin/reject/{id}
│       └── handler_test.go
├── templates/
│   ├── base.html                    # shared layout (htmx 2.0.4 from unpkg CDN)
│   ├── register.html                # form with htmx blur validation and fragment rendering
│   ├── success.html                 # "Sign in to OpenCloud" link → /signin/v1/identifier
│   ├── pending.html                 # approval-mode confirmation
│   ├── admin.html                   # dashboard with status tab navigation
│   └── admin_row.html               # htmx row fragment for approve/reject inline updates
├── static/style.css                 # CSS (embedded at build time)
├── e2e/
│   └── registration_test.go         # integration tests against the live stack
├── go.mod                           # module: github.com/Performant-Labs/opencloud-registration
├── go.sum
├── docs/                            # documentation
│   ├── INSTALLATION.md              # installation and deployment details
│   ├── PLAN.md                      # implementation plan / design doc
│   └── TESTING_INSTRUCTIONS.md      # unit and integration testing guide
├── Dockerfile                       # multi-stage: golang:1.25-bookworm → debian:bookworm-slim
├── docker-compose.yml               # standalone dev (port 8080, uses OC_INSECURE directly)
└── docker-compose.addon.yml         # pl-opencloud-server add-on (opencloud-net + Traefik, maps INSECURE → OC_INSECURE)
```

### HTTP routes

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/` | — | Registration form |
| `POST` | `/register` | — | Submit registration (htmx or standard POST) |
| `POST` | `/register/validate/{field}` | — | Per-field blur validation (`username`, `email`) |
| `GET` | `/success` | — | "Account created" confirmation with sign-in link |
| `GET` | `/pending` | — | "Awaiting approval" confirmation |
| `GET` | `/admin` | token | List registrations (filterable by `?status=pending\|approved\|rejected`) |
| `POST` | `/admin/approve/{id}` | token | Approve → decrypt password → create user in OpenCloud |
| `POST` | `/admin/reject/{id}` | token | Reject registration |
| `GET` | `/health` | — | `{"status":"ok"}` |
| `GET` | `/static/*` | — | Embedded CSS |
