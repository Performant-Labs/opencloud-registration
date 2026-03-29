# Testing Instructions

This document explains how to run tests for `opencloud-registration`. The testing approach covers both fast unit tests for internal packages and comprehensive end-to-end (E2E) integration tests against a live OpenCloud stack.

---

## Unit Tests

Unit test files (`*_test.go`) exist for all internal packages: `config`, `db`, `opencloud`, and `handler`. 

To run all unit tests:

```bash
go test ./internal/...
```

These tests verify internal logic (e.g., SQLite DB schema setup, config parsing, OpenCloud API error handling) and do not require external services to be running.

---

## Integration / End-to-End Tests

The tests in `e2e/` run against the live stack. They create and clean up their own test users in OpenCloud to ensure repeatability without leaving artifacts behind.

### 1. Start the Stack

You must have the underlying `pl-opencloud-server` stack running before executing E2E tests:

```bash
cd ~/Sites/pl-opencloud-server
./occtl start
```

### 2. Run the tests

From the repository root of `opencloud-registration`:

```bash
go test ./e2e/ -v -timeout 60s
```

> **Note:** The tests will automatically be skipped if the stack is not reachable. Ensure the container is healthy using `./occtl status`.

### Custom Test Configuration

By default, the tests use configuration that matches the stock `pl-opencloud-server/.env`. If your setup differs, you can override the target environment using the following variables:

```bash
REGISTRATION_URL=https://register.example.com \
OC_URL=https://cloud.example.com \
ADMIN_TOKEN=your-token \
OC_ADMIN_PASSWORD=your-password \
go test ./e2e/ -v
```

**Default Test Configuration:**

| Variable | Default Value |
|---|---|
| `REGISTRATION_URL` | `https://register.opencloud.test` |
| `OC_URL` | `https://cloud.opencloud.test` |
| `ADMIN_TOKEN` | `localtest` |
| `OC_ADMIN_USER` | `admin` |
| `OC_ADMIN_PASSWORD` | `admin` |

### E2E Test Coverage

The following behaviors are verified by the E2E suite:

| Test | What it verifies |
|---|---|
| `HealthCheck` | `/health` returns `{"status":"ok"}` |
| `RegistrationFormAtRoot` | `GET /` renders the form, not the success page |
| `SuccessPageLinksToOpenCloud` | Success page links to the OC sign-in page (`/signin/v1/identifier`), not the registration app |
| `OpenRegistration_CreatesUser` | All submitted fields (display name, username, email) arrive correctly in OpenCloud |
| `OpenRegistration_UserCanLogin` | The registered user can authenticate against OpenCloud via WebDAV |
| `DuplicateEmail_Rejected` | Second registration with same email shows an error |
| `DuplicateUsername_Rejected` | Second registration with same username shows an error |
| `AdminAuth_Required` | `/admin` without a valid token returns 401 |
| `FieldValidation_Rejected` | Blank fields, bad email, short/mismatched passwords are all rejected |
| `HtmxErrorResponse_IsFragment` | Error responses are a bare `#form-container` fragment, not a nested full page |
| `FullFlow_FormToLogin` | End-to-end: GET form → POST → success page → sign-in link → OC login page → WebDAV auth |
| `AdminPanel_ShowsRegistration` | Registered users appear in the admin panel with correct username and email |
