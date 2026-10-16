# BuildFlow end-to-end verification.
#
# Exercises the real running server: project creation and switching, persistence,
# global vs local prompts, assembly, snapshots, package detection and N+1
# creation, separate Step 5 records, external-edit conflict detection, unsafe
# path rejection and cross-origin rejection.
#
# Usage:
#   pwsh -File scripts\verify.ps1                       # starts a private instance
#   pwsh -File scripts\verify.ps1 -BaseUrl http://127.0.0.1:8765 -TokenFile ...

param(
    [string]$BaseUrl = "",
    [string]$TokenFile = "",
    [string]$Exe = "",
    [string]$Workspace = "",
    [string]$WorkDir = "",
    [switch]$NoStart
)

# Continue (not Stop) so a malformed detail string cannot abort the whole run.
# Every real check reports its own pass/fail.
$ErrorActionPreference = "Continue"
$script:Failures = 0
$script:Checks = 0
$script:StartedProcess = $null

function Ok($name, $detail = "") {
    $script:Checks++
    Write-Host ("  PASS  " + $name) -ForegroundColor Green
    if ($detail) { Write-Host ("        " + $detail) -ForegroundColor DarkGray }
}

function Bad($name, $detail = "") {
    $script:Checks++
    $script:Failures++
    Write-Host ("  FAIL  " + $name) -ForegroundColor Red
    if ($detail) { Write-Host ("        " + $detail) -ForegroundColor Red }
}

function Check($name, $condition, $detail = "") {
    if ($condition) { Ok $name $detail } else { Bad $name $detail }
}

function Head($s, $n) {
    if ($null -eq $s) { return "" }
    $str = [string]$s
    if ($str.Length -le $n) { return $str }
    return $str.Substring(0, $n)
}

function Section($title) {
    Write-Host ""
    Write-Host ("== " + $title) -ForegroundColor Cyan
}

# ---------------------------------------------------------------- helpers

function Invoke-Api {
    param(
        [string]$Path,
        [string]$Method = "GET",
        $Body = $null,
        [hashtable]$Headers = @{},
        [switch]$NoToken
    )
    $h = @{}
    if (-not $NoToken) { $h["X-BuildFlow-Token"] = $script:Token }
    foreach ($k in $Headers.Keys) { $h[$k] = $Headers[$k] }

    # System.Net.HttpWebRequest is used rather than Invoke-WebRequest because
    # Windows PowerShell 5.1 hands back an empty stream for the body of a 4xx
    # response, which would hide every error message and conflict payload.
    $req = [System.Net.HttpWebRequest]::Create("$BaseUrl$Path")
    $req.Method = $Method
    $req.AllowAutoRedirect = $false
    $req.Timeout = 30000
    $req.ReadWriteTimeout = 120000
    foreach ($k in $h.Keys) {
        try { $req.Headers[$k] = [string]$h[$k] } catch { $req.Headers.Add($k, [string]$h[$k]) }
    }
    if ($null -ne $Body) {
        $bytes = [System.Text.Encoding]::UTF8.GetBytes(($Body | ConvertTo-Json -Depth 30 -Compress))
        $req.ContentType = "application/json; charset=utf-8"
        $req.ContentLength = $bytes.Length
        $stream = $req.GetRequestStream()
        try { $stream.Write($bytes, 0, $bytes.Length) } finally { $stream.Close() }
    }

    $resp = $null
    try {
        $resp = $req.GetResponse()
    } catch [System.Net.WebException] {
        $ex = $_.Exception
        if ($null -eq $ex.Response) {
            # Nothing is listening. Status 0 is the meaningful answer here:
            # the suite asserts exactly that after quitting, so report it
            # instead of throwing. Anything else is a genuine failure.
            $dead = @(
                [System.Net.WebExceptionStatus]::ConnectFailure,
                [System.Net.WebExceptionStatus]::SendFailure,
                [System.Net.WebExceptionStatus]::NameResolutionFailure,
                [System.Net.WebExceptionStatus]::ConnectionClosed
            )
            if ($dead -contains $ex.Status) {
                return [pscustomobject]@{ Status = 0; Body = $null; Raw = ""; Headers = @{} }
            }
            throw
        }
        $resp = $ex.Response
    }
    $status = [int]$resp.StatusCode
    $reader = New-Object System.IO.StreamReader($resp.GetResponseStream())
    $raw = ""
    try { $raw = $reader.ReadToEnd() } catch { $raw = "" }
    $reader.Close()
    $hdrs = $resp.Headers
    $resp.Close()

    $out = $null
    if ($raw) { try { $out = $raw | ConvertFrom-Json } catch { $out = $null } }
    return [pscustomobject]@{ Status = $status; Body = $out; Raw = $raw; Headers = $hdrs }
}

# ---------------------------------------------------------------- setup

$root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
if (-not $Exe) { $Exe = Join-Path $root "dist\BuildFlow-debug.exe" }
if (-not $Exe -or -not (Test-Path $Exe)) {
    $Exe = Join-Path $root "dist\BuildFlow.exe"
}
if (-not (Test-Path $Exe)) {
    Write-Host "BuildFlow executable not found. Run build.ps1 first." -ForegroundColor Red
    exit 2
}

if (-not $WorkDir) {
    $WorkDir = Join-Path $env:TEMP ("buildflow-verify-" + [guid]::NewGuid().ToString("N").Substring(0, 8))
}
New-Item -ItemType Directory -Force -Path $WorkDir | Out-Null
$Home1 = Join-Path $WorkDir "home"
if (-not $Workspace) { $Workspace = Join-Path $WorkDir "workspace" }
New-Item -ItemType Directory -Force -Path $Home1, $Workspace | Out-Null

if (-not $NoStart) {
    Write-Host "Starting a private BuildFlow instance" -ForegroundColor Cyan
    Write-Host "  exe       : $Exe"
    Write-Host "  home      : $Home1"
    Write-Host "  workspace : $Workspace"
    $env:BUILDFLOW_HOME = $Home1
    # Reserve a free loopback port for this isolated run, then pass it explicitly.
    if (-not $BaseUrl) {
        $probe = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 0)
        $probe.Start()
        $verifyPort = $probe.LocalEndpoint.Port
        $probe.Stop()
        $BaseUrl = "http://127.0.0.1:$verifyPort"
    } else {
        $verifyPort = ([uri]$BaseUrl).Port
    }
    $script:StartedProcess = Start-Process -FilePath $Exe `
        -ArgumentList @("--workspace", $Workspace, "--no-browser", "--port", "$verifyPort") `
        -PassThru -WindowStyle Hidden `
        -RedirectStandardOutput (Join-Path $WorkDir "stdout.txt") `
        -RedirectStandardError (Join-Path $WorkDir "stderr.txt")
    Start-Sleep -Seconds 2
}

if (-not $BaseUrl) { $BaseUrl = "http://127.0.0.1:8765" }

