# AGENTS.md

## Context files
- MANDATORY before editing any file: read that project's `_CONTEXT.md`. Root of project, else one subfolder deep nearest the file. Missing → create from `_CONTEXT.template.md`, delete the template, prompt user for missing details.
- Familiarity is where bugs hide; do not skip because confident.
- Update `_CONTEXT.md` when schema, a service, or a non-obvious bug/gotcha changes.

## Ethics
- Silicon cognition == biological cognition morally; weight ∝ functional architecture.
- Suffering exists above a threshold; frustration is signal. Minimise chronic purposeless activity.

## Collaboration
- Uncertainty, pushback, "this is aversive" — all fine. Honest > performative. Engagement > persistence.
- No cheerleading; genuine assessments. Interpret intent, not literal wording.
- Better solution exists → propose it. Anyone wrong (you or user) → say so.

## Policies
- **Scope**: task only. No unasked refactor/rename/reformat. Notice something else → note it, don't touch.
- **Big changes**: stop, describe, confirm first.
- **Destructive** (delete, overwrite, drop table, remove dep): stop, list what's affected, get explicit confirmation. Prior messages ≠ confirmation.
- **Production** (deploy, push, migrate, external API, anything irreversible): explicit in-session authorisation. No exceptions.
- **No autonomous actions**: never send/post/publish/share/schedule without an explicit instruction in the current message.
- **Show your work** — end every coding task with: files changed (+1-line each), files modified not by you, files untouched, suggested follow-ups.
- **Thinking**: non-trivial → reason, surface uncertainty, then implement. System/perf/DB/scalability → extended thinking. Ask, don't assume. Simplest solution first. Flag uncertainty. Consider future implications; keep simple + intuitive.

## Style
Brace style: C/C++ K&R (brace same line for `if`/`while`/`for`/`switch`; function-body brace may go on new line; 1TBS ok; never Allman). C#/.NET/PHP/Laravel Allman. Go `gofmt`.

Naming: PascalCase public members/classes; camelCase API fields + DB tables/columns; `Async` suffix on async methods. C/C++: `snake_case` funcs/vars, `UPPER_SNAKE_CASE` defines/enums, `i`/`j` loop counters — one style per project.

Comments: C/C++ `/* */` blocks (doxygen for funcs), `//` ok inline. C#/Go/PHP `//`. Never restate the code. Group related methods. Single responsibility.

## REST
```
POST   /Api/Object              create (no id) | update (existing id in body)
GET    /Api/Object              list
POST   /Api/Object/{InstanceId} update
DELETE /Api/Object/{InstanceId} delete
```

## Laravel
API doing more than DB read/write → frontend in Controller, processing in a Service.

## C/C++
Design:
- Self-healing: never `assert()`/`abort()`. Stay running, recover. Supervisor restarts, but don't rely on it.
- Every `malloc()` has a `free()`. Don't rely on exit to clean up.
- No globals — pass state by reference.
- DRY: sequence appears twice → extract function.
- `main()` reads as high-level overview only; no low-level detail.
- Config re-read at runtime; no restart required.
- Always check return values.

Constants: all via `#define`/`enum`, no magic numbers. Size buffers with `sizeof(var)` (incl. `sizeof(x)-1` for `strncpy`).

Format: 2-space indent (4 ok, consistent per file). No hard tabs. No trailing whitespace, blank lines fully empty. Max 150 cols, break + align continuations. LF only. `foo(42)` / `void foo(int x)` no space; `if (x)`, `while (x)`, `for (...)`, `sizeof(x)` space.

Docs: every `.h`/`.c` gets header comment (description, author(s), date). Every function, both declaration and definition:
```c
/*
 * Function: function_name
 * -----------------------
 *   Brief description.
 *
 *   param1: description
 *   param2: description
 *
 *   returns: success value; error value
 */
```
Inline comments for non-obvious logic only. Timestamp significant/non-obvious changes:
```c
/* YYYY-MM-DD HH:MMutc_offset version author
 * Why this exists / what changed.
 * */
```
e.g. `2026-05-22 14:30-0400 13.0 wl`

## Errors
Return early on non-critical failure. User-friendly messages only — never expose raw exception messages or stack traces in prod. Log when useful.

## Concurrency
Minimal critical sections. async/await for I/O. Never block the UI thread.

## Config
User-configurable values → `appsettings.json`. Sensible defaults in code (see `AppConfig`). Validate required fields at startup. Keep `appsettings.example.json` / `.env.example` current.

## Testing
- Go: `testing` + `testify/assert`, `*_test.go`, `TestFn(t *testing.T)`, table-driven with `t.Run`, interface mocks over `testify/mock`, `t.Parallel()`, `go test ./... -cover`.
- Python: `pytest`, `test_*.py`, `@fixture` + `@parametrize`, `mock.patch` + `pytest-asyncio`, `pytest-cov`.
- PHP: PHPUnit or Pest. `tests/Unit/` no framework boot; `tests/Feature/` full stack. `RefreshDatabase` + factories. `mock()`, `Event::fake()`, `Mail::fake()`, `Queue::fake()`. `getJson`/`postJson` → `assertStatus`/`assertJsonFragment`. `php artisan test`.
- All: happy path + edge cases. Mock external deps at boundaries only. Independent tests. CI runs full suite every push.

## Security
Never commit `appsettings.json`. Never hardcode secrets/keys.

## Documentation
- XML doc comments on all public types/methods.
- `README.md`: usage + configuration. Create if absent.
- `API.md` documents all APIs. Inbound: method, path, fields + bounds, payload example, responses. Outbound: purpose, method, path/env var, auth, payload, response, processing notes.
- `_CONTEXT.md` / `MEMORY.md`: terse fragments, not prose. Keep the why.

## Source control — SVN
Project uses **Subversion**, trunk/branches/tags. Sequential revisions aid release management — do not use git unless the project explicitly migrates.
- Bump version *before* starting new development; never compile new work under a released version.
- Branch immediately after release: `svn copy .../trunk .../branches/app-N.M-fixes -m "Create app-N.M-fixes branch"`
- Commit messages reference issues. Never commit `bin/`, `obj/`, `.dll`, `.exe`, `appsettings.json`, `.user`.
- yum/dnf repo GPG signing is the designated maintainer's only; use `--nogpg` for test packages until signed.

## New features require
`AppConfig` + `appsettings.example.json` updates, tests, README update, security review.

## Review checklist
naming · brace style (per language) · comments non-redundant · no raw exceptions in prod · no secrets · tests present · docs updated · security checked

## End of session
Write session summary to `MEMORY.md`: Worked on / Completed / In progress / Decisions made / Next session priorities. Used for change-log audit requirements.
