# Build BuildFlow for Windows.
#
#   powershell -ExecutionPolicy Bypass -File build.ps1
#
# Produces two executables in dist\:
#   BuildFlow.exe         the normal application (no console window, stripped)
#   BuildFlow-debug.exe   a console build for troubleshooting and for
#                         scripts\verify.ps1
#
# Nothing outside dist\ is modified and the result has no external
# dependencies: no Go runtime, no Node, no database, no web server.

param(
    [string]$OutputDir = "",
    [switch]$SkipTests,
    [switch]$NoSign
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $root

if (-not $OutputDir) { $OutputDir = Join-Path $root "dist" }
New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null

# --- locate Go --------------------------------------------------------------
$go = $null
$fromPath = Get-Command go -ErrorAction SilentlyContinue
if ($fromPath) {
    $go = $fromPath.Source
} else {
    foreach ($candidate in @("$env:ProgramFiles\Go\bin\go.exe", "${env:ProgramFiles(x86)}\Go\bin\go.exe", "$env:LOCALAPPDATA\Programs\Go\bin\go.exe")) {
        if ($candidate -and (Test-Path $candidate)) { $go = $candidate; break }
    }
}
if (-not $go) {
    Write-Host "Go was not found." -ForegroundColor Red
    Write-Host "Install Go 1.24 or newer from https://go.dev/dl/ and either add it to PATH"
    Write-Host "or re-run this script from a terminal where 'go' resolves."
    exit 2
}
Write-Host ("Using " + $go) -ForegroundColor DarkGray
& $go version

$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"

# --- checks -----------------------------------------------------------------
Write-Host ""
Write-Host "Checking formatting..." -ForegroundColor Cyan
$fmt = & $go fmt ./...
if ($fmt) {
    Write-Host ("  reformatted: " + ($fmt -join ", ")) -ForegroundColor Yellow
    Write-Host "  (the files above were rewritten by gofmt; review and commit them)"
} else {
    Write-Host "  all files already formatted" -ForegroundColor DarkGray
}

Write-Host "Running go vet..." -ForegroundColor Cyan
& $go vet ./...
if ($LASTEXITCODE -ne 0) { Write-Host "go vet reported problems." -ForegroundColor Red; exit 1 }

if (-not $SkipTests) {
    Write-Host "Running unit tests..." -ForegroundColor Cyan
    & $go test ./...
    if ($LASTEXITCODE -ne 0) { Write-Host "tests failed." -ForegroundColor Red; exit 1 }
}

# --- build ------------------------------------------------------------------
$release = Join-Path $OutputDir "BuildFlow.exe"
$debug = Join-Path $OutputDir "BuildFlow-debug.exe"

Write-Host ""
Write-Host "Building $release" -ForegroundColor Cyan
& $go build -trimpath -ldflags "-s -w -H windowsgui" -o $release .
if ($LASTEXITCODE -ne 0) { Write-Host "release build failed." -ForegroundColor Red; exit 1 }

Write-Host "Building $debug" -ForegroundColor Cyan
& $go build -trimpath -o $debug .
if ($LASTEXITCODE -ne 0) { Write-Host "debug build failed." -ForegroundColor Red; exit 1 }

# --- code signing -----------------------------------------------------------
# Windows 11 Smart App Control refuses to launch a freshly built unsigned .exe
# ("An Application Control policy has blocked this file"). Signing with a
# certificate that the current user trusts fixes that without administrator
# rights. This affects every self-built desktop program, not just BuildFlow.
# Pass -NoSign to skip, or see BUILD.md for real (purchased) certificates.
if ($NoSign) {
    Write-Host ""
    Write-Host "Skipping signing (-NoSign)." -ForegroundColor Yellow
    Write-Host "Windows may refuse to launch an unsigned build: Smart App Control."
} else {
    Write-Host ""
    Write-Host "Signing the executables..." -ForegroundColor Cyan
    & (Join-Path $PSScriptRoot "scripts\sign.ps1") -Path @($release, $debug)
    if ($LASTEXITCODE -ne 0) { Write-Host "signing failed; the build is unusable on Smart App Control." -ForegroundColor Red; exit 1 }
}

# --- report -----------------------------------------------------------------
Write-Host ""
Write-Host "Done." -ForegroundColor Green
Get-ChildItem $OutputDir -Filter *.exe | ForEach-Object {
    $hash = (Get-FileHash $_.FullName -Algorithm SHA256).Hash.Substring(0, 16)
    $sig = (Get-AuthenticodeSignature $_.FullName).Status
    Write-Host ("  {0,-22} {1,10:N0} bytes  sha256:{2}...  signed:{3}" -f $_.Name, $_.Length, $hash, $sig)
}
Write-Host ""
Write-Host "Start it with:  $release"
Write-Host "Open it with:  http://127.0.0.1:8765"
Write-Host "Verify it with: powershell -ExecutionPolicy Bypass -File scripts\verify.ps1"
