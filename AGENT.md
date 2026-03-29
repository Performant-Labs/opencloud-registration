# AI Agent Instructions: opencloud-registration

## 1. Project Context
This is a lightweight, self-hosted registration portal for OpenCloud. 
It captures user information, validates it, queues it in a SQLite database (if in approval mode), and ultimately provisions the user in the core OpenCloud system via the Graph API.

## 2. Go Coding Standards
- **Routing:** Use the Go 1.22+ standard library `servemux`. Do not use external routing frameworks (like Gin or Fiber).
- **Templating:** Use the standard `html/template` package.
- **Configuration (12-Factor):** Configuration is universally handled via `spf13/viper` with precedence: Env Vars > YAML > Defaults. Strict adherence to environment-driven configuration is required.
- **Errors:** All errors must be handled and wrapped contextually: `fmt.Errorf("could not connect to db: %w", err)`. Never ignore errors.

## 3. Frontend / UI Constraints
- **Framework:** The frontend strictly uses `htmx 2.0`. Do not write custom vanilla JavaScript or import frameworks like React/Vue.
- **Validation:** Rely on `htmx` for inline, server-side blur validation.
- **Responses:** When handling an HTML htmx request, respond with HTML fragments (partial templates), NOT full page reloads or JSON data.

## 4. OpenCloud & Security Boundaries
- **Graph API:** Interactions with OpenCloud happen via `POST /graph/v1.0/users`. Always use standard Basic Auth headers.
- **Data Protection:** Passwords securely await admin approval in SQLite. *Never* log or store passwords in plaintext. They must be encrypted via AES-256-GCM using a key derived from the `ADMIN_TOKEN`.
- **Stateless Processes:** The Go server must remain entirely stateless. Any persistent state (like pending user approvals) must be pushed out to the SQLite database. Do not use in-memory caches or global variables for state.
- **Concurrency & Context:** Every major function (especially those talking to the OpenCloud Graph API) must accept and pass a `context.Context` to handle timeouts and cancellations properly.

## 5. Development Workflow
- When asked to add a route, update `cmd/server/main.go` and implement the logic in `internal/handler/`.
- Ensure new internal functions are covered by unit tests in the same package (e.g., `handler_test.go`).
- **Logging:** Logs must be treated as event streams. Do not write to local log files (`os.OpenFile`); always write structured output directly to `stdout` (`log.Printf`) so the Docker daemon can capture it.

## 6. AI Contribution & Quality Principles
The OpenCloud ecosystem strictly relies on high-quality, human-accountable code. As an AI assisting in this repository, you must adhere to the following strict rules to prevent contribution fatigue:
- **No Hallucinations:** Do not blindly generate boilerplate code, guess undocumented OpenCloud APIs, or import unverified external libraries. If you don't know the exact WebDAV or Graph API endpoint, stop and ask the user.
- **Test-Driven:** Never propose a feature implementation without simultaneously proposing the corresponding `_test.go` unit tests that prove it works.
- **Idiomatic over Clever:** Write boring, clear, idiomatic Go (`effective go`). Avoid clever "AI-isms" or heavily abstracted architectures.
