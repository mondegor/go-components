---
name: go-style-guide
description: Code style guide for Go library modules following this company style. Use whenever writing, editing, or reviewing Go code in such a repo so the result matches the established conventions and passes the strict golangci-lint config. Based on the Uber Go Style Guide, adapted with grouped type blocks, English or Russian doc comments, Proto/facade patterns, table-driven _test packages.
---

# Go Library Style Guide

Conventions for Go library modules following this company style. Follows the
[Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md) as a base;
the deviations and company-specific rules below take precedence. Everything here is enforced
by `.golangci.yaml` (`golangci-lint` runs strict — `make lint` must pass before done).

## Formatting & imports

- `gofumpt` with `extra-rules: true` + `gofmt` + `goimports` + `gci`. Tabs for indentation.
- Line length ≤ **160** (tab-width 4) — `lll`.
- Import groups in this exact order (`gci`/`goimports` local-prefix), blank line between:
  1. standard library
  2. third-party
  3. the module's own packages (its module-path prefix)
- No file/copyright headers (no `goheader` config). No package doc comments required
  (`staticcheck` `-ST1000`, `revive` `package-comments` disabled).
- Use `any`, never `interface{}` (`revive use-any`). Prefer `strconv` over `fmt.Sprintf`
  for simple conversions (`perfsprint`).

## Declarations — company conventions

- **Always wrap type declarations in a grouped `type ( … )` block — even a single type.**
  This is pervasive (the norm here, unlike Uber which groups only related decls):
  ```go
  type (
      // MessageFormatter - преобразует плейсхолдеры ...
      MessageFormatter struct {
          extractor *PlaceholderExtractor
      }
  )
  ```
- Group related `var`/`const` in blocks. Predefined error catalogs are `var ( … )` blocks
  of `ErrXxx` factory protos.
- No global mutable state (`gochecknoglobals`) — error/sentinel `var`s are the accepted
  exception. No `init()` functions (`gochecknoinits`).

## Comments (English or Russian godot-checked)

- Exported symbols **must** have a doc comment, in **English** or **Russian**, format
  `// Name - descript / описание.` (name, space-dash-space, then text). Doc comments
  end with a period (`godot`). Match the existing terse style.
- **Internal comments** (inside function/method bodies) may start with a lowercase
  letter; when they do, they **must not** end with a period. (`godot`'s scope is
  declarations only, so these aren't linter-enforced — follow the convention manually.)
- Document constructor params with a bulleted list when non-trivial:
  ```go
  // NewMessageFormatter - создаёт MessageFormatter ...
  // Параметры:
  //   - leftDelim, rightDelim - ограничители плейсхолдеров;
  //   - formatter - функция преобразования плейсхолдера.
  ```

## Naming

- Constructors: `NewXxx`. Receivers: short (1 letter), consistent per type
  (`e *protoError`, `w *customErrorWrapper`). `revive receiver-naming` enforces consistency.
- Sentinel errors prefixed `Err`, error *types* suffixed `Error` (`errname`).
  Note: error *codes* are camelCase string literals (e.g. `"errInternalErrorDetected"`).
- Initialisms via `revive var-naming`: `HTTP`, `JSON` (not `Http`/`Json`).
- Import aliases lowercase `^[a-z][a-z0-9]*$`; no redundant aliases.
- `staticcheck -ST1003` is off, so some naming rules are relaxed — still follow Go idiom.

## Errors

- Follow the **Proto pattern** where used: a proto is an immutable factory built once;
  derive concrete instances via `New`/`Wrap`/`WithDetails`. Never mutate a proto after
  construction.
- Root packages act as **facades**: expose new behavior via type aliases
  (`X = subpkg.X`) and `var` function aliases (`NewX = subpkg.NewX`); implement in the
  subpackage. Prefer importing the root facade in consumers.
- Wrap errors crossing external/package boundaries — use `%w`, and compare with
  `errors.As`/`errors.Is` (`errorlint`). Boundary-wrapping itself is a **manual
  convention** — `wrapcheck` is currently disabled in `.golangci.yaml`. Never return
  `nil, nil` (`nilnil`). Don't return `nil` after a non-nil error check (`nilerr`).
- Forbidden imports: `crypto/md5`, `crypto/sha1` (`revive imports-blocklist`, `gosec`).

## Control flow (wsl_v5 / whitespace / nlreturn / revive)

- Blank line before `return`/`break`/`continue` (`nlreturn`); no leading/trailing blank
  lines in blocks (`whitespace`). `wsl_v5` governs statement cuddling — keep related
  statements together, separate unrelated ones with a blank line.
- Early return / guard clauses; avoid `else` after a returning `if`
  (`indent-error-flow`, `early-return`, `superfluous-else`, all `preserveScope`).
  Avoid deep nesting (`nestif`).
- Prefer `make(...)` to init maps/slices (`enforce-map-style`, `enforce-slice-style`).
  Preallocate slices with a capacity hint when length is known (`prealloc`):
  `make([]string, 0, len(x)*2)`.
- No naked returns in non-trivial funcs (`nakedret`, `bare-return`). Functions return
  ≤ 3 results (`function-result-limit`). Watch `gocyclo`/`gocritic`/`unparam`.
- No `fmt.Print*`/debug forbiddens in library code (`forbidigo`) — allowed in
  `examples/` and `_test.go`.

## Tests

- Separate package: `package foo_test` (`testpackage`). Use `testify`
  `require` (fatal) / `assert` (non-fatal) (`testifylint`).
- `t.Parallel()` at the top of every test and subtest (`tparallel`). Test helpers call
  `t.Helper()` (`thelper`).
- Table-driven with a local `type testCase struct`, named cases, `t.Run(tt.name, …)`:
  ```go
  func TestX_Method(t *testing.T) {
      t.Parallel()
      type testCase struct{ name, in, want string }
      tests := []testCase{ {name: "empty", in: "", want: ""} }
      for _, tt := range tests {
          t.Run(tt.name, func(t *testing.T) {
              t.Parallel()
              // ...
          })
      }
  }
  ```
- Relaxed in tests (excluded linters): `dupl`, `gosec`, `forbidigo`, `forcetypeassert`,
  `noctx`, `revive`, `unparam`.
- Benchmarks live in dedicated `*_bench_test.go` files; `go test -bench=. ./pkg/`.

## Before finishing

- Run `make lint` (or `golangci-lint run`) and `make test` (or `go test ./...`).
  The lint config is strict — treat any finding as a blocker.
- Files end with a trailing newline.