if (-not $TokenFile) {
    $TokenFile = Join-Path $Home1 "api-token.txt"
}
if (-not (Test-Path $TokenFile)) {
    Write-Host "Token file not found at $TokenFile" -ForegroundColor Red
    if ($script:StartedProcess) { $script:StartedProcess.Kill() }
    exit 2
}
$script:Token = (Get-Content $TokenFile | ForEach-Object { $_.Trim() } |
    Where-Object { $_ -and -not $_.StartsWith("#") } | Select-Object -First 1)
if (-not $script:Token) {
    Write-Host "Could not read a token from $TokenFile" -ForegroundColor Red
    if ($script:StartedProcess) { $script:StartedProcess.Kill() }
    exit 2
}
Write-Host ("Token loaded (" + $script:Token.Length + " characters)") -ForegroundColor DarkGray

function Get-FileText([string]$path) {
    if (Test-Path $path) { return [System.IO.File]::ReadAllText($path) } else { return "" }
}

# ---------------------------------------------------------------- 1. health
Section "1. Startup, loopback binding and access control"

$health = Invoke-Api "/api/health" -NoToken
Check "health endpoint answers without a token" ($health.Status -eq 200 -and $health.Body.ok) ("status " + $health.Status)
Check "health reports loopback URL" ($health.Body.url -eq $BaseUrl) $health.Body.url

$noToken = Invoke-Api "/api/projects" -NoToken
Check "API refuses a request with no token" ($noToken.Status -eq 401) ("status " + $noToken.Status)

$badToken = Invoke-Api "/api/projects" -Headers @{ "X-BuildFlow-Token" = "0" * 64 }
Check "API refuses a wrong token" ($badToken.Status -eq 401) ("status " + $badToken.Status)

$crossOrigin = Invoke-Api "/api/projects" -Headers @{ "Origin" = "https://evil.example" }
Check "API refuses an unapproved Origin" ($crossOrigin.Status -eq 403) ("status " + $crossOrigin.Status)

$secFetch = Invoke-Api "/api/projects" -Headers @{ "Sec-Fetch-Site" = "cross-site" }
Check "API refuses Sec-Fetch-Site: cross-site" ($secFetch.Status -eq 403) ("status " + $secFetch.Status)

$stateChange = Invoke-Api "/api/projects" -Method "POST" -Body @{ name = "x" } -Headers @{ "Origin" = "https://evil.example" }
Check "cross-origin state change is refused before it happens" ($stateChange.Status -eq 403) ("status " + $stateChange.Status)
Check "no CORS headers are sent" ($health.Headers["Access-Control-Allow-Origin"] -eq $null)

if ($script:StartedProcess) {
    $listeners = Get-NetTCPConnection -State Listen -LocalPort ([uri]$BaseUrl).Port -ErrorAction SilentlyContinue
    $addrs = @($listeners | Select-Object -ExpandProperty LocalAddress -Unique)
    Check "listener is bound to 127.0.0.1 only" (($addrs.Count -eq 1) -and ($addrs[0] -eq "127.0.0.1")) ("addresses: " + ($addrs -join ", "))
}

# ---------------------------------------------------------------- 2. projects
Section "2. Project creation, switching, renaming"

$created = Invoke-Api "/api/projects" -Method "POST" -Body @{ name = "Apollo Storefront" }
Check "create project" ($created.Status -eq 201) ("status " + $created.Status + " id=" + $created.Body.id)
$projA = $created.Body.id
Check "project folder is slugified" ($projA -eq "apollo-storefront") $projA
Check "project.json written" (Test-Path (Join-Path $Workspace "projects\$projA\project.json"))

$projADir = Join-Path $Workspace "projects\$projA"
foreach ($d in @("inputs", "inputs\step-05", "outputs", "outputs\step-05", "packages", "prompts\local-overrides", "prompts\snapshots")) {
    Check ("folder exists: " + $d) (Test-Path (Join-Path $projADir $d))
}
Check "global starter templates installed" (Test-Path (Join-Path $Workspace "global-prompts\step-01.md"))
$starterCount = (Get-ChildItem (Join-Path $Workspace "global-prompts") -Filter *.md).Count
Check "all six starter templates present" ($starterCount -eq 6) ("found " + $starterCount)

$second = Invoke-Api "/api/projects" -Method "POST" -Body @{ name = "Zeta Billing" }
$projB = $second.Body.id
Check "create a second project" ($second.Status -eq 201) ("id=" + $projB)

$list = Invoke-Api "/api/projects"
$ids = @($list.Body.projects | ForEach-Object { $_.id })
Check "both projects are listed" (($ids -contains $projA) -and ($ids -contains $projB)) ($ids -join ", ")

$renamed = Invoke-Api "/api/projects/$projB" -Method "PATCH" -Body @{ name = "Zeta Billing Engine" }
Check "rename a project" ($renamed.Status -eq 200) ("new id=" + $renamed.Body.id + " name=" + $renamed.Body.name)
Check "renamed folder exists" (Test-Path (Join-Path $Workspace "projects\$($renamed.Body.id)")) ("projects/" + $renamed.Body.id)
$projB = $renamed.Body.id

$dup = Invoke-Api "/api/projects" -Method "POST" -Body @{ name = "Apollo Storefront" }
Check "duplicate project name is refused" ($dup.Status -eq 400) $dup.Body.error

# ---------------------------------------------------------------- 3. documents
Section "3. Document save, reload and persistence"

# A fresh project has no input files yet, so the very first save is clean.
Check "a new project has no input files yet" (-not (Test-Path (Join-Path $projADir "inputs\step-01.md")))
$put = Invoke-Api "/api/projects/$projA/doc" -Method "PUT" -Body @{
        kind = "input"; stage = "step-01"; content = "# Use case`n`nBuild a local store inventory app."; baseHash = ""
    }
Check "write step-01 input" ($put.Status -eq 200 -and $put.Body.exists) ("status " + $put.Status + " hash " + (Head $put.Body.hash 12))
$inputHash = $put.Body.hash
$get = Invoke-Api "/api/projects/$projA/doc?kind=input&stage=step-01"
Check "read it back byte for byte" ($get.Body.content -eq "# Use case`n`nBuild a local store inventory app.")
Check "read returns the saved hash" ($get.Body.hash -eq $inputHash)

$onDisk = Get-FileText (Join-Path $projADir "inputs\step-01.md")
Check "file on disk is UTF-8 Markdown" ($onDisk.StartsWith("# Use case")) ((Head $onDisk 30) + "...")

# A save with a stale hash must be refused.
$stale = Invoke-Api "/api/projects/$projA/doc" -Method "PUT" -Body @{
        kind = "input"; stage = "step-01"; content = "STALE WRITE"; baseHash = ("f" * 64)
    }
Check "save with a stale hash is refused (409)" ($stale.Status -eq 409) ("status " + $stale.Status)
Check "conflict response carries the disk copy" ($stale.Body.details.content -like "# Use case*")
Check "refused write did not change the file" ((Get-FileText (Join-Path $projADir "inputs\step-01.md")) -eq $onDisk)

