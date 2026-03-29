# Installation & Deployment

## Installation

### 1. Clone alongside pl-opencloud-server

```bash
cd ~/Sites
git clone https://github.com/Performant-Labs/opencloud-registration
```

The repository must sit at `../opencloud-registration` relative to `pl-opencloud-server` (the default). Override with `REGISTRATION_APP_PATH` in your `.env` if your layout differs.

### 2. Configure pl-opencloud-server

Add the following to `~/Sites/pl-opencloud-server/.env`:

```dotenv
ADMIN_TOKEN=change-me-secret          # protects /admin — use a real secret
REGISTRATION_DOMAIN=register.opencloud.test
REGISTRATION_MODE=open                # open | approval
```

> **Note:** The registration container derives `OC_URL` from `OC_DOMAIN` (defaulting to `cloud.opencloud.test`) and uses the stack-wide `INSECURE` variable (not `OC_INSECURE`) for TLS verification. It reads `OC_ADMIN_PASSWORD` directly (defaulting to `admin`). These are already set by the stock `pl-opencloud-server/.env`, so you typically don't need to touch them.

### 3. Add the hostname (local dev only)

```bash
sudo sh -c 'echo "127.0.0.1  register.opencloud.test" >> /etc/hosts'
```

### 4. Start the stack

From `~/Sites/pl-opencloud-server/`:

```bash
./occtl start
```

Check that the registration container is healthy:

```bash
./occtl status
```

The registration app is now available at **https://register.opencloud.test**.

---

## Deployment

### As a Docker Compose add-on (recommended)

Include `docker-compose.addon.yml` in your compose invocation:

```bash
docker compose \
  -f docker-compose.yml \
  -f traefik/opencloud.yml \
  -f ../opencloud-registration/docker-compose.addon.yml \
  up -d --build
```

Or add it permanently to `COMPOSE_FILE` in `pl-opencloud-server/.env`:

```dotenv
COMPOSE_FILE=docker-compose.yml:traefik/opencloud.yml:../opencloud-registration/docker-compose.addon.yml
```

The container joins the `opencloud-net` network and registers its Traefik routing labels automatically. Registration data is persisted in a named Docker volume (`registration-data`).

### Standalone (without pl-opencloud-server)

```bash
cp .env.example .env
# edit .env — set OC_URL, OC_ADMIN_PASSWORD, ADMIN_TOKEN at minimum
docker compose up -d --build
```

The standalone `docker-compose.yml` uses `OC_INSECURE` directly (not `INSECURE`). The app runs on `http://localhost:8080`.

---

## Configuration Overview

The application supports multiple configuration layers (in order of priority):
1. Environment Variables (Highest)
2. External YAML Configuration (`/data/config.yml`)
3. Built-in defaults

By placing a `config.yml` in your data volume, your config will persist and not be overwritten when the OpenCloud registration container is updated.

### Application variables

These are used by the registration server binary itself (set in the container environment):

| Variable | Default | Required | Description |
|---|---|---|---|
| `OC_REG_ADMIN_TOKEN` | — | ✓ | Token for the `/admin` dashboard (also used as the key seed for password encryption in approval mode) |
| `OC_REG_OC_URL` | — | ✓ | OpenCloud base URL (e.g. `https://cloud.opencloud.test`) |
| `OC_REG_OC_ADMIN_PASSWORD` | — | ✓ | OpenCloud admin password (used for Basic auth against the Graph API) |
| `OC_REG_OC_ADMIN_USER` | `admin` | | OpenCloud admin username |
| `OC_REG_OC_INSECURE` | `false` | | Skip TLS verification (`true` for self-signed certs) |
| `OC_REG_REGISTRATION_MODE` | `open` | | `open` or `approval` |
| `OC_REG_APP_BASE_URL` | `http://localhost:8080` | | Public URL of this app (used in templates) |
| `OC_REG_DB_PATH` | `/data/registration.db` | | SQLite database path |
| `OC_REG_LISTEN_ADDR` | `:8080` | | HTTP listen address |
| `OC_REG_CONFIG_PATH` | `/data/registration.yaml` | | Optional YAML configuration file path |
| `OC_REG_TEMPLATE_DIR` | — | | Path to directory containing custom `templates/` and `static/` files |

### Compose add-on variables

These are set in `pl-opencloud-server/.env` and consumed by `docker-compose.addon.yml` to configure the container:

| Variable | Default | Description |
|---|---|---|
| `REGISTRATION_DOMAIN` | `register.opencloud.test` | Hostname for Traefik routing and `APP_BASE_URL` |
| `REGISTRATION_APP_PATH` | `../opencloud-registration` | Path to this repo (relative to `pl-opencloud-server/`) |
| `REGISTRATION_MODE` | `open` | Passed through to the container |
| `ADMIN_TOKEN` | — (required) | Passed through to the container |
| `OC_DOMAIN` | `cloud.opencloud.test` | Used to derive `OC_URL` (`https://${OC_DOMAIN}`) |
| `OC_ADMIN_USER` | `admin` | Passed through |
| `OC_ADMIN_PASSWORD` | `admin` | Passed through |
| `INSECURE` | `false` | Mapped to the container's `OC_INSECURE` |
| `LOG_DRIVER` | `local` | Docker log driver for the container |

> **⚠️ Important:** The add-on maps the stack-wide `INSECURE` variable to the container's `OC_INSECURE`. If you set `OC_INSECURE=true` in your `.env` it will have no effect on the registration container — set `INSECURE=true` instead (which `pl-opencloud-server` already does by default for local dev).

See `.env.example` for a full template.
