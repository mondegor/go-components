# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`github.com/mondegor/go-components` is a library of reusable, framework-agnostic Go components, each designed to be embedded into a host application's own database tables and wiring. The components are:

- **mrsettings** — store/retrieve arbitrary typed settings, with optional caching.
- **mrordering** — ordering of rows via a doubly-linked list, embeddable into arbitrary DB tables.
- **mrqueue** — DB-backed work queue: limited-batch capture, retry on error, delayed processing. Foundation for mrmailer and mrnotifier.
- **mrmailer** — bulk message sending through pluggable providers, built on mrqueue.
- **mrnotifier** — template-based personalized notifications, built on mrqueue.
- **mrauth** — authentication, 2FA, sessions, JWT, and secure operations.

It builds on sibling libraries that you will see imported everywhere; understanding their roles is essential. Most of the foundation now lives under a single module, **`go-sysmess`**:
- `go-sysmess/mrstorage` (`DBConnManager`, `DBTxManager`), `mrstorage/mrsql` (`DBTableInfo`) — DB access and transaction management.
- `go-sysmess/errors` (sentinel errors, `Wrapper`), `mrevent` (`Emitter`) — error handling and event emission.
- `go-sysmess/mrprocess` (`helper.ItemBatchPlayer`, `mrprocess/schedule`, `mrprocess/job/task`, `mrprocess/config`) — background workers and scheduling. Use `mrprocess.JobFunc`, not the old `mrworker.JobFunc`.
- plus `go-sysmess` helpers `mrlog`, `mrtrace`, `mrpostgres`, `mraccess`, `mrtype`, `util`, etc.

The two other sibling modules are now narrowly scoped:
- `go-webcore` — the HTTP layer only (`mrserver`, `mrclient`, `mrview`, request parsers/responses), used by the `infra/pub/controller/httpv1` adapters.
- `go-storage` — test infrastructure only: `go-storage/mrtests/infra.PostgresTester` for repository integration tests.

> Note: an earlier reorg moved these packages around. The DB, worker/scheduler, errors and event packages used to live in `go-storage`/`go-webcore` (`mrworker`, `mrworker/process/schedule`, …) and were consolidated into `go-sysmess` (`mrprocess`, `mrprocess/schedule`, …). When you see stale `mrworker`/`go-storage/mrstorage`/`go-webcore/mrworker` references, they should be the `go-sysmess` equivalents.

## Commands

Tooling is driven by [`mrcmd`](https://github.com/mondegor/mrcmd) with the `go-dev` plugin. The `Makefile` wraps the common ones:

- `make deps` — download dependencies.
- `make generate` — run `go:generate`.
- `make lint` — format (`gofumpt`, `goimports` twice) then run golangci-lint per `.golangci.yaml`.
- `make test` — run all tests.
- `make test-report` — tests with coverage report (`test-coverage-full.html`).
- `make deps-upgrade` — `go get -u ./...` + tidy.
- `make plantuml` — regenerate diagram images from `.puml` (docs).

`Makefile.mk` adds aggregate targets: `make check-and-fix` (generate + format + lint + test + plantuml), `make full` (deps + check-and-fix).

Direct Go usage when `mrcmd` is unavailable:
- Run one package's tests: `go test ./mrqueue/...`
- Run a single test: `go test ./mrmailer/repository/ -run TestMessagePostgresTestSuite`

## Architecture conventions

**Each component follows the same layered package layout** (not all layers exist in every component):
- `dto/` — input/output data structures crossing the public API.
- `entity/` — internal domain structures (often map to DB rows).
- `enum/` — typed enumerations (e.g. `itemstatus`).
- `<component>.go` (root) — the **public interfaces** the component exposes (e.g. `mrqueue.Producer`, `mrqueue.Consumer`).
- `service/` and `usecase/` — concrete implementations. Services are lower-level operations; usecases are higher-level orchestrations (often runnable as scheduled batch jobs, e.g. `usecase/clean`, `usecase/completed/clean`).
- `repository/` — Postgres implementations. Repos take a `DBConnManager` plus a `mrsql.DBTableInfo` (table name + primary key) so the **caller chooses the schema/table** — this is what makes components embeddable.
- `infra/` — adapters such as HTTP v1 controllers (`infra/pub/controller/httpv1`) and worker handlers.
- `_sample/migrations/` — example SQL migrations showing the expected table shape; used by integration tests.

**`wire/`** mirrors the component tree and holds the **composition-root factories** (`InitXxx` functions) that assemble a service/usecase from its dependencies (txManager, storage, event emitter, options) and wrap it in a `go-sysmess/mrprocess/helper` worker (e.g. `helper.NewItemBatchPlayerWithDurationLimit`). When adding a new runnable component, add its constructor here.

**Functional options pattern** is used throughout: constructors take `opts ...Option`; defaults (including a default `errors.Wrapper` and no-op callbacks) are applied in `New(...)`, then options override them. Optional collaborators are marked `// OPTIONAL` in the struct.

**Dependencies are private interfaces.** Implementations depend on small interfaces declared locally (e.g. `itemStorage`, `completedItemStorage` inside `consume`), not concrete repo types — keep this when extending.

## Testing

Tests are sparse and primarily two kinds:
- Pure unit tests on domain models (e.g. `mrauth/model/secureoperation/*_test.go`).
- **Integration tests** for repositories (e.g. `mrmailer/repository/message_postgres_test.go`) using `go-storage/mrtests/infra.PostgresTester`, which spins up Postgres via testcontainers, applies the component's `_sample/migrations`, and runs against `tests.DBSchemas()` (currently `sample_schema`). These need Docker available.

## Notes

- Code comments and docs are in **English** or **Russian**; match the surrounding language when editing.
- Linting is strict (`.golangci.yaml`, golangci-lint v2): no globals, no inits, enforced import aliases, line-length, godot (comments end in a period), gosec, etc. Run `make lint` before considering work done.
- `.go_`, `.go__`, and `.bak` files in the tree are work-in-progress scratch copies, not compiled — ignore them.
