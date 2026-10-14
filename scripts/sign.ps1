# Sign BuildFlow executables so Windows will actually launch them.
#
#   powershell -ExecutionPolicy Bypass -File scripts\sign.ps1
#   powershell -ExecutionPolicy Bypass -File scripts\sign.ps1 -Path C:\BuildFlow\dist\BuildFlow.exe
#   powershell -ExecutionPolicy Bypass -File scripts\sign.ps1 -CertificateThumbprint ABC123...
#
# Why this exists
# ---------------
# Windows 11 Smart App Control refuses to run a freshly built, unsigned .exe:
#
#   Program 'BuildFlow.exe' failed to run: An Application Control policy has
#   blocked this file.
#
# The Code Integrity log records it as "did not meet the Enterprise signing
# level requirements". This affects any self-built desktop program, not just
# BuildFlow.
#
# What this script does
# ---------------------
# With no -CertificateThumbprint it creates (or reuses) a self-signed code
# signing certificate called "BuildFlow Local Development", adds it to the
# *current user's* Trusted Root store, and signs the given files. None of that
# needs administrator rights, and the certificate is reused on later builds.
#
# With -CertificateThumbprint it signs with a certificate you already have -
# use that when you own a real code signing certificate and the private key is
# in Cert:\CurrentUser\My or Cert:\LocalMachine\My.

param(
    [string[]]$Path = @(),
    [string]$CertificateThumbprint = "",
    [string]$Subject = "BuildFlow Local Development",
    [string[]]$TimestampServer = @()
)

$ErrorActionPreference = "Stop"

if (-not $Path -or $Path.Count -eq 0) {
    $root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
    $Path = @(
        (Join-Path $root "dist\BuildFlow.exe"),
        (Join-Path $root "dist\BuildFlow-debug.exe")
    ) | Where-Object { Test-Path $_ }
}

if (-not $Path -or $Path.Count -eq 0) {
    Write-Host "Nothing to sign: no executables were found." -ForegroundColor Red
    exit 1
}

# --- find or create the certificate -----------------------------------------
$cert = $null

if ($CertificateThumbprint) {
    $thumb = ($CertificateThumbprint -replace '[^0-9A-Fa-f]', '').ToUpper()
    $found = Get-ChildItem Cert:\CurrentUser\My, Cert:\LocalMachine\My -ErrorAction SilentlyContinue |
        Where-Object { $_.Thumbprint -eq $thumb } | Select-Object -First 1
    if (-not $found) {
        Write-Host "No certificate with thumbprint $thumb has a private key on this machine." -ForegroundColor Red
        exit 1
    }
    $cert = $found
    Write-Host ("Using certificate " + $cert.Subject + " (" + $cert.Thumbprint + ")") -ForegroundColor DarkGray
} else {
    $existing = Get-ChildItem Cert:\CurrentUser\My -ErrorAction SilentlyContinue |
        Where-Object { $_.Subject -eq "CN=$Subject" -and $_.HasPrivateKey } | Select-Object -First 1

    if ($existing) {
        if ($existing.NotAfter -lt (Get-Date).AddDays(30)) {
            Write-Host "The local development certificate expires soon; renewing it." -ForegroundColor Yellow
            Remove-Item ("Cert:\CurrentUser\My\" + $existing.Thumbprint) -Force -ErrorAction SilentlyContinue
            $existing = $null
        }
    }
    if (-not $existing) {
        Write-Host "Creating a self-signed code signing certificate for the current user..." -ForegroundColor Cyan
        $existing = New-SelfSignedCertificate `
            -Subject "CN=$Subject" `
            -Type CodeSigningCert `
            -CertStoreLocation Cert:\CurrentUser\My `
            -NotAfter (Get-Date).AddYears(5) `
            -KeyUsage DigitalSignature `
            -KeyAlgorithm RSA -KeyLength 3072 `
            -HashAlgorithm SHA256
    }
    $cert = $existing
    Write-Host ("Certificate " + $cert.Subject + " (" + $cert.Thumbprint + ")") -ForegroundColor DarkGray

    # Trust it for the current user so Code Integrity accepts the signature.
    # The X509Store API is used on purpose: Import-Certificate wants to show a
    # confirmation dialog for the Root store, which fails in a non-interactive
    # session.
    $trusted = Get-ChildItem Cert:\CurrentUser\Root -ErrorAction SilentlyContinue |
        Where-Object { $_.Thumbprint -eq $cert.Thumbprint } | Select-Object -First 1
    if (-not $trusted) {
        $store = New-Object System.Security.Cryptography.X509Certificates.X509Store("Root", "CurrentUser")
        $store.Open([System.Security.Cryptography.X509Certificates.OpenFlags]::ReadWrite)
        try { $store.Add($cert) } finally { $store.Close() }
        Write-Host "  added to the current user's Trusted Root store" -ForegroundColor DarkGray
    } else {
        Write-Host "  already trusted by the current user" -ForegroundColor DarkGray
    }
}

# --- sign -------------------------------------------------------------------
$failed = 0
foreach ($file in $Path) {
    if (-not (Test-Path $file)) {
        Write-Host ("  missing: " + $file) -ForegroundColor Red
        $failed++
        continue
    }
    $params = @{
        FilePath      = $file
        Certificate   = $cert
        HashAlgorithm = "SHA256"
    }
    if ($TimestampServer -and $TimestampServer.Count -gt 0) {
        $params["TimeStampServer"] = $TimestampServer
    }
    $result = Set-AuthenticodeSignature @params
    if ($result.Status -eq "Valid") {
        Write-Host ("  signed " + (Split-Path -Leaf $file) + " -> " + $result.Status) -ForegroundColor Green
    } else {
        Write-Host ("  FAILED " + (Split-Path -Leaf $file) + " -> " + $result.Status + " " + $result.StatusMessage) -ForegroundColor Red
        $failed++
    }
}

if ($failed -gt 0) {
    Write-Host ""
    Write-Host "$failed file(s) could not be signed." -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "All executables are signed and should launch normally." -ForegroundColor Green
if (-not $CertificateThumbprint) {
    Write-Host "This certificate is trusted only on this machine. To hand the .exe to" -ForegroundColor DarkGray
    Write-Host "someone else, sign it with a real code signing certificate." -ForegroundColor DarkGray
}
exit 0
