
# LazyAPI

> OpenAPI-driven API exploration, testing, and automation from your terminal.

LazyAPI is a Terminal User Interface (TUI) that allows developers to explore, execute, and validate API operations directly from OpenAPI specifications.

Instead of manually creating collections, requests, and environments, LazyAPI uses your OpenAPI document as the source of truth, providing a faster and more reliable workflow for API development and testing.

---

## Features

### Available

* OpenAPI-first workflow
* Interactive Terminal UI (TUI)
* Endpoint discovery
* Request execution
* Parameter inspection
* Response visualization
* Lightweight and portable

### Planned

* Automated test execution
* Contract validation
* CI/CD integration
* Scenario-based testing
* Environment management
* Test reports

---

## Why LazyAPI?

Most API tools require you to manually recreate information that already exists in your OpenAPI specification.

LazyAPI focuses on using the specification directly:

```text
OpenAPI
    │
    ▼
 LazyAPI
    │
    ├── Explore Operations
    ├── Execute Requests
    ├── Validate Responses
    └── Run Automated Tests
```

This approach helps reduce duplication, configuration drift, and maintenance overhead.

---

## Installation

### Build from Source

```bash
git clone https://github.com/<your-org>/lazyapi.git

cd lazyapi

go build -o lazyapi ./cmd/tui
```

---

## Project Structure

```text
.
├── cmd
│   └── tui
├── examples
└── internal
```

### cmd/tui

Contains the application entrypoint.

### examples

Example OpenAPI specifications and sample projects.

### internal

Internal application packages and business logic.

---

## Running the TUI

```bash
go run ./cmd/tui/main.go
```

Or after building:

```bash
./lazyapi
```

---

## Example Use Cases

### Explore an API

Load an OpenAPI document and browse available operations.

### Test Endpoints

Execute requests directly from the specification without manually building requests.

### Validate API Behavior

Inspect status codes, headers, and payloads against the expected contract.

### Automate Verification

Use future test commands to execute repeatable validation suites from OpenAPI definitions.

---

## Roadmap

* [x] OpenAPI 3.x support
* [ ] Request execution
* [ ] Authentication support

  * [ ] API Key
  * [ ] Bearer Token
  * [ ] OAuth2
* [ ] Automated testing command
* [ ] Response assertions
* [ ] Test reports

---

## Contributing

Contributions are welcome.

Please open an issue before starting large changes so implementation details can be discussed.

For pull requests:

* Ensure the project builds successfully.
* Ensure all CI checks pass.
* Add tests when applicable.
* Document user-facing changes.

---

## License

GPLv3.
