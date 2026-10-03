---
name: project-bootstrap
description: Initialize and validate a fresh Go + Gin backend without inventing a module path or unnecessary infrastructure.
---

Use this skill only for initial project setup.

Workflow:
1. Confirm there is no existing go.mod before initialization.
2. Ask for or read the intended module path from the user/project metadata; never invent a real company/repository path.
3. Run `go mod init <module-path>`.
4. Add Gin with `go get github.com/gin-gonic/gin`.
5. Run `go mod tidy`.
6. Create the minimal server entrypoint and health endpoint.
7. Run `gofmt`, `go test ./...`, and `go vet ./...`.
8. Verify imports use the actual module path.

Do not add database, auth, Redis, queues, Docker, or cloud SDKs unless requested.