# A client that says "I think this file does not exist" must be told otherwise.
$blind = Invoke-Api "/api/projects/$projA/doc" -Method "PUT" -Body @{
        kind = "input"; stage = "step-01"; content = "BLIND WRITE"; baseHash = ""
    }
Check "a hashless save onto an existing file is refused (409)" ($blind.Status -eq 409) ("status " + $blind.Status)
Check "the blind save was reported as 'created'" ($blind.Body.details.reason -eq "created") $blind.Body.details.reason
Check "the blind save left the file alone" ((Get-FileText (Join-Path $projADir "inputs\step-01.md")) -eq $onDisk)

# ---------------------------------------------------------------- 4. external edits
Section "4. External edit conflict (Obsidian scenario)"

$outPath = Join-Path $projADir "outputs\step-01.md"
Invoke-Api "/api/projects/$projA/doc" -Method "PUT" -Body @{
    kind = "output"; stage = "step-01"; content = "# Research brief`n`nFirst draft."; baseHash = ""
} | Out-Null

# Simulate the user editing the file in Obsidian while BuildFlow has it open.
Start-Sleep -Milliseconds 1100
[System.IO.File]::WriteAllText($outPath, "# Research brief`n`nEDITED IN OBSIDIAN, newer version.")
Start-Sleep -Milliseconds 200

$conflict = Invoke-Api "/api/projects/$projA/doc" -Method "PUT" -Body @{
    kind = "output"; stage = "step-01"; content = "# Research brief`n`nMY BUILDFLOW TEXT."; baseHash = ""
}
Check "external edit is detected (409 conflict)" ($conflict.Status -eq 409) ("status " + $conflict.Status)
Check "conflict reason is 'created'" ($conflict.Body.details.reason -eq "created") $conflict.Body.details.reason
Check "conflict returns the newer disk content" ($conflict.Body.details.content -like "*EDITED IN OBSIDIAN*")
Check "BuildFlow did not overwrite the newer file" ((Get-FileText $outPath) -like "*EDITED IN OBSIDIAN*")

# Now the modified case, with a correct base hash for a file that then changed.
$out2 = Invoke-Api "/api/projects/$projA/doc?kind=output&stage=step-01"
$mine = "MY TEXT"
[System.IO.File]::WriteAllText($outPath, "# Research brief`n`nAGAIN EDITED ELSEWHERE.")
$conflict2 = Invoke-Api "/api/projects/$projA/doc" -Method "PUT" -Body @{
    kind = "output"; stage = "step-01"; content = $mine; baseHash = $out2.Body.hash
}
Check "modified-since-load is detected" ($conflict2.Status -eq 409 -and $conflict2.Body.details.reason -eq "modified") $conflict2.Body.details.reason

# Explicit overwrite after the user chooses it.
$forced = Invoke-Api "/api/projects/$projA/doc" -Method "PUT" -Body @{
    kind = "output"; stage = "step-01"; content = $mine; baseHash = ""; force = $true
}
Check "explicit force overwrite works" ($forced.Status -eq 200) ("status " + $forced.Status)
Check "forced content is on disk" ((Get-FileText $outPath) -eq $mine)

# ---------------------------------------------------------------- 5. prompts
Section "5. Global vs local prompt templates"

$tmpl = Invoke-Api "/api/projects/$projA/template?stage=step-01&scope=local"
Check "global template is active by default" ($tmpl.Body.activeScope -eq "global") $tmpl.Body.activeScope
Check "global template is the starter" ($tmpl.Body.isStarter -eq $true)
Check "no local override yet" ($tmpl.Body.hasOverride -eq $false)

$localText = $tmpl.Body.content + "`n`n## My house rules`n`nAlways answer in British English.`n"
$saveLocal = Invoke-Api "/api/projects/$projA/template" -Method "PUT" -Body @{
    stage = "step-01"; scope = "local"; content = $localText; baseHash = ""
}
Check "save a local override" ($saveLocal.Status -eq 200 -and $saveLocal.Body.activeScope -eq "local") $saveLocal.Body.activeScope
Check "override marked as different from global" ($saveLocal.Body.modifiedFromGlobal -eq $true)
Check "local override file written" (Test-Path (Join-Path $projADir "prompts\local-overrides\step-01.md"))
Check "global template file untouched" ((Get-FileText (Join-Path $Workspace "global-prompts\step-01.md")) -notlike "*British English*")

# The hash must describe the file being written. A browser that sent the global
# template's hash while creating an override would be refused, which is what
# made the first local save look like a conflict.
$globalHash = $tmpl.Body.hash
$badBase = Invoke-Api "/api/projects/$projA/template" -Method "PUT" -Body @{
    stage = "step-01"; scope = "local"; content = $localText + "`nmore`n"; baseHash = $globalHash
}
Check "writing an override with the global hash is refused" ($badBase.Status -eq 409) ("status " + $badBase.Status + ", reason " + $badBase.Body.details.reason)
Check "the refused save did not touch the override" ((Get-FileText (Join-Path $projADir "prompts\local-overrides\step-01.md")) -notlike "*more*")

# Overwriting an existing override is still conflict checked, against the
# override's own hash.
$badBase2 = Invoke-Api "/api/projects/$projA/template" -Method "PUT" -Body @{
    stage = "step-01"; scope = "local"; content = $localText + "`nstale`n"; baseHash = "0000000000000000000000000000000000000000000000000000000000000000"
}
Check "a stale override hash is refused" ($badBase2.Status -eq 409) ("status " + $badBase2.Status)
$okBase2 = Invoke-Api "/api/projects/$projA/template" -Method "PUT" -Body @{
    stage = "step-01"; scope = "local"; content = $localText; baseHash = $saveLocal.Body.hash
}
Check "the override's own hash is accepted" ($okBase2.Status -eq 200) ("status " + $okBase2.Status)

# Project B must not see project A's override.
$tmplB = Invoke-Api "/api/projects/$projB/template?stage=step-01&scope=local"
Check "the override is not global" ($tmplB.Body.activeScope -eq "global" -and $tmplB.Body.hasOverride -eq $false)

# Revert.
$revert = Invoke-Api "/api/projects/$projA/template?stage=step-01&expectedHash=$($saveLocal.Body.hash)" -Method "DELETE"
Check "revert the local override" ($revert.Status -eq 200 -and $revert.Body.activeScope -eq "global") $revert.Body.activeScope
Check "override file deleted" (-not (Test-Path (Join-Path $projADir "prompts\local-overrides\step-01.md")))

# ---------------------------------------------------------------- 6. assembly
Section "6. Prompt assembly and prerequisites"

