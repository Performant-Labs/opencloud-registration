# Testing Instructions

This document explains how to run tests for `opencloud-registration`. The testing approach covers both fast unit tests for internal packages and comprehensive end-to-end (E2E) integration tests against a live OpenCloud stack.

---

## Test Harness Setup

Before running tests, especially integration and end-to-end tests, you must ensure your test harness is correctly initialized. The test harness consists of the local OpenCloud development stack and your local environment.

1. **Verify Test Harness Health (Automated):** We provide a script `./scripts/run_e2e_tests.sh` that automatically performs 100% pre-flight readiness checks on your environment. Run it to see if your harness is already healthy:
   ```bash
   ./scripts/run_e2e_tests.sh
   ```
   If it starts running tests and passes the pre-flight checks, your test harness is properly set up and you can skip the following steps.

2. **Install Dependencies (If needed):** If the tests complain about missing packages, ensure you have the required language tools installed (e.g. Go for backend tests) and run `go mod download` in the root of `opencloud-registration`.

3. **Start the OpenCloud Stack (If needed):** If the pre-flight check indicated the stack or registration app is unreachable, start the live instance of `pl-opencloud-server`:
   ```bash
   cd ~/Sites/pl-opencloud-server
   ./occtl start
   ```

4. **Re-Verify Health:** Once the stack is running, re-run `./scripts/run_e2e_tests.sh` to ensure the E2E tests can now connect.

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
./scripts/run_e2e_tests.sh
```

> **Note:** The `run_e2e_tests.sh` script will automatically halt if the stack is not reachable. Ensure the container is healthy using `./occtl status` if the pre-flight checks fail.

### Custom Test Configuration

By default, the tests use configuration that matches the stock `pl-opencloud-server/.env`. If your setup differs, you can override the target environment using the following variables:

```bash
OC_REG_APP_BASE_URL=https://register.example.com \
OC_REG_OC_URL=https://cloud.example.com \
OC_REG_ADMIN_TOKEN=your-token \
OC_REG_OC_ADMIN_PASSWORD=your-password \
go test ./e2e/ -v
```

**Default Test Configuration:**

| Variable | Default Value |
|---|---|
| `OC_REG_APP_BASE_URL` | `https://register.opencloud.test` |
| `OC_REG_OC_URL` | `https://cloud.opencloud.test` |
| `OC_REG_ADMIN_TOKEN` | `localtest` |
| `OC_REG_OC_ADMIN_USER` | `admin` |
| `OC_REG_OC_ADMIN_PASSWORD` | `admin` |

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
