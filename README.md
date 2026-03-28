# opencloud-registration

Self-registration portal for [OpenCloud](https://opencloud.eu). Users fill out a form and accounts are created via the OpenCloud Graph API. Supports **open** (instant) and **approval** (admin-gated) modes.

Built with Go + [htmx](https://htmx.org), SQLite, and a single Docker container that plugs into any `pl-opencloud-server` stack via Traefik.

---

## Features

- Registration form with per-field validation (display name, username, email, password)
- **Open mode** — account created immediately on submission
- **Approval mode** — submissions queued in SQLite; admin approves or rejects via a dashboard
- Admin dashboard at `/admin` protected by a static token
- Passwords encrypted at rest (AES-256-GCM) in approval mode
- Audit log of all registration events
- Single self-contained binary with embedded templates and CSS
- Docker Compose add-on — joins `opencloud-net` and registers itself with Traefik

---

## Requirements

- Docker + Docker Compose
- A running [pl-opencloud-server](https://github.com/opencloud-eu/opencloud-compose) stack
- The `opencloud-net` Docker network (created by pl-opencloud-server)

---

## Quick start (local dev)

### 1. Add env vars to pl-opencloud-server

Open `~/Sites/pl-opencloud-server/.env` and add:

```dotenv
ADMIN_TOKEN=localtest
REGISTRATION_DOMAIN=register.opencloud.test
REGISTRATION_MODE=open
```

### 2. Add the hostname to `/etc/hosts`

```bash
sudo sh -c 'echo "127.0.0.1  register.opencloud.test" >> /etc/hosts'
```

### 3. Start the stack

From `~/Sites/pl-opencloud-server/` using `occtl`:

```bash
./occtl start
```

Or manually:

```bash
docker compose \
  -f docker-compose.yml \
  -f traefik/opencloud.yml \
  -f ../opencloud-registration/docker-compose.addon.yml \
  up -d --build
```

Wait ~30 seconds, then check the registration container is healthy:

```bash
./occtl status
```

---

## Testing the registration flow

### Open mode (instant account creation)

1. Open **https://register.opencloud.test** — accept the self-signed cert warning.
2. Try submitting a blank form — you should see "display name is required".
3. Try a bad username like `AB` (too short, uppercase) — you should see the format error.
4. Try mismatched passwords — you should see "passwords do not match".
5. Fill in valid details and submit:
   - Display name: `Test User`
   - Username: `testuser`
   - Email: `test@example.com`
   - Password: `TestPass123`
6. You should land on the `/success` page.

**Verify the account was created:**

```bash
curl -sk -u admin:admin \
  https://cloud.opencloud.test/graph/v1.0/users \
  | python3 -m json.tool | grep -A5 testuser
```

Or go to **https://cloud.opencloud.test** → admin menu → **User Management** and look for `testuser`.

**Log in as the new user:**

Go to **https://cloud.opencloud.test** and sign in with `testuser` / `TestPass123`.

---

### Approval mode (admin-gated)

1. In `pl-opencloud-server/.env`, change:
   ```dotenv
   REGISTRATION_MODE=approval
   ```

2. Restart the registration container:
   ```bash
   ./occtl restart
   # or just the registration service:
   docker compose ... up -d registration
   ```

3. Register a new user (e.g. username `pendinguser`) — you should land on `/pending`. No account is created yet.

4. Open the admin dashboard:
   **https://register.opencloud.test/admin?token=localtest**

5. You should see `pendinguser` in the pending list.

6. Click **Approve** — the row updates and the account is created in OpenCloud. Verify with the `curl` command above.

7. Register a second user and click **Reject** — the row disappears and no account is created.

---

## Environment variables

| Variable | Default | Required | Description |
|---|---|---|---|
| `OC_URL` | — | ✓ | OpenCloud base URL |
| `OC_ADMIN_PASSWORD` | — | ✓ | OpenCloud admin password |
| `ADMIN_TOKEN` | — | ✓ | Token to access the `/admin` dashboard |
| `OC_ADMIN_USER` | `admin` | | OpenCloud admin username |
| `OC_INSECURE` | `false` | | Skip TLS verification (set `true` for self-signed certs) |
| `REGISTRATION_MODE` | `open` | | `open` or `approval` |
| `APP_BASE_URL` | `http://localhost:8080` | | Public URL of this app (used in success page link) |
| `DB_PATH` | `/data/registration.db` | | SQLite database path |
| `LISTEN_ADDR` | `:8080` | | HTTP listen address |

See `.env.example` for a full template.

---

## Deployment

### As a Docker Compose add-on (recommended)

Add to `pl-opencloud-server/.env`:

```dotenv
ADMIN_TOKEN=your-secret-token
REGISTRATION_DOMAIN=register.yourdomain.com
REGISTRATION_MODE=open          # or: approval
OC_ADMIN_PASSWORD=your-admin-password
```

Then include `docker-compose.addon.yml` in your compose command or `COMPOSE_FILE`:

```dotenv
COMPOSE_FILE=docker-compose.yml:traefik/opencloud.yml:../opencloud-registration/docker-compose.addon.yml
```

Or pass it directly:

```bash
docker compose \
  -f docker-compose.yml \
  -f traefik/opencloud.yml \
  -f ../opencloud-registration/docker-compose.addon.yml \
  up -d --build
```

### Standalone

Copy `.env.example` to `.env`, fill in values, then:

```bash
docker compose up -d --build
```

The app will be available at `http://localhost:8080`.

---

## Development

### Run locally (without Docker)

```bash
cp .env.example .env
# edit .env with your values

source .env
go run ./cmd/server
```

### Run tests

```bash
go test ./...
```

### Project structure

```
├── assets.go                    # embeds templates/ and static/ into the binary
├── cmd/server/main.go           # entry point
├── internal/
│   ├── config/                  # env var loading and validation
│   ├── db/                      # SQLite layer (registrations + audit_log)
│   ├── opencloud/               # Graph API client
│   └── handler/                 # HTTP handlers, crypto, middleware
├── templates/                   # htmx HTML templates
├── static/style.css             # minimal CSS
├── Dockerfile                   # multi-stage build
├── docker-compose.yml           # standalone dev
└── docker-compose.addon.yml     # pl-opencloud-server add-on
```
