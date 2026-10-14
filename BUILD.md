# Building BuildFlow

BuildFlow is a single Go program. It uses the standard library only, so there
is nothing to download and no module cache to warm up beyond the toolchain
itself.

## Requirements

| Requirement | Notes |
| --- | --- |
| Windows 11 | Windows 10 build 17763 or newer should also work. |
| Go 1.24 or newer to *build* | Only needed on the build machine. The `.exe` has no runtime dependencies. |
| PowerShell 5.1 or newer | For `build.ps1` and `scripts\verify.ps1`. |

End users need none of this. They run `BuildFlow.exe`.

## One command

```powershell
cd C:\BuildFlow
powershell -ExecutionPolicy Bypass -File build.ps1
```

`build.ps1` finds Go (on `PATH`, or in the usual install locations), runs
`gofmt`, `go vet` and the unit tests, and then writes two executables:

| File | Purpose |
| --- | --- |
| `dist\BuildFlow.exe` | The application. Built with `-H windowsgui -s -w`, so it has no console window and a small binary. This is the file you install. |
| `dist\BuildFlow-debug.exe` | A console build that echoes logs. Use it for troubleshooting, and it is what `scripts\verify.ps1` drives. |

Useful switches:

```powershell
powershell -File build.ps1 -SkipTests            # skip the unit tests
powershell -File build.ps1 -OutputDir C:\Temp\bf # build somewhere else
```

## Building by hand

If you would rather not use the script, this is exactly what it runs:

```powershell
cd C:\BuildFlow
$env:PATH = "C:\Program Files\Go\bin;$env:PATH"   # if Go is not already on PATH

go build -trimpath -ldflags "-s -w -H windowsgui" -o dist\BuildFlow.exe .
go build -trimpath                                 -o dist\BuildFlow-debug.exe .
```

Flags explained:

- `-trimpath` strips local folder paths from the binary, so builds are
  reproducible and the binary leaks no directory names.
- `-s -w` drop the symbol table and DWARF data.
- `-H windowsgui` builds a GUI subsystem binary, which is why the normal
  `BuildFlow.exe` shows no console window. All output goes to the log file
  instead.

## Cross-compiling from another machine

The target is always Windows, so from any host with Go installed:

```powershell
$env:GOOS   = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -trimpath -ldflags "-s -w -H windowsgui" -o dist\BuildFlow.exe .
```

`CGO_ENABLED=0` is the default for cross builds and is safe here: BuildFlow
calls Windows directly through `internal/winutil`, not through cgo.

## Running the verification suite

`scripts\verify.ps1` starts a private BuildFlow instance in a temporary folder
(so your real workspace and `%LOCALAPPDATA%\BuildFlow` are never touched) and
exercises the running application over HTTP:

```powershell
powershell -ExecutionPolicy Bypass -File scripts\verify.ps1
```

It checks loopback-only binding, token and origin rejection, project
create/rename/switch, document saves and byte-for-byte reloads, external-edit
conflict detection, global versus local prompt templates, prompt assembly and
its prerequisites, prompt snapshots, `P01`/`P02` package detection and import,
N+1 package creation, per-package Step 5 records, path traversal rejection,
persistence across a restart, single-instance enforcement, the sign-in setting
and a clean quit. It currently runs 182 checks.

Point it at an instance you are already running instead of starting a new one:

```powershell
powershell -File scripts\verify.ps1 -NoStart `
    -BaseUrl http://127.0.0.1:8765 `
    -TokenFile "$env:LOCALAPPDATA\BuildFlow\api-token.txt" `
    -Workspace "D:\Obsidian\BuildFlow"
```

## Code signing

`build.ps1` signs both executables with `scripts\sign.ps1` unless you pass
`-NoSign`. This is a **self-signed development certificate**: it is created in
`Cert:\CurrentUser\My` and also added to `Cert:\CurrentUser\Root`, which is what
lets Windows 11 Smart App Control launch a freshly built binary on the machine
that built it.

```powershell
powershell -ExecutionPolicy Bypass -File scripts\sign.ps1
```

Two things to be clear about:

- The certificate is trusted **only on this machine**. Handing `dist\BuildFlow.exe`
  to somebody else gives them an executable that Windows will warn about or block.
  For distribution, buy a real code-signing certificate and sign with it instead
  (`signtool sign /fd sha256 /tr http://timestamp.digicert.com /td sha256 /f
  cert.pfx /p <password> dist\BuildFlow.exe`).
- Adding to `Cert:\CurrentUser\Root` is a trust decision for your own user
  account only. No administrator rights and no machine-wide change are involved,
  and removing the certificate from both stores reverses it completely.

## Repository layout

```text
main.go                      process wiring: flags, mutex, tray, shutdown
internal/appdata/            settings, log folder and token file
internal/logging/            levelled logging with rotation
internal/server/             HTTP server, security chain, REST handlers
internal/winutil/            Windows shell calls (tray, mutex, autostart, picker)
internal/workspace/          every file operation, path resolution, templates
internal/workspace/starter/  the six built-in starter prompt templates
web/                         the embedded HTML, CSS and JavaScript workbench
scripts/verify.ps1           end-to-end verification
scripts/sign.ps1             self-signed code signing for local builds
```

`web/` is its own Go package because `go:embed` cannot reach into a sibling
package directory. `internal/workspace/starter/` is a second one for the same
reason.

## Versioning

`internal/server/server.go` holds `const Version = "1.0.0"`. It is reported by
`GET /api/health`, shown in the workbench footer and printed by
`BuildFlow.exe --version`. Bump it when you cut a release.
