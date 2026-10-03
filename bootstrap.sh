#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "Usage: ./bootstrap.sh <go-module-path>"
  echo "Example: ./bootstrap.sh github.com/your-org/your-project"
  exit 1
fi

MODULE_PATH="$1"

if [[ -f go.mod ]]; then
  echo "go.mod already exists; refusing to replace it."
  exit 1
fi

command -v go >/dev/null 2>&1 || { echo "Go is not installed or not in PATH."; exit 1; }

GO_VERSION="$(go version | awk '{print $3}' | sed 's/^go//')"
GO_MINOR="$(printf '%s' "$GO_VERSION" | sed -E 's/^1\.([0-9]+).*$/\1/')"
if [[ "$GO_VERSION" == 1.* && "$GO_MINOR" =~ ^[0-9]+$ && "$GO_MINOR" -lt 25 ]]; then
  echo "Gin currently requires Go 1.25 or newer. Detected Go $GO_VERSION."
  exit 1
fi

go mod init "$MODULE_PATH"
for f in cmd/api/main.go internal/server/server.go; do
  sed -i.bak "s#github.com/your-org/your-project#$MODULE_PATH#g" "$f"
  rm -f "$f.bak"
done
go get github.com/gin-gonic/gin
go mod tidy

gofmt -w cmd/api/main.go internal/server/server.go internal/config/config.go

go test ./...
go vet ./...

echo
echo "Bootstrap complete for module: $MODULE_PATH"
echo "Run: go run ./cmd/api"
