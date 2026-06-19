
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

go build -o lazyapi ./cmd/lazyapi
```

---

## Project Structure

```text
.
├── cmd
│   └── lazyapi
├── examples
└── internal
    ├── app
    ├── cli
    ├── components
    ├── config
    ├── env
    ├── inmath
    ├── model
    └── store
```

### cmd/lazyapi

Root entry point. Dispatches to TUI (default) or CLI (subcommands).

### examples

Example OpenAPI specifications and sample projects.

### internal

Internal application packages and business logic.

---

## Building

```bash
go build -o lazyapi ./cmd/lazyapi
```

## Running

### TUI (default)

```bash
./lazyapi
```

Or with a specific OpenAPI file:

```bash
./lazyapi examples/openapi.yml
```

### CLI commands

#### Create a template

```bash
./lazyapi create file my-api.yml
```

Creates a minimal OpenAPI 3.0 template. Defaults to `openapi.yml` if no name is given. Server URLs can be appended after the filename:

```bash
./lazyapi create file my-api.yml https://api.example.com http://localhost:3000
```

#### Add a request

```bash
./lazyapi add request my-api.yml /users POST
./lazyapi add request my-api.yml /users/{id} GET
```

#### Add a server

```bash
./lazyapi add server my-api.yml https://api.example.com
./lazyapi add server my-api.yml http://localhost:3000
```

#### Remove a request

```bash
./lazyapi remove request my-api.yml GET /users
./lazyapi remove request my-api.yml POST /sales
```

#### Smoke tests (not yet implemented)

```bash
./lazyapi smoke tests my-api.yml
./lazyapi smoke tests my-api.yml --server http://localhost:3000 --env .env
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
* [x] Request execution
* [x] Authentication support
  * [x] API Key
  * [x] Bearer Token
  * [x] OAuth2
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