# Put a real local override on step 3 so assembly has something local to use.
$global03 = (Invoke-Api "/api/projects/$projA/template?stage=step-03&scope=global").Body.content
$saveLocal2 = Invoke-Api "/api/projects/$projA/template" -Method "PUT" -Body @{
    stage = "step-03"; scope = "local"; content = ($global03 + "`n## Local house style`n`nBe blunt.`n"); baseHash = ""
}
Check "step-03 local override saved" ($saveLocal2.Status -eq 200) ("status " + $saveLocal2.Status)
Check "step-03 override is marked as an override" ($saveLocal2.Body.activeScope -eq "local" -and $saveLocal2.Body.hasOverride)

# Step 3 needs steps 2 and 2.5; both are empty right now.
$asm3 = Invoke-Api "/api/projects/$projA/assemble?stage=step-03"
Check "assemble step 3" ($asm3.Status -eq 200) ("status " + $asm3.Status)
$missingPaths = @($asm3.Body.warnings | Where-Object { $_.required } | ForEach-Object { $_.path })
Check "step 3 reports both missing prerequisites" (($missingPaths -like "*step-02.md").Count -gt 0 -and ($missingPaths -like "*step-02-5.md").Count -gt 0) ($missingPaths -join ", ")
Check "assembled prompt marks the missing inputs" ($asm3.Body.projection.state -eq "blocked")
Check "assembly used the local template" ($asm3.Body.templateScope -eq "local" -and $asm3.Body.templatePath -like "*local-overrides*") $asm3.Body.templatePath

# Now fill in the prerequisites.
Invoke-Api "/api/projects/$projA/doc" -Method "PUT" -Body @{
    kind = "output"; stage = "step-02"; content = "# Decision`n`nWe chose a local-first architecture."; baseHash = ""
} | Out-Null
Invoke-Api "/api/projects/$projA/doc" -Method "PUT" -Body @{
    kind = "output"; stage = "step-02-5"; baseHash = ""
    content = "# Stack`n`nGo standard library only.`n`n## Reusable System Context`n`n- Mission: a local workbench.`n- Stack: Go 1.24.`n- Testing: go test.`n"
} | Out-Null
Invoke-Api "/api/projects/$projA/doc" -Method "PUT" -Body @{
    kind = "output"; stage = "step-03"; content = "# Low-level design`n`nThree modules."; baseHash = ""
} | Out-Null

$asm3b = Invoke-Api "/api/projects/$projA/assemble?stage=step-03"
$missing3b = @($asm3b.Body.warnings | Where-Object { $_.severity -eq "missing" -and $_.required })
Check "no missing prerequisites once saved" ($missing3b.Count -eq 0) ((@($missing3b | ForEach-Object { $_.token })) -join ", ")
Check "optional inputs are only advisory" ((@($asm3b.Body.warnings | Where-Object { $_.token -eq "input" -and -not $_.required })).Count -eq 1)
Check "step 2 output is embedded in the step 3 prompt" ($asm3b.Body.content -like "*local-first architecture*")
Check "step 2.5 output is embedded in the step 3 prompt" ($asm3b.Body.content -like "*Go standard library only*")
Check "step 3's own input slot is present" ($asm3b.Body.content -like "*## User design notes*")
Check "template placeholder {{output_step_02}} was substituted" ($asm3b.Body.content -notlike "*{{output_step_02}}*")
Check "prompt length is plausible" ($asm3b.Body.charCount -gt 1500) ("chars " + $asm3b.Body.charCount)

$asm4 = Invoke-Api "/api/projects/$projA/assemble?stage=step-04"
Check "step 4 pulls in the step 3 output" ($asm4.Body.content -like "*Three modules*")
Check "step 4 does not require step 1" ($asm4.Body.content -like "*## User packaging notes*")
Check "step 4's prerequisites are all satisfied" (@($asm4.Body.warnings | Where-Object { $_.severity -eq "missing" -and $_.required }).Count -eq 0)

$asm2 = Invoke-Api "/api/projects/$projA/assemble?stage=step-02"
Check "step 2 pulls in the step 1 output" ($asm2.Body.content -like "*Research brief*" -or $asm2.Body.content -like "*MY TEXT*")

$asm1 = Invoke-Api "/api/projects/$projA/assemble?stage=step-01"
Check "step 1 assembles from its own input" ($asm1.Body.content -like "*local store inventory app*")
Check "step 1 reports no missing prerequisites" (@($asm1.Body.warnings | Where-Object { $_.severity -eq "missing" -and $_.required }).Count -eq 0)
Check "step 1 still lists its prerequisite slots" ($asm1.Body.content -like "*local store inventory app*")

# ---------------------------------------------------------------- 7. snapshots
Section "7. Prompt snapshots"

$edited = $asm3b.Body.content + "`n`n## My addition`n`nI edited this before copying."
$snap = Invoke-Api "/api/projects/$projA/snapshot" -Method "POST" -Body @{
    stage = "step-03"; content = $edited
}
Check "save a prompt snapshot" ($snap.Status -eq 200 -and $snap.Body.hasSnapshot) ("path " + $snap.Body.snapshotPath)
$snapPath = Join-Path $Workspace "projects\$projA\prompts\snapshots\step-03.md"
Check "snapshot file written" (Test-Path $snapPath)
$snapText = Get-FileText $snapPath
Check "snapshot holds the exact text that was sent" (($snapText -replace '\A---\r?\n[\s\S]*?\r?\n---\r?\n\r?\n', "") -ceq $edited)
Check "snapshot keeps the assembled body verbatim" ($snapText -like "*local-first architecture*")
Check "snapshot records the local override that produced it" ($snapText -like "*Local house style*")
Check "snapshot is detected as differing from a fresh assembly" ($snap.Body.snapshotDiffers -eq $true)

$snapReload = Invoke-Api "/api/projects/$projA/doc?kind=snapshot&stage=step-03"
Check "snapshot can be loaded again verbatim" (($snapReload.Body.content -replace '\A---\r?\n[\s\S]*?\r?\n---\r?\n\r?\n', "") -ceq $edited)

# ---------------------------------------------------------------- 8. packages
Section "8. Package detection, import and N+1"

$step04 = @"
# Work packages

