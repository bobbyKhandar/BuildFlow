#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$repo_root"

if ! command -v go >/dev/null 2>&1; then
  echo "Go 1.24 or newer is required to build BuildFlow." >&2
  exit 1
fi

find . -type f -name '*.go' -not -path './dist/*' -print0 | xargs -0 gofmt -w
go vet ./...
go test ./...

stage="$(mktemp -d)"
trap 'rm -rf "$stage"' EXIT

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o "$stage/BuildFlow" .
# Keep the Windows implementation covered from Linux without changing the
# existing PowerShell release and signing workflow.
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -o "$stage/BuildFlow.exe" .
cp README-linux.md "$stage/README-linux.md"

mkdir -p dist
tar -C "$stage" -czf dist/BuildFlow-linux-amd64.tar.gz BuildFlow README-linux.md
echo "Created $repo_root/dist/BuildFlow-linux-amd64.tar.gz"
