# opencloud-registration — Implementation Plan

Self-registration portal for OpenCloud. Users fill out a form; accounts are created via the OpenCloud Graph API. Supports **open** (instant) and **approval** (admin-gated) modes.

---

## Architecture

```
opencloud-registration/
├── assets.go                          # //go:embed templates static
├── cmd/server/main.go                 # entry point: wire config, db, handlers
├── internal/
│   ├── config/config.go               # env var loading + validation
│   ├── db/
│   │   ├── db.go                      # SQLite CRUD (Open, Migrate, Create, List, Update, Audit)
│   │   └── schema.go                  # DDL: registrations + audit_log tables
│   ├── opencloud/client.go            # POST /graph/v1.0/users (Basic auth)
│   └── handler/
│       ├── crypto.go                  # AES-256-GCM password encrypt/decrypt
│       ├── middleware.go              # AdminAuth (Bearer or ?token= query param)
│       ├── register.go                # GET /, POST /register, /success, /pending
│       └── admin.go                   # GET /admin, POST /admin/approve/{id}, /admin/reject/{id}
├── templates/                         # htmx HTML (embedded in binary)
│   ├── base.html, register.html, success.html, pending.html
│   ├── admin.html, admin_row.html
├── static/style.css                   # minimal CSS (embedded in binary)
├── Dockerfile                         # multi-stage: golang:1.22-bookworm → debian:bookworm-slim
├── docker-compose.yml                 # standalone dev (port 8080)
├── docker-compose.addon.yml           # add-on for pl-opencloud-server (opencloud-net + Traefik)
└── .env.example
```

---

## HTTP Routes

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/` | — | Registration form |
| POST | `/register` | — | Submit registration |
| POST | `/register/validate/{field}` | — | Per-field blur validation (htmx) |
| GET | `/success` | — | Account created confirmation |
| GET | `/pending` | — | Awaiting approval confirmation |
| GET | `/admin` | token | List registrations (`?status=pending\|approved\|rejected`) |
| POST | `/admin/approve/{id}` | token | Approve → create user in OpenCloud |
| POST | `/admin/reject/{id}` | token | Reject registration |
| GET | `/health` | — | `{"status":"ok"}` |
| GET | `/static/` | — | Embedded CSS |

---

## Registration Modes

### `REGISTRATION_MODE=open`
1. User submits form → validated
2. `POST /graph/v1.0/users` called immediately
3. Success → redirect `/success` | Failure → form re-rendered with error

### `REGISTRATION_MODE=approval`
1. User submits form → validated
2. Password encrypted with AES-256-GCM (key derived from `ADMIN_TOKEN`)
3. `Registration` record written to SQLite with `status=pending`
4. Redirect → `/pending`
5. Admin visits `/admin?token=...`, approves or rejects
6. On approval: password decrypted → `POST /graph/v1.0/users` → status → `approved`

---

## Environment Variables

| Variable | Default | Required |
|----------|---------|----------|
| `OC_URL` | — | ✓ |
| `OC_ADMIN_PASSWORD` | — | ✓ |
| `ADMIN_TOKEN` | — | ✓ |
| `OC_ADMIN_USER` | `admin` | |
| `OC_INSECURE` | `false` | |
| `REGISTRATION_MODE` | `open` | |
| `APP_BASE_URL` | `http://localhost:8080` | |
| `DB_PATH` | `/data/registration.db` | |
| `LISTEN_ADDR` | `:8080` | |

---

## Deployment (add-on to pl-opencloud-server)

```bash
# From pl-opencloud-server/
docker compose \
  -f docker-compose.yml \
  -f traefik/opencloud.yml \
  -f ../opencloud-registration/docker-compose.addon.yml \
  up -d --build
```

Add to `.env` in `pl-opencloud-server/`:
```dotenv
ADMIN_TOKEN=change-me-secret
REGISTRATION_DOMAIN=register.opencloud.test
REGISTRATION_MODE=open          # or: approval
```

Add to `/etc/hosts` for local dev:
```
127.0.0.1  register.opencloud.test
```

---

## Tests

### Unit (`go test ./internal/...`)
- **config**: required fields, defaults, invalid mode
- **db**: migrate idempotent, CRUD, UNIQUE constraints, audit log
- **opencloud**: 201/400/409/network error, Basic auth header
- **handler**: form validation, open/approval flows, crypto round-trip, admin auth

### E2E (`go test ./e2e/...`)
- Health check
- Open registration → OC called, redirect `/success`
- Approval flow: submit → pending → admin approve → OC called → status approved
- Approval reject → OC never called → status rejected
- Duplicate username rejected (approval mode)
- All validation error cases → no OC calls
- OC error → form re-rendered, no redirect
- Admin auth required (no token / wrong token → 401)

### Run all
```bash
go test ./...
```