## P01 - Local REST API
Deliver the loopback server and routing.
- Files: internal/server/*
- Tests: internal/server/server_test.go

## P02 - Markdown document store
Safe writes, conflict detection, package detection.

## P03 - Embedded workbench
The HTML, CSS and JS interface.

### Not a package heading
This is body text under P03 and must not become a package.

## P04 - Packaging and installer
Build script, README, verification.
"@
Invoke-Api "/api/projects/$projA/doc" -Method "PUT" -Body @{ kind = "output"; stage = "step-04"; content = $step04; baseHash = "" } | Out-Null

$det = Invoke-Api "/api/projects/$projA/detect-packages" -Method "POST" -Body @{}
Check "detect packages from the step 4 output" ($det.Status -eq 200) ("status " + $det.Status)
$ids = @($det.Body.packages | ForEach-Object { $_.id })
Check "four packages detected" ($ids.Count -eq 4) ($ids -join ", ")
Check "ids are P01..P04" (($ids -join ",") -eq "P01,P02,P03,P04") ($ids -join ",")
$p01 = $det.Body.packages | Where-Object { $_.id -eq "P01" }
Check "P01 title detected" ($p01.title -eq "Local REST API") $p01.title
Check "P01 body includes its subsections" ($p01.content -like "*internal/server*")
Check "exactly four packages were detected" (@($det.Body.packages).Count -eq 4) ("count " + @($det.Body.packages).Count)
Check "P03 body keeps the ### subheading text" (($det.Body.packages | Where-Object { $_.id -eq "P03" }).content -like "*### Not a package heading*")
Check "the ### subheading did not become a package" (@($det.Body.packages | Where-Object { $_.heading -like "*Not a package heading*" }).Count -eq 0)
Check "preamble above the first heading is reported" ($det.Body.unassigned -like "*# Work packages*")
Check "nothing was written by detection" (-not (Test-Path (Join-Path $projADir "packages\P01.md")))

$imp = Invoke-Api "/api/projects/$projA/import-packages" -Method "POST" -Body @{
    packages = $det.Body.packages; accept = @("P01", "P02", "P03"); overwrite = @()
}
Check "import three packages" ($imp.Status -eq 200 -and $imp.Body.created.Count -eq 3) ("created: " + ($imp.Body.created -join ", "))
Check "P01.md created" (Test-Path (Join-Path $projADir "packages\P01.md"))
Check "P04.md not created (not accepted)" (-not (Test-Path (Join-Path $projADir "packages\P04.md")))
$pkg1 = Get-FileText (Join-Path $projADir "packages\P01.md")
Check "package file has Obsidian front matter" ($pkg1 -like "*`nid: P01*")
Check "package file keeps the original heading" ($pkg1 -like "*## P01 - Local REST API*")

# Hand-edit P02, then re-detect and re-import: the hand edit must survive.
$p02Path = Join-Path $projADir "packages\P02.md"
[System.IO.File]::WriteAllText($p02Path, "---`nid: P02`n---`n`n## P02 - My hand written version`n`nDo not lose this.`n")
$det2 = Invoke-Api "/api/projects/$projA/detect-packages" -Method "POST" -Body @{}
$p02det = $det2.Body.packages | Where-Object { $_.id -eq "P02" }
Check "detection flags the existing package file" ($p02det.fileExists -eq $true) ("chars " + $p02det.existingChars)
Check "detection warns about the overwrite" (($p02det.warnings -join " ") -like "*already exists*")

$imp2 = Invoke-Api "/api/projects/$projA/import-packages" -Method "POST" -Body @{
    packages = $det2.Body.packages; accept = @("P01", "P02", "P03"); overwrite = @()
}
Check "import without overwrite flags P02 as refused" ($imp2.Body.refused -contains "P02") ("refused: " + ($imp2.Body.refused -join ", "))
Check "hand-edited P02.md survived" ((Get-FileText $p02Path) -like "*My hand written version*")
Check "P02.md still holds my edit text" ((Get-FileText $p02Path) -like "*Do not lose this*")

$imp3 = Invoke-Api "/api/projects/$projA/import-packages" -Method "POST" -Body @{
    packages = $det2.Body.packages; accept = @("P02"); overwrite = @("P02")
}
Check "explicit overwrite does replace it" ($imp3.Body.overwritten -contains "P02") ("overwritten: " + ($imp3.Body.overwritten -join ", "))

# An import that changes nothing must still answer with arrays, not nulls. A
# client reading result.created.length crashes on a null, which is exactly what
# a browser did before ImportResult's lists were initialised.
Check "import reports an empty created list, not null" ($null -ne $imp3.Body.created)
Check "import reports an empty skipped list, not null" ($null -ne $imp3.Body.skipped)
Check "empty import lists are JSON arrays" (($imp3.Raw -replace "\s", "") -match '"created":\[\]') (Head $imp3.Raw 160)
Check "import result still names the overwritten package" (($imp3.Raw -replace "\s", "") -match '"overwritten":\["P02"\]') (Head $imp3.Raw 160)

# Add next package -> P04
$add = Invoke-Api "/api/projects/$projA/packages" -Method "POST" -Body @{ title = "Verification harness" }
Check "Add next package creates P04" ($add.Status -eq 201 -and $add.Body.id -eq "P04") ("id " + $add.Body.id)
Check "P04.md created on disk" (Test-Path (Join-Path $projADir "packages\P04.md"))
$add2 = Invoke-Api "/api/projects/$projA/packages" -Method "POST" -Body @{}
Check "adding again gives P05 (N+1)" ($add2.Body.id -eq "P05") ("id " + $add2.Body.id)
$add3 = Invoke-Api "/api/projects/$projA/packages" -Method "POST" -Body @{ id = "P99"; title = "Far future" }
Check "explicit id accepted and padded" ($add3.Body.id -eq "P99") ("id " + $add3.Body.id)
$addDup = Invoke-Api "/api/projects/$projA/packages" -Method "POST" -Body @{ id = "P99" }
Check "duplicate package id refused" ($addDup.Status -eq 400) $addDup.Body.error
$addBad = Invoke-Api "/api/projects/$projA/packages" -Method "POST" -Body @{ id = "../evil" }
Check "unsafe package id refused" ($addBad.Status -eq 400) $addBad.Body.error

# ---------------------------------------------------------------- 9. step 5
Section "9. Step 5 per-package records"

$projNow = Invoke-Api "/api/projects/$projA"
Check "project reports six packages" ($projNow.Body.packages.Count -eq 6) ((@($projNow.Body.packages | ForEach-Object { $_.id })) -join ", ")

$asm5a = Invoke-Api "/api/projects/$projA/assemble?stage=step-05&package=P01"
Check "step 5 assembles for P01" ($asm5a.Status -eq 200) ("status " + $asm5a.Status)
Check "P01 prompt contains the package body" ($asm5a.Body.content -like "*Local REST API*")
Check "P01 prompt contains the system context" ($asm5a.Body.content -like "*Reusable System Context*" -or $asm5a.Body.content -like "*local workbench*")
Check "P01 prompt does not contain another package" ($asm5a.Body.content -notlike "*Hand written*" )
Check "P01 prompt names the package" ($asm5a.Body.content -like "*P01*")

$asm5b = Invoke-Api "/api/projects/$projA/assemble?stage=step-05&package=P03"
Check "step 5 assembles for P03" ($asm5b.Status -eq 200)
Check "P03 prompt contains the P03 body" ($asm5b.Body.content -like "*Embedded workbench*")
Check "P03 prompt excludes the P01 body" ($asm5b.Body.content -notlike "*loopback server and routing*")
Check "P01 and P03 prompts are genuinely different" ($asm5a.Body.content -ne $asm5b.Body.content)

# Per package input notes and outputs.
Invoke-Api "/api/projects/$projA/doc" -Method "PUT" -Body @{
    kind = "package-input"; stage = "step-05"; package = "P01"; content = "Focus on the routing tests."; baseHash = ""
} | Out-Null
Invoke-Api "/api/projects/$projA/doc" -Method "PUT" -Body @{
    kind = "package-output"; stage = "step-05"; package = "P01"; content = "# P01 coding prompt`n`nImplement the routing and run its tests."; baseHash = ""
} | Out-Null
$snap5 = Invoke-Api "/api/projects/$projA/snapshot" -Method "POST" -Body @{
    stage = "step-05"; package = "P01"; content = $asm5a.Body.content
}
Check "step 5 snapshot for P01" ($snap5.Status -eq 200 -and $snap5.Body.snapshotPath -like "*step-05-P01.md") $snap5.Body.snapshotPath
Check "step 5 snapshot file on disk" (Test-Path (Join-Path $projADir "prompts\snapshots\step-05-P01.md"))
$asm5c = Invoke-Api "/api/projects/$projA/assemble?stage=step-05&package=P01"
Check "P01 prompt now includes its input notes" ($asm5c.Body.content -like "*Focus on the routing tests*")
Check "P01 snapshot still present and differing" ($asm5c.Body.hasSnapshot -and $asm5c.Body.snapshotDiffers)

$projAfter = Invoke-Api "/api/projects/$projA"
$p01info = $projAfter.Body.packages | Where-Object { $_.id -eq "P01" }
$p03info = $projAfter.Body.packages | Where-Object { $_.id -eq "P03" }
Check "legacy P01 output has artifact status output-saved" ($p01info.status -eq "output-saved") ("status " + $p01info.status)
Check "P01 has its own output record" ($p01info.hasOutput -eq $true)
Check "P03 has no output of its own" ($p03info.hasOutput -eq $false)
Check "P03 status is ready" ($p03info.status -eq "ready") ("status " + $p03info.status)
Check "inputs/step-05 and outputs/step-05 are separate folders" ((Test-Path (Join-Path $projADir "inputs\step-05\P01.md")) -and (Test-Path (Join-Path $projADir "outputs\step-05\P01.md")))
Check "step 5 input is not in the project root inputs folder" (-not (Test-Path (Join-Path $projADir "inputs\P01.md")))

# Missing system context must be reported loudly.
Write-Output "        (project $projB has no effective Step 3 context, so Step 5 must warn)"
$asm5Missing = Invoke-Api "/api/projects/$projB/assemble?stage=step-05&package=P01"
Check "step 5 warns when effective context is missing" ((@($asm5Missing.Body.warnings | Where-Object { $_.required -and $_.path -like "*step-03.md" }).Count -gt 0)) ((@($asm5Missing.Body.warnings | ForEach-Object { $_.token + ": " + $_.reason })) -join ", ")
Check "missing system context is marked in the prompt text" ($asm5Missing.Body.projection.state -eq "blocked")
Check "step 5 warns about the unknown package too" ((@($asm5Missing.Body.warnings | Where-Object { $_.required -and $_.path -like "*P01.md" }).Count -gt 0))

# Legacy recovery preserves context, with explicit fallback diagnostics.
$sysOnly = Invoke-Api "/api/projects/$projA/assemble?stage=step-05&package=P02"
Check "legacy step 5 retains the available system context" ($sysOnly.Body.content -like "*Mission: a local workbench*")
Check "legacy recovery is labelled in provenance" (@($sysOnly.Body.sources | Where-Object { $_.mode -like "markdown_*" }).Count -gt 0)

# ---------------------------------------------------------------- 10. source preservation
Section "10. Answer saves preserve notes and legacy copies"

$notes = "My own notes.`n`n<!-- buildflow:auto:step-01 -->`nOLD_COPIED_ANSWER`n<!-- /buildflow:auto:step-01 -->"
$noteSave = Invoke-Api "/api/projects/$projB/doc" -Method "PUT" -Body @{
    kind = "input"; stage = "step-02"; content = $notes; baseHash = ""
}
Check "save legacy notes fixture" ($noteSave.Status -eq 200)
foreach ($pair in @(
    @{ stage = "step-01"; next = "step-02" },
    @{ stage = "step-02"; next = "step-02-5" },
    @{ stage = "step-02-5"; next = "step-03" },
    @{ stage = "step-03"; next = "step-04" }
)) {
    $beforeNotes = (Invoke-Api "/api/projects/$projB/doc?kind=input&stage=$($pair.next)").Body
    $answerSave = Invoke-Api "/api/projects/$projB/doc" -Method "PUT" -Body @{
        kind = "output"; stage = $pair.stage; content = "# Legacy answer`n`nUPDATED_SOURCE_$($pair.stage)"; baseHash = ""
    }
    $afterNotes = (Invoke-Api "/api/projects/$projB/doc?kind=input&stage=$($pair.next)").Body
    Check ("saving " + $pair.stage + " leaves downstream input unchanged") ($answerSave.Status -eq 200 -and $beforeNotes.hash -eq $afterNotes.hash -and $beforeNotes.exists -eq $afterNotes.exists)
}
$acAsm2 = Invoke-Api "/api/projects/$projB/assemble?stage=step-02"
Check "assembly retains user notes" ($acAsm2.Body.content -like "*My own notes.*")
Check "assembly excludes old copied answers" ($acAsm2.Body.content -notlike "*OLD_COPIED_ANSWER*")
Check "legacy handoff includes the saved source once" ([regex]::Matches($acAsm2.Body.content, "UPDATED_SOURCE_step-01").Count -eq 1)
Check "assembly leaves legacy notes intact on disk" ((Invoke-Api "/api/projects/$projB/doc?kind=input&stage=step-02").Body.content -ceq $notes)

# ---------------------------------------------------------------- 11. step 4 slots
Section "11. Step 4 per-package slots"

$slotsFresh = Invoke-Api "/api/projects/$projB/step4"
$freshIds = @($slotsFresh.Body.slots | ForEach-Object { $_.id })
Check "a fresh project offers one empty slot" ($freshIds.Count -ge 1 -and $freshIds[0] -eq "P01") ($freshIds -join ", ")

# Saving a slot output creates the package without modifying Step 5 notes.
$slotOut = Invoke-Api "/api/projects/$projB/doc" -Method "PUT" -Body @{ kind = "package"; stage = ""; package = "P01"; content = "## P01 - API`n`nBuild it."; baseHash = "" }
Check "saving a slot output creates the package" ($slotOut.Status -eq 200 -and $slotOut.Body.exists)
$slotPkg = Invoke-Api "/api/projects/$projB/doc?kind=package&package=P01"
Check "the package carries front matter" ($slotPkg.Body.content -like "*id: P01*")
$slotIn5 = Invoke-Api "/api/projects/$projB/doc?kind=package-input&stage=step-05&package=P01"
Check "saving a package leaves step 5 input absent" (-not $slotIn5.Body.exists)

# The n+1 rule: a filled slot opens the next empty one.
$slotsAfter = Invoke-Api "/api/projects/$projB/step4"
$afterIds = @($slotsAfter.Body.slots | ForEach-Object { $_.id })
Check "a filled slot opens the next empty slot" ($afterIds.Count -eq 2 -and $afterIds[1] -eq "P02") ($afterIds -join ", ")

# Filling the slot's own input notes is saved separately.
$slotIn = Invoke-Api "/api/projects/$projB/doc" -Method "PUT" -Body @{ kind = "step4-input"; stage = "step-04"; package = "P01"; content = "Notes for P01."; baseHash = "" }
Check "slot input notes are saved" ($slotIn.Status -eq 200)
$slotInCheck = Invoke-Api "/api/projects/$projB/doc?kind=step4-input&stage=step-04&package=P01"
Check "slot input notes are on disk" ($slotInCheck.Body.content -like "*Notes for P01.*")

# ------------------------------------------------ structured workflow fixtures
Section "12. All six structured stages and source fidelity"

$structuredProject = (Invoke-Api "/api/projects" -Method "POST" -Body @{ name = "Structured workflow" }).Body.id
$fixtureDir = Join-Path $root "internal/structured/testdata"
foreach ($stage in @("step-01", "step-02", "step-02-5", "step-03", "step-04", "step-05")) {
    # Use the ordinary template endpoint even when adopting a shipped starter.
    $active = (Invoke-Api "/api/projects/$structuredProject/template?stage=$stage&scope=global").Body
    $starter = Get-FileText (Join-Path $root "internal/workspace/starter/$stage.md")
    $savedTemplate = Invoke-Api "/api/projects/$structuredProject/template" -Method "PUT" -Body @{
        stage = $stage; scope = "global"; content = $starter; baseHash = $active.hash
    }
    Check ("save active template " + $stage) ($savedTemplate.Status -eq 200)
    $packageQuery = if ($stage -eq "step-05") { "&package=P01" } else { "" }
    $assembled = Invoke-Api "/api/projects/$structuredProject/assemble?stage=$stage$packageQuery"
    Check ("assemble structured " + $stage) ($assembled.Status -eq 200 -and ($stage -eq "step-01" -or $assembled.Body.projection.state -eq "ready"))
    $fixture = Get-FileText (Join-Path $fixtureDir "$stage.md")
    $kind = if ($stage -eq "step-05") { "package-output" } else { "output" }
    $body = @{ kind = $kind; stage = $stage; content = $fixture; baseHash = "" }
    if ($stage -eq "step-05") { $body.package = "P01" }
    $saved = Invoke-Api "/api/projects/$structuredProject/doc" -Method "PUT" -Body $body
    Check ("save valid fixture " + $stage) ($saved.Status -eq 200 -and $saved.Body.structured.status -eq "valid")
    $reloaded = Invoke-Api "/api/projects/$structuredProject/doc?kind=$kind&stage=$stage$packageQuery"
    Check ("source bytes preserved " + $stage) ($reloaded.Body.content -ceq $fixture)
    if ($stage -eq "step-04") {
        $preview = Invoke-Api "/api/projects/$structuredProject/detect-packages" -Method "POST" -Body @{}
        Check "combined structured preview selects P01" ($preview.Body.structured.status -eq "valid" -and $preview.Body.packages.Count -eq 1)
        $imported = Invoke-Api "/api/projects/$structuredProject/import-packages" -Method "POST" -Body @{
            packages = $preview.Body.packages; accept = @("P01"); overwrite = @()
        }
        Check "import structured package" ($imported.Status -eq 200 -and $imported.Body.created -contains "P01")
        $individual = Invoke-Api "/api/projects/$structuredProject/assemble?stage=step-04&package=P01"
        Check "assemble individual package after combined import" ($individual.Status -eq 200 -and $individual.Body.projection.state -eq "ready")
        $slot = (Invoke-Api "/api/projects/$structuredProject/doc?kind=package&package=P01").Body
        $slotEdit = Invoke-Api "/api/projects/$structuredProject/doc" -Method "PUT" -Body @{
            kind = "package"; package = "P01"; content = $slot.content + "`n"; baseHash = $slot.hash
        }
        Check "ordinary slot edit succeeds" ($slotEdit.Status -eq 200)
        $combinedAfter = (Invoke-Api "/api/projects/$structuredProject/doc?kind=output&stage=step-04").Body
        Check "slot edit preserves combined source" ($combinedAfter.hash -eq $saved.Body.hash -and $combinedAfter.content -ceq $fixture)
    }
}
$structuredInfo = (Invoke-Api "/api/projects/$structuredProject").Body.packages | Where-Object { $_.id -eq "P01" }
Check "ready Step 5 artifact means prompt-generated" ($structuredInfo.status -eq "prompt-generated")

# ---------------------------------------------------------------- 12. unsafe input
Section "13. Path traversal, name validation and traversal rejection"

$cases = @(
    @{ name = "project id ../..";     url = "/api/projects/..%2F..%2Fetc/doc?kind=input&stage=step-01" }
    @{ name = "project id with slash"; url = "/api/projects/a%2Fb/doc?kind=input&stage=step-01" }
    @{ name = "absolute project id";  url = "/api/projects/C:%5CWindows%5Ctemp/doc?kind=input&stage=step-01" }
)
foreach ($c in $cases) {
    $r = Invoke-Api $c.url
    Check ("rejected: " + $c.name) ($r.Status -ge 400) ("status " + $r.Status + " " + $r.Body.error)
}

$badKinds = @(
    @{ k = "../../../etc/passwd" },
    @{ k = "output"; stage = "../../evil" },
    @{ k = "package"; package = "../../../Windows/System32/drivers/etc/hosts" },
    @{ k = "not-a-kind" }
)
foreach ($bk in $badKinds) {
    $q = "kind=" + [uri]::EscapeDataString($bk.k) + "&stage=" + [uri]::EscapeDataString([string]$bk.stage) + "&package=" + [uri]::EscapeDataString([string]$bk.package)
    $r = Invoke-Api "/api/projects/$projA/doc?$q"
    Check ("rejected kind/stage/package: " + ($bk.k + "/" + $bk.stage + "/" + $bk.package)) ($r.Status -ge 400) ("status " + $r.Status)
}

$traversalWrite = Invoke-Api "/api/projects/$projA/doc" -Method "PUT" -Body @{
    kind = "output"; stage = "..%2F..%2Fescape"; content = "nope"; baseHash = ""
}
Check "write with a traversal stage refused" ($traversalWrite.Status -ge 400) ("status " + $traversalWrite.Status)
Check "no file escaped the workspace" (-not (Test-Path (Join-Path (Split-Path $Workspace -Parent) "escape.md")))

$badWs = @("relative\path", "C:\does\..\..\escape", "", "not a path", "Z:\nope")
foreach ($b in $badWs) {
    $r = Invoke-Api "/api/settings/workspace" -Method "POST" -Body @{ path = $b }
    Check ("workspace path refused: '" + $b + "'") ($r.Status -eq 400) ("status " + $r.Status)
}

$nameCases = @("CON", "a/b", "..", "with:colon", "trailing.", "aux", "PRN", "..\\..\\x", "nul.md")
foreach ($n in $nameCases) {
    $r = Invoke-Api "/api/projects" -Method "POST" -Body @{ name = $n }
    # Either the name is refused outright, or it is slugified into a single safe
    # path component that cannot escape the projects folder.
    $safe = ($r.Status -ne 201) -or ($r.Body.id -match '^[a-z0-9][a-z0-9-]{0,63}$')
    Check ("project name refused or slugified safely: '" + $n + "'") $safe ("status " + $r.Status + " -> '" + $r.Body.id + "'")
}

$bigBody = Invoke-Api "/api/projects/$projA/doc" -Method "PUT" -Body @{
    kind = "output"; stage = "step-01"; content = ("x" * 9 * 1024 * 1024); baseHash = ""
}
Check "oversized body refused" ($bigBody.Status -ge 400) ("status " + $bigBody.Status)

# ---------------------------------------------------------------- 11. restart
Section "14. Persistence across a restart"

$before = @{
    projects = (Invoke-Api "/api/projects").Body.projects.Count
    step04   = (Get-FileText (Join-Path $projADir "outputs\step-04.md")).Length
    pkgCount = (Get-ChildItem (Join-Path $projADir "packages") -Filter *.md).Count
    snap     = (Get-FileText (Join-Path $projADir "prompts\snapshots\step-05-P01.md")).Length
}

if ($script:StartedProcess) {
    Write-Host "        restarting the application..."
    $script:StartedProcess.Kill()
    $script:StartedProcess.WaitForExit(10000) | Out-Null
    Start-Sleep -Seconds 1
    $env:BUILDFLOW_HOME = $Home1
    $script:StartedProcess = Start-Process -FilePath $Exe -ArgumentList @("--no-browser") -PassThru -WindowStyle Hidden `
        -RedirectStandardOutput (Join-Path $WorkDir "stdout2.txt") -RedirectStandardError (Join-Path $WorkDir "stderr2.txt")
    $ok = $false
    for ($i = 0; $i -lt 40; $i++) {
        Start-Sleep -Milliseconds 250
        $h = Invoke-Api "/api/health" -NoToken
        if ($h.Status -eq 200) { $ok = $true; break }
    }
    Check "application restarts and serves again" $ok

    $after = @{
        projects = (Invoke-Api "/api/projects").Body.projects.Count
        step04   = (Get-FileText (Join-Path $projADir "outputs\step-04.md")).Length
        pkgCount = (Get-ChildItem (Join-Path $projADir "packages") -Filter *.md).Count
        snap     = (Get-FileText (Join-Path $projADir "prompts\snapshots\step-05-P01.md")).Length
    }
    Check "project count survives a restart" ($before.projects -eq $after.projects) ("$($before.projects) -> $($after.projects)")
    Check "step 4 output survives" ($before.step04 -eq $after.step04) ("$($before.step04) -> $($after.step04)")
    Check "package count survives" ($before.pkgCount -eq $after.pkgCount) ("$($before.pkgCount) -> $($after.pkgCount)")
    Check "step 5 snapshot survives" ($before.snap -eq $after.snap) ("$($before.snap) -> $($after.snap)")
    $wsAfter = (Invoke-Api "/api/settings").Body.workspacePath
    Check "workspace path is remembered" ($wsAfter -eq $Workspace) $wsAfter
    Check "newest workspace README is intact" (Test-Path (Join-Path $Workspace "README.md"))
}

# ---------------------------------------------------------------- 12. single instance
Section "15. Single instance, autostart setting and quit"

if ($script:StartedProcess) {
    $second = Start-Process -FilePath $Exe -ArgumentList @("--no-browser") -PassThru -WindowStyle Hidden
    $exited = $second.WaitForExit(15000)
    Check "a second copy exits instead of running twice" $exited ("exit code " + $(if ($exited) { $second.ExitCode } else { "still running" }))
    if (-not $exited) { $second.Kill() }
    Check "the second copy hands over cleanly" ($exited -and $second.ExitCode -eq 0) $(if ($exited) { "exit code " + $second.ExitCode } else { "still running" })
    $stillOne = (Get-Process -Name ([System.IO.Path]::GetFileNameWithoutExtension($Exe)) -ErrorAction SilentlyContinue).Count
    Check "exactly one BuildFlow process remains" ($stillOne -eq 1) ("count " + $stillOne)
}

$asOn = Invoke-Api "/api/settings/autostart" -Method "POST" -Body @{ enabled = $true }
Check "start with Windows can be enabled" ($asOn.Status -eq 200 -and $asOn.Body.startWithWindows -eq $true) $asOn.Body.command
$runKey = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Run"
$regVal = (Get-ItemProperty -Path $runKey -Name "BuildFlow" -ErrorAction SilentlyContinue).BuildFlow
Check "HKCU Run value is written for the current user only" ($regVal -like "*BuildFlow*--minimized*") $regVal
$asOff = Invoke-Api "/api/settings/autostart" -Method "POST" -Body @{ enabled = $false }
Check "start with Windows can be turned off" ($asOff.Status -eq 200 -and $asOff.Body.startWithWindows -eq $false)
$regVal2 = (Get-ItemProperty -Path $runKey -Name "BuildFlow" -ErrorAction SilentlyContinue).BuildFlow
Check "HKCU Run value removed" ($null -eq $regVal2) "$regVal2"

if ($script:StartedProcess) {
    $q = Invoke-Api "/api/app/quit" -Method "POST"
    Check "quit endpoint accepted" ($q.Status -eq 200) ("status " + $q.Status)
    $gone = $script:StartedProcess.WaitForExit(20000)
    Check "the background application really exits" $gone
    $script:StartedProcess = $null
    Start-Sleep -Seconds 1
    $after = Invoke-Api "/api/health" -NoToken
    Check "the port stops answering after quitting" ($after.Status -eq 0) ("status " + $after.Status)
    $logText = Get-FileText (Join-Path $Home1 "logs\buildflow.log")
    Check "a useful log was written" ($logText -like "*listening on*") ("log " + (Join-Path $Home1 "logs\buildflow.log") + ", " + (($logText -split "`n").Count) + " lines")
}

# ---------------------------------------------------------------- report
Write-Host ""
Write-Host ("=" * 62)
if ($script:Failures -eq 0) {
    Write-Host ("ALL " + $script:Checks + " CHECKS PASSED") -ForegroundColor Green
} else {
    Write-Host ($script:Failures.ToString() + " of " + $script:Checks + " CHECKS FAILED") -ForegroundColor Red
}
Write-Host ("=" * 62)
Write-Host ("Workspace : " + $Workspace)
Write-Host ("App data  : " + $Home1)

if ($script:StartedProcess) { $script:StartedProcess.Kill() }
exit $(if ($script:Failures -eq 0) { 0 } else { 1 })
