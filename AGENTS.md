# AGENTS.md - Project Guidelines

## Build, Test, and Lint Commands
* **Run Tests:** `go test -v ./...`
* **Run Specific Test:** `go test -v ./internal/auth -run TestTokenExchange`
* **Build Binary:** `make`
* **Release:** `make release-[major|minor|patch]` depending on whether we are increasing the major, minor or revision level in semantic versioning (e.g. v1.2.3)
* **Other arguments:** run `make help` to get a list of available command line switches.

## Spec-Driven Development (SDD) Workflow
1. **Never code blind:** All new features or non-trivial bug fixes must have an approved specification file in the `.specs/` directory before writing implementation code.
2. **Plan first:** Always use Claude's `/plan` mode to review requirements against the codebase before editing files.
3. **Threat Model:** Whenever developing a complex solution, check that there is a Threat Model in the spec, ask for one otherwise, and take it into account when designing the solution itself and writing the code.
4. **Test-Driven Implementation:** Write unit or integration tests matching the acceptance criteria in the spec *before* writing the implementation code.
5. **Compliance Audit:** Before finishing a task, verify that the implementation matches the API contracts, data models, and edge cases defined in the active spec.
6. **No Secrets in Commits:** Before committing anything, check with all available local tools (e.g. gitleaks, trufflehog) that no secrets are in the sources being committed. 
7. **No vulnerable dependencies in Commits:** Before committing, run govulncheck and check that there are no vulnerable dependencies in `go.mod`.
8. **No unneeded backward compatibility:** Backward compatibility is not necessary (and should be avoided if it complicates code, introduces warts or hinders comprehension of the design and the code) until a product's major version (e.g. v1.0.0) is released, and even then it may be worth evaluating costs and benefits of keeping the product backward-compatible versus bumping the product's major version. Always ask for the user's choice unless the decision is trivial.
9. **Fuzzy Testing and Benchmarking:** Propose fuzzy testing and benchmarking when reasonable.
10. **Lab Testing:** Whenever applicable, propose to set up a lab environment to cover complex integration scenarios; don't shy away from proposing more coverage scenarios, including lengthy stress tests and chaos monkey to check for correctness under random conditions (e.g. race conditions, malfunctions), whenever reasonable.
11. **Up-to-date Documentation:** Always keep documentation up to date and aligned with the code and the product behaviour.
12. **Publish Documentation:** Always produce and attach to the release a single Manual, in PDF format from Markdown sources, that covers setup, operation, architecture and design, and the library (one document, not one per audience); when diagrams and tables do not fit a page in portrait mode, use the landscape orientation for that page.
13. **Sane Commits:** run `go test ./...` before every commit and check that everything is still sane.
14. **Overnight Runs:** Any run that takes hours (soak and chaos runs, full lab rebuilds, release-run campaigns) is written up as a plan and run in an overnight, unattended session, never started in the middle of a working day: propose it, say how long it takes, and report the outcome in the morning.

## Coding Style & Architecture Standards
* **Language/Framework:** Go (Golang), standard library routers; propose additional libraries where applicable.
* **Error Handling:** Never swallow errors. Wrap errors using `fmt.Errorf("context: %w", err)` and return appropriate HTTP status codes mapped cleanly at the transport layer. Log errors where they occur using `log/slog`.
* **Concurrency:** Use explicit context propagation (`context.Context`) across all database calls, network requests, and background worker threads.
* **Logging:** Use structured logging (with `log/slog`) with contextual keys (`tenant_id`, `request_id`). Never log raw secrets, tokens, or PII.

## Project Structure Overview
* `cmd/` - Application entrypoints and main binaries; for details on how the application command lines are structured, see the paragraph on command line arguments
* `internal/` - Private application and library code.
* `pkg/` - Public library code (e.g. the build metadata injected at link time).
* `.specs/` - Feature design documents and architectural specifications.

## Command line
Each application supports command line switches.
The command line arguments parsing is based on `github.com/jessevdk/go-flags`. Where applicable, the command line follows the simple convention:
```bash
$> my-application <object> <verb>
```
where the <object> is the type of resource to act upon and <verb> is the action. This is similar to how docker works (e.g. docker image list). 
All applications should provide a version switch, with a one-liner compact mode and a verbose mode providing more detailed information at least about the application, its version, license, author, build and packaging environments.