# Security To-Do

Items identified during the April 2026 security review, ordered by priority.
`subtle.ConstantTimeCompare` for admin token comparison was already fixed (`aa4bfda`).

---

## 1. Rate-limit `POST /register` against enumeration and queue spam

**Files:** `internal/handler/register.go`, `cmd/server/main.go`

**Risk:** Two distinct error messages (`"username is already taken"` /
`"email is already registered"`) are returned from a SQLite lookup that runs
*before* any OpenCloud call. An attacker can enumerate registered
usernames and email addresses at full Go-server speed without OpenCloud's
own rate limiting ever applying.

In **approval mode** the situation is worse: OpenCloud is never called until
an admin approves, so the pending queue can be flooded with thousands of
crafted entries at zero cost.

> Note: OpenCloud's Graph API rate limiting *does* protect against account-
> creation spam in open mode, but it provides no protection for the SQLite
> enumeration path or approval-mode queue writes.

**Suggested fix:** Add a simple per-IP token-bucket or sliding-window counter
on `POST /register` — e.g. 5 attempts per minute per IP. A lightweight
in-memory counter (sync.Map + time.Now) is sufficient; no external dependency
needed.

```
// rough shape
type rateLimiter struct {
    mu      sync.Mutex
    buckets map[string]*bucket
}
```

---

## 2. Remove or scope the `?token=` query-string fallback on `/admin`

**File:** `internal/handler/middleware.go`

**Risk:** Token values passed in the URL appear in:
- Server access logs (stdout via Docker daemon)
- Browser history
- `Referer` headers sent to any third-party resource loaded by the admin page
- Reverse-proxy (Traefik) access logs

The `Authorization: Bearer <token>` header path is already implemented and
correct. The `?token=` fallback should either be:

- **Removed** (preferred for production hardening), or
- **Restricted to localhost** (check `r.RemoteAddr`), or
- **Logged with a deprecation warning** so operators know it is being used

The admin UI currently embeds the token back into every action URL, which
amplifies the exposure:

```go
// admin.go — token round-tripped through every htmx action
http.Redirect(w, r, "/admin?token="+token, http.StatusSeeOther)
```

Consider storing the token in a short-lived, HttpOnly, Secure cookie on first
auth instead of re-embedding it in URLs.

---

## 3. Align password validation with OpenCloud's actual policy

**File:** `internal/handler/register.go` (`validate` function)

**Risk / UX:** The current minimum is `len(password) < 8`. If OpenCloud
enforces a stricter policy (complexity, common-password blocklist, etc.) the
Graph API call will fail and the user sees the generic message
`"Could not create account, please try again"` with no indication of what
went wrong.

**Suggested fix:**
- Investigate what OpenCloud actually enforces via `POST /graph/v1.0/users`
  and mirror that logic in `validate()`.
- Parse the Graph API error body on failure and surface a human-readable
  reason in `formData.Error` where safe to do so (avoid leaking internal
  details, but "Password does not meet complexity requirements" is
  appropriate to show).

---

## 4. Scope the OpenCloud admin credential (structural / operational)

**Env var:** `OC_REG_OC_ADMIN_PASSWORD`

**Risk:** The registration service holds a full OpenCloud admin password in
its environment. A container breakout gives an attacker superuser access to
the entire OpenCloud instance.

**Suggested fix:** Create a dedicated, scoped service account in OpenCloud
(if the Graph API supports it) that has only `User.ReadWrite` permission,
rather than using the top-level `admin` account. Document this as a
deployment requirement in `docs/INSTALLATION.md`.

---

## 5. Document `OC_REG_OC_INSECURE` production posture

**File:** `docker-compose.yml`, `docs/INSTALLATION.md`

**Risk:** The standalone `docker-compose.yml` does not set `OC_REG_OC_INSECURE`
at all. A developer running against a self-signed OpenCloud instance who sets
`OC_INSECURE=true` in their shell can accidentally carry that setting into a
production deploy without realising it.

**Suggested fix:** Explicitly set `OC_REG_OC_INSECURE: "false"` in both
compose files so the value is always visible and must be consciously overridden,
and add a startup log warning if the value is `true`:

```go
if cfg.OCInsecure {
    log.Println("WARNING: TLS verification disabled (OC_REG_OC_INSECURE=true) — do not use in production")
}
```
