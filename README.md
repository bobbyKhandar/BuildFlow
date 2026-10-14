# BuildFlow

A local Windows workbench for a **manual, prompt-driven AI build workflow**.

You write a brief, BuildFlow assembles a complete prompt from everything you
have saved so far, you paste that prompt into whatever AI chatbot you use, and
you paste the answer back. Step 2.5 pins down the stack and a *reusable system
context*. Step 4 splits the design into work packages. Step 5 then produces one
self-contained coding prompt at a time, for exactly one package, with that
system context attached.

Everything is stored as plain UTF-8 Markdown in a folder **you** choose, so the
same files open cleanly in Obsidian. There is no database, no cloud account and
no AI API call anywhere in the program. BuildFlow never talks to a model; it
only organises your text.

- One `.exe`. No Go, Node, database or web server needed to run it.
- A local web interface at **<http://127.0.0.1:8765>**, bound to loopback only.
- Keeps running in the notification area when you close the browser tab.

---

## Contents

1. [Install](#install)
2. [First run and choosing a workspace](#first-run-and-choosing-a-workspace)
3. [Opening BuildFlow again](#opening-buildflow-again)
4. [Quitting](#quitting)
5. [Starting with Windows](#starting-with-windows)
6. [The workflow](#the-workflow)
7. [Workspace layout](#workspace-layout)
8. [Prompt templates: global and local](#prompt-templates-global-and-local)
9. [Prompt snapshots](#prompt-snapshots)
10. [Working in Obsidian at the same time](#working-in-obsidian-at-the-same-time)
11. [Backup and restore](#backup-and-restore)
12. [Using the REST API from a script](#using-the-rest-api-from-a-script)
13. [Security model](#security-model)
14. [Where files are kept](#where-files-are-kept)
15. [Command line flags](#command-line-flags)
16. [Building from source](#building-from-source)
17. [Verification](#verification)
18. [Known limitations](#known-limitations)

---

## Install

Copy `BuildFlow.exe` anywhere you like - it is self-contained. A sensible spot
is `C:\Program Files\BuildFlow\` or a folder in your user profile. Nothing is
installed system-wide and no administrator rights are needed.

To also get the "Open BuildFlow" entry in the notification area working the way
described below, keep the `.exe` at a **fixed path** and don't move it
afterwards: the sign-in shortcut records that path.

### If Windows blocks the file

Windows 11 Smart App Control refuses to launch unsigned programs, and a
freshly built `.exe` is exactly that:

```
Program 'BuildFlow.exe' failed to run: An Application Control policy has
blocked this file.
```

This is not a defect in BuildFlow - it happens with any self-built program. Two
ways through it:

- **Build it yourself** and let the build script sign it:

  ```powershell
  powershell -ExecutionPolicy Bypass -File build.ps1
  ```

  `build.ps1` creates a code signing certificate in your own user profile,
  trusts it for your user account only, and signs both executables. No
  administrator rights, no internet, and the certificate is reused on later
  builds. See [scripts/sign.ps1](scripts/sign.ps1) and
  [BUILD.md](BUILD.md#code-signing).

- **Or** right-click the `.exe` → Properties → **Unblock** at the bottom, if
  your Windows shows that checkbox. If Smart App Control still refuses, see
  Microsoft Support on turning it off for a program you trust.

---

## First run and choosing a workspace

Start `BuildFlow.exe`. It:

1. creates `%LOCALAPPDATA%\BuildFlow\` for settings, logs and the API token,
2. starts a web server on `127.0.0.1:8765` and nothing else,
3. shows a tray icon in the notification area,
4. opens <http://127.0.0.1:8765> in your default browser.

On the first visit the page asks for a **workspace folder** - the place your
Markdown lives. Use **Browse** for the native folder picker, or type a path such
as `D:\Obsidian\BuildFlow`. BuildFlow creates the folder if it does not exist.

### Pointing it at an Obsidian vault

Any folder works. Two options:

- **Dedicated folder inside the vault.** Choose
  `D:\MyVault\BuildFlow`. Your notes, the vault and the BuildFlow workspace all
  live side by side. This is usually what you want.
- **The vault root itself.** Choose `D:\MyVault`. BuildFlow adds
  `global-prompts/` and `projects/` to it and never touches anything else. It
  also drops a `README.md` and a small `.gitignore` if they are not already
  there, so commit the vault with those ignored if you version it.

Notes:

- An **existing** folder is used as-is. BuildFlow never deletes or overwrites
  files it did not create; it fills in missing folders and prompts, and asks
  before touching anything that already has content in it.
- Pick a folder that is not inside `C:\Program Files`, otherwise Windows
  permissions will stop BuildFlow from writing.
- You can change workspace later in **Settings**. The old folder is left
  untouched.

## Opening BuildFlow again

While it is running, any of these opens the interface in your browser:

- the **notification area icon → Open BuildFlow**,
- **Start → Open BuildFlow** if you pinned it, or the desktop shortcut,
- double-clicking `BuildFlow.exe` again - a second copy opens the browser on
  the running instance and then exits, so you never get two servers,
- or just visit <http://127.0.0.1:8765> yourself.

Double-clicking twice in quick succession is safe. The second copy waits for the
first to finish starting, retries the port for a few seconds, and hands over
quietly instead of reporting a conflict you did not cause. The
"BuildFlow cannot start" dialog only appears when something *other* than
BuildFlow genuinely holds port 8765 after that wait.

**Closing the browser tab does not stop BuildFlow.** The tab is only a view. The
tray icon and the local server keep running, and reopening the URL shows the
same state.

## Quitting

Any of these stops the application and frees the port:

- the **notification area icon → Quit BuildFlow**,
- the **Quit** button on the Settings page of the web interface,
- `POST /api/app/quit` (see [the API section](#using-the-rest-api-from-a-script)),
- closing a console build with `Ctrl+C`.

Quitting flushes settings and the log. Your workspace is never modified on quit.

## Starting with Windows

**Settings → Start with Windows**, or the tray menu item. It writes a single
value for the current user:

```
HKCU\Software\Microsoft\Windows\CurrentVersion\Run
    BuildFlow = "<full path to BuildFlow.exe>" --minimized
```

- Per user, so no administrator rights and nothing visible to other accounts.
- BuildFlow is a normal program, **not a Windows service**. There is no service,
  no driver and no scheduled task.
- `--minimized` means it starts in the notification area without opening a
  browser, so signing in does not hijack your screen.
- Turn it off the same way. The checkbox reflects the real registry state, and
  the tray menu item toggles the same setting.

If you move `BuildFlow.exe`, switch it off and on again so the recorded path is
correct.

---

## The workflow

Six stages. The left-hand navigator shows each one, what it needs, and whether
it has been filled in.

| Step | Produces | Needs from earlier steps |
| --- | --- | --- |
| **1** Research brief | A researched, written brief | your input |
| **2** Decision and high-level design | One decision and a high-level design | Step 1 |
| **2.5** Stack, cost and system context | Chosen technologies, packages, estimated cost, and the **reusable system context** | Step 2 |
| **3** Low-level design | Detailed design: modules, interfaces, data, error handling | Step 2 **and** Step 2.5 |
| **4** Work packages | Several independently buildable work packages | Step 3 |
| **5** Package coding prompt | One coding prompt for **one** package, plus the Step 2.5 system context | the package, and Step 2.5 |

For each stage the page has separate areas:

- **Input** - what you paste in for this step.
- **Prompt** - the editable template, with `{{placeholders}}` you can insert by
  clicking them.
- **Output** - what the AI gave back.
- **Assembled prompt** - the finished text, with a preview, a copy button and
  the list of every document that fed into it.

The assembled prompt can be edited before you copy it. Nothing is written back
into your template by editing the preview, so you always know which is which.

### Missing prerequisites

If a stage needs something you have not saved yet, BuildFlow says so in three
places: a warning next to the stage, a list under the preview, and a marked
line inside the prompt text itself:

```
- **Step 2 output (decision and high-level design)** -
  projects/my-app/outputs/step-02.md (not saved yet). Save Step 2's output first.
```

Optional inputs (the step's own input notes, a package's input notes) are
marked as optional and never block you. Under the preview they are listed
separately as "Worth knowing" so a real gap is never buried among advice.

Each entry in the `warnings` array of the assembly response carries
`required` (does this gap block the step?), `severity` (`missing`, `empty` or
`note`) and a `hint` telling you what to do. Treat `required` as authoritative;
`severity: "note"` entries are advice, such as an unknown `{{placeholder}}` in a
template, and never block even though they are worth acting on.

### Output flows into the next step's input

When you save an output, BuildFlow automatically copies it into the next step's
input, so each step starts with the previous result in front of it:

```
My own notes for this step.


<!-- buildflow:auto:step-01 -->
# Research brief

The AI researched the topic.
<!-- /buildflow:auto:step-01 -->
```

- Your own notes stay at the top. The copied block is appended below, separated
  by three blank lines.
- The block is wrapped in a marker comment, so the copy is always visible,
  never mysterious. Delete it and it stays gone until you save that output again.
- Saving the output again **replaces** the block instead of appending a second
  one, so re-running a step never duplicates context.
- The assembled prompt uses your notes for `{{input}}` and the previous output
  through the dedicated `{{output_step_XX}}` tokens, so nothing is duplicated
  inside the prompt.
- The previous output is **also** placed at the very bottom of the assembled
  prompt under a `## Previous step output` heading (via the `{{previous_output}}`
  token), so selecting all and copying gives you a self-contained prompt to
  paste straight into a chatbot. For Step 5 the bottom block is the package itself.

The chain runs Step 1 → 2 → 2.5 → 3 → 4. Step 4 is different: its output is the
work packages themselves, and each one is copied into its own Step 5 input (see
below).

### Step 4: work packages as you go

Step 4 shows one slot per work package, and always one more than you have filled
in — the extra empty slot is how you add the next package:

- **Notes (input)** — your notes for that package, saved to
  `inputs/step-04/P01.md`.
- **Work package (output)** — the package content. Saving it creates
  `packages/P01.md` (with Obsidian front matter) **and** copies it into that
  package's Step 5 input, so Step 5 starts with the package already in front of
  you.

A slot counts as filled once it holds anything — notes or a package — so saving
a package also opens the next empty slot. The single `outputs/step-04.md` file,
the "Split into work packages" detector and "Add next package" all still work
for existing projects.

### Step 4: splitting into packages

Put your packages in the Step 4 output as headings:

```markdown
## P01 - Local REST API
Deliver the loopback server and routing.
- Files: internal/server/*
- Tests: internal/server/server_test.go

## P02 - Markdown document store
Safe writes, conflict detection, package detection.
```

**Detect packages** finds `## P01 - Title` headings and shows you a preview
first. It will not touch a file you have already edited unless you tick it
explicitly. `###` sub-headings are kept as part of the package body rather than
becoming packages of their own, and any text above the first package heading is
reported separately instead of being silently dropped.

You can always skip detection and create packages by hand.

### Step 5: one package at a time

Each package gets its own row with its own status, and **Step 5 assembles only
the package you selected**. A dropdown at the top of the step lists every package
with its title and status; picking one shows that package's input, prompt, output
and notes. The sidebar package list mirrors the same selection.

```
prompts/snapshots/step-05-P01.md   the exact prompt used for P01
inputs/step-05/P01.md              your notes for the P01 session
outputs/step-05/P01.md             what the AI delivered for P01
packages/P01.md                    the package itself
```

So P01 and P03 are entirely separate records - separate prompt, separate input
notes, separate output, separate status. **Add next package** creates the next
number after the highest one you have (N → N+1), and is always offered, whether
or not the previous package is finished.

Status is derived from what is on disk, not from a checkbox you can forget:

| Status | Meaning |
| --- | --- |
| `new` | the package exists but has no prompt snapshot yet |
| `ready` | a prompt snapshot exists, no output yet |
| `in progress` | there is a snapshot and an output, and the output is newer |
| `done` | the output has not changed since the snapshot |
| `stale` | the package file changed after the snapshot was taken |

---

## Workspace layout

Everything BuildFlow stores is a Markdown file. Only `project.json` is JSON, and
only because a few dozen bytes of metadata do not belong in a note.

```text
<your workspace>/
  README.md                        written on first use, explains the layout
  .gitignore                       keeps Obsidian's own files out of git
  global-prompts/
    step-01.md   step-02.md   step-02-5.md
    step-03.md   step-04.md   step-05.md
  projects/
    <project-name>/
      project.json                 id, name, description, timestamps
      README.md
      inputs/
        step-01.md  step-02.md  step-02-5.md  step-03.md  step-04.md
        step-05/
          P01.md  P02.md ...      your notes for each coding session
      outputs/
        step-01.md  step-02.md  step-02-5.md  step-03.md  step-04.md
        step-05/
          P01.md  P02.md ...      what the AI delivered for each package
      packages/
        P01.md  P02.md ...         work packages, with YAML front matter
      prompts/
        local-overrides/           prompt templates for this project only
          step-01.md ... step-05.md
        snapshots/                 the exact prompt used, per step and package
          step-01.md ... step-05.md
          step-05-P01.md step-05-P02.md ...
```

- **Every direct child folder of `projects/` is a project.** Drop a folder in
  there by hand, or copy a project in from elsewhere, and it appears in the
  project list. `project.json` is optional; BuildFlow falls back to the folder
  name if it is missing, and it is never required to read the project.
- Inputs and outputs are deliberately **separate files**. A step's answer never
  overwrites the notes you fed in.
- Every file is UTF-8. Writes are atomic: content goes to a temporary file in
  the same folder and is then renamed into place, so an interrupted save cannot
  leave half a file behind.
- One file in the workspace is yours: the workspace `README.md` is only written
  if you do not already have one.

## Prompt templates: global and local

Each stage has one editable template. Two scopes:

- **Global** - `global-prompts/step-0X.md`, shared by every project. Edit it to
  change how BuildFlow prompts for every project on this machine.
- **Local override** - `projects/<project>/prompts/local-overrides/step-0X.md`,
  used for that project only. Your global templates are never touched.

The badge above the template editor always says which one is active
(`global template`, `local override`, or `built-in starter`), and the hint line
tells you whether it is unmodified, edited, or when it was last saved.
**Revert to global** deletes the local override after asking.

### Placeholders

Click one in the editor to append it. BuildFlow substitutes them when it
assembles the prompt.

| Placeholder | Filled with |
| --- | --- |
| `{{project_name}}` `{{project_id}}` `{{project_description}}` | project metadata |
| `{{stage}}` `{{stage_num}}` `{{stage_title}}` | which step you are on |
| `{{today}}` `{{generated_at}}` | date and timestamp |
| `{{input}}` | this step's input notes |
| `{{output_step_01}}` … `{{output_step_04}}` | a saved output of any step |
| `{{previous_output}}` | the previous step's output, for the bottom of the prompt |
| `{{system_context}}` | the *Reusable System Context* section of Step 2.5 |
| `{{package_id}}` `{{package_title}}` | which package Step 5 is for |
| `{{package_content}}` | that package's file |
| `{{package_input}}` | your notes for that package |
| `{{missing_sources}}` | a list of what is still missing, or a note that nothing is |

A placeholder BuildFlow does not recognise is left exactly as written and
reported as a warning, rather than being silently emptied.

All six stages ship with complete, written-out starter prompts, so the app is
useful the first time you open it.

## Prompt snapshots

Before you copy an assembled prompt, **Save prompt snapshot** records the exact
text - including any edits you made to the preview - in
`prompts/snapshots/`. This is your record of what was actually sent, per step
and per package.

Snapshots are compared against a fresh assembly, so the interface can tell you
"this snapshot no longer matches the current inputs" and offer to update it.
A snapshot is never overwritten without you saying so.

## Working in Obsidian at the same time

This is supported, and BuildFlow is careful about it.

Every save is checked against the file on disk first. BuildFlow remembers the
SHA-256 of what it loaded, and refuses the write if the file changed in the
meantime. When that happens you get a conflict notice that shows you the
newer text from disk, and **your editor keeps everything you typed**. You then
choose one of:

- **Reload from disk** - take the Obsidian version, discarding your text.
- **Keep mine** - overwrite, having seen what you were overwriting.
- **Copy my text** - put your version on the clipboard so you can merge it by
  hand.

The same protection applies to prompt templates, work packages and snapshots.
BuildFlow never silently overwrites something it did not write, and it never
loses your text to a failed write.

**Obsidian setup tip:** open the workspace folder as a vault. `.gitignore` in
the workspace keeps `.obsidian/` noise out of version control. Files BuildFlow
writes end in `\n` and are UTF-8, which Obsidian handles cleanly.

## Backup and restore

Your data is files, so backup is copying files.

**To back up, copy the workspace folder.** That is the whole of your content -
every brief, design, package, prompt and answer. Nothing else is needed to move
BuildFlow to another machine.

Settings, logs and the API token live separately in
`%LOCALAPPDATA%\BuildFlow\`. Back that up too if you want the chosen workspace
path, the sign-in setting and the token to come with you.

**To restore**, copy the workspace folder back and start BuildFlow, then set the
workspace path in **Settings**. Projects appear from the folder contents; you do
not have to re-import anything.

Two things to know:

- Never restore a workspace while BuildFlow is running. Quit first.
- Back up the folder **not** the individual files, so a save that happens
  mid-copy cannot produce a half-written note. Atomic writes mean you will
  never see a truncated file, but a consistent snapshot is still nicer.

## Using the REST API from a script

BuildFlow exposes a local REST API on the same loopback port. It is meant for
scripts **you** run, on your own machine.

### Get the token

On first start BuildFlow writes a random 256-bit token to:

```
%LOCALAPPDATA%\BuildFlow\api-token.txt
```

The file is mostly comments; the token is the one line that does not begin with
`#`:

```powershell
$token = @(Get-Content "$env:LOCALAPPDATA\BuildFlow\api-token.txt" |
           ForEach-Object { $_.Trim() } |
           Where-Object { $_ -and -not $_.StartsWith("#") })[0]
```

```bash
TOKEN=$(grep -v '^#' "$LOCALAPPDATA/BuildFlow/api-token.txt" | grep -v '^$')
```

Send it in the `X-BuildFlow-Token` header on every `/api/` request. The one
exception is `GET /api/health`, which is unauthenticated on purpose so a second
copy of the app can check whether an instance is already running.

> This file is a secret. Anything that can read it can read and change your
> projects. Do not put it in a shared folder, and do not paste it anywhere.

### Examples

```bash
curl -H "X-BuildFlow-Token: $TOKEN" http://127.0.0.1:8765/api/health

# list projects
curl -H "X-BuildFlow-Token: $TOKEN" http://127.0.0.1:8765/api/projects

# create a project
curl -X POST -H "X-BuildFlow-Token: $TOKEN" -H "Content-Type: application/json" \
     -d '{"name":"Storefront","description":"local inventory"}' \
     http://127.0.0.1:8765/api/projects

# read a document (kind, stage, package - never a path)
curl -H "X-BuildFlow-Token: $TOKEN" \
     "http://127.0.0.1:8765/api/projects/storefront/doc?kind=input&stage=step-01"

# save a document. baseHash is the hash you read; "" only works on a new file.
curl -X PUT -H "X-BuildFlow-Token: $TOKEN" -H "Content-Type: application/json" \
     -d '{"kind":"input","stage":"step-01","content":"# Use case\n","baseHash":""}' \
     http://127.0.0.1:8765/api/projects/storefront/doc

# assemble the coding prompt for one package
curl -H "X-BuildFlow-Token: $TOKEN" \
     "http://127.0.0.1:8765/api/projects/storefront/assemble?stage=step-05&package=P01"

# read the active prompt template
curl -H "X-BuildFlow-Token: $TOKEN" \
     "http://127.0.0.1:8765/api/projects/storefront/template?stage=step-01&scope=local"

# detect work packages in the step 4 output (previews, writes nothing)
curl -X POST -H "X-BuildFlow-Token: $TOKEN" -H "Content-Type: application/json" \
     -d '{}' http://127.0.0.1:8765/api/projects/storefront/detect-packages

# create the next package
curl -X POST -H "X-BuildFlow-Token: $TOKEN" -H "Content-Type: application/json" \
     -d '{"title":"Documentation"}' \
     http://127.0.0.1:8765/api/projects/storefront/packages

# quit
curl -X POST -H "X-BuildFlow-Token: $TOKEN" http://127.0.0.1:8765/api/app/quit
```

### Endpoints

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/api/health` | liveness, version, port. No token. |
| GET | `/api/diagnostics` | paths, settings, workspace status |
| GET | `/api/settings` | current settings |
| PUT | `/api/settings` | update settings |
| POST | `/api/settings/workspace` | set the workspace folder |
| GET | `/api/settings/workspace/validate?path=` | check a path before using it |
| POST | `/api/settings/workspace/pick` | open the native folder picker |
| POST | `/api/settings/autostart` | turn sign-in startup on or off |
| GET | `/api/stages` | the six workflow stages |
| GET | `/api/templates` | summary of every template |
| GET | `/api/projects` | list projects |
| POST | `/api/projects` | create a project |
| GET | `/api/projects/{id}` | one project: steps and packages |
| PATCH | `/api/projects/{id}` | rename or re-describe |
| DELETE | `/api/projects/{id}?confirm={id}` | delete a project |
| GET | `/api/projects/{id}/doc` | read a document |
| PUT | `/api/projects/{id}/doc` | save a document |
| DELETE | `/api/projects/{id}/doc` | delete a document |
| GET | `/api/projects/{id}/template` | read the active template |
| PUT | `/api/projects/{id}/template` | save a global or local template |
| DELETE | `/api/projects/{id}/template` | revert a local override |
| GET | `/api/projects/{id}/assemble` | build the prompt for a step or package |
| POST | `/api/projects/{id}/detect-packages` | preview package splits |
| POST | `/api/projects/{id}/import-packages` | write the accepted packages |
| POST | `/api/projects/{id}/packages` | add the next package |
| POST | `/api/projects/{id}/snapshot` | save the exact prompt used |
| POST | `/api/app/open` | open the interface in a browser |
| POST | `/api/app/copy` | copy text to the clipboard |
| POST | `/api/app/quit` | shut BuildFlow down |

### The one rule: send kinds, not paths

There is no endpoint that takes a filesystem path. You pass a **kind** plus
identifiers and the server resolves them to one of a fixed set of locations:

| `kind` | Goes to |
| --- | --- |
| `input` | `projects/<p>/inputs/step-<stage>.md` |
| `output` | `projects/<p>/outputs/step-<stage>.md` |
| `system-context` | the `## Reusable System Context` section of `projects/<p>/outputs/step-02-5.md` |
| `package` | `projects/<p>/packages/<package>.md` |
| `package-input` | `projects/<p>/inputs/step-05/<package>.md` |
| `package-output` | `projects/<p>/outputs/step-05/<package>.md` |
| `snapshot` | `projects/<p>/prompts/snapshots/step-<stage>.md` |
| `package-snapshot` | `projects/<p>/prompts/snapshots/step-05-<package>.md` |
| `template-global` | `global-prompts/step-<stage>.md` |
| `template-local` | `projects/<p>/prompts/local-overrides/step-<stage>.md` |

`stage` is one of `step-01`, `step-02`, `step-02-5`, `step-03`, `step-04`,
`step-05`. `package` is `P01` style. Anything else is rejected with `400`.

Two details worth knowing:

- `system-context` may only be requested at `step-02-5`. The reusable context is
  a section of that step's output rather than a file of its own, so any other
  stage is refused with `400` instead of quietly reading something else.
- List fields in a response are always JSON arrays, never `null`. An import that
  changed nothing answers `"created": []`, so you can read `result.created.length`
  without a null check.

This is why path traversal is not merely blocked but impossible: a request
literally cannot express a location outside those templates.

### Saving safely from a script

Always read first and send the `hash` you got back as `baseHash`:

```jsonc
// GET /api/projects/storefront/doc?kind=output&stage=step-01
{ "exists": true, "content": "...", "hash": "3f9a...", "path": "projects/storefront/outputs/step-01.md" }

// PUT /api/projects/storefront/doc
{ "kind": "output", "stage": "step-01", "content": "...", "baseHash": "3f9a..." }
```

A `409` means the file changed since you read it. The response carries the
current disk contents in `details` so you can merge and retry. Send
`"force": true` to overwrite deliberately. If you omit `baseHash` and the file
already exists, BuildFlow answers `409` rather than guessing.

## Security model

The threat is a web page you visit, not a network attacker - so the design is
about making a hostile page unable to reach this one.

| Control | What it does |
| --- | --- |
| **Loopback only** | The listener binds `127.0.0.1:8765` on IPv4 specifically. Never `0.0.0.0`, never a LAN address. Verified with `Get-NetTCPConnection`. |
| **Not reachable from the network** | There is no UPnP, no port forwarding, no LAN discovery, no external callback. Another machine cannot connect even on the same Wi-Fi. |
| **Host header allowlist** | Requests whose `Host` is not a loopback name are refused. This blocks DNS-rebinding, where a hostile page resolves its own name to `127.0.0.1` and then talks to BuildFlow from your browser. |
| **Origin allowlist** | A cross-origin `Origin` is refused with `403` before the handler runs. |
| **`Sec-Fetch-Site`** | `cross-site` and `none` requests are refused outright. |
| **No CORS** | BuildFlow never sends `Access-Control-Allow-Origin`. A page on another origin cannot read any response. |
| **Token on every `/api/` call** | A 256-bit random token, compared in constant time. It is injected into the page from the server rather than stored in the browser. |
| **CSP** | `default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'`. No inline script, no remote anything. |
| **Structured API** | No endpoint accepts a path. Kinds, stages and package ids are validated against fixed patterns. |
| **Traversal and link defences** | Every resolved path is confirmed to be inside the workspace, and symlinks or junctions found inside it are refused. |
| **Writes are atomic** | Temp file plus rename, so a crash or a full disk cannot truncate your notes. |
| **Bounded input** | 8 MiB request body, 8 MiB per document, 400 000 characters per template. |

**What is not protected against:** another program running as **you** can read
`api-token.txt` and therefore control BuildFlow. That is inherent - the token
has to be readable by something - so treat the file as you would a saved
password. It is not meant for shared machines, and the port is a single-user
local service, not a hardened multi-user daemon.

## Where files are kept

All per user, no administrator rights:

```text
%LOCALAPPDATA%\BuildFlow\
  config.json        settings: port, workspace path, sign-in preference
  api-token.txt      the local REST API token
  logs\
    buildflow.log    rotating, 2 MiB per file
```

Override the whole folder with the `BUILDFLOW_HOME` environment variable, which
is handy for running a throwaway instance:

```powershell
$env:BUILDFLOW_HOME = "$env:TEMP\bf-test"
.\BuildFlow.exe --workspace "$env:TEMP\bf-ws" --no-browser
```

If `config.json` is ever unreadable or corrupt, BuildFlow moves it aside as
`config.json.bak` and starts from defaults rather than refusing to start. The
log always says which files it is using; **Settings** shows all of them.

## Command line flags

```
BuildFlow.exe [options]

  --console        attach to the parent console and echo logs
  --minimized      start in the notification area without opening a browser
  --no-browser     do not open a browser on start
  --port <n>       use a different loopback port (default 8765)
  --workspace <p>  use this workspace folder on start
  --version        print the version and exit
  --help           full usage, including the data locations
```

The port is persisted, so `--port 8765` once is enough.

If the port is unusable, BuildFlow says so in a dialog, names the port and the
config file to change, and writes the reason to the log. A second copy of
BuildFlow never fails this way: it detects the running instance, opens its
browser and exits quietly.

## Building from source

See **[BUILD.md](BUILD.md)**. In short:

```powershell
powershell -ExecutionPolicy Bypass -File build.ps1
```

Go 1.24 or newer is the only requirement, and only on the build machine. The
result is a self-contained `.exe` with no runtime dependencies - standard
library only, no third-party modules.

## Verification

```powershell
powershell -ExecutionPolicy Bypass -File scripts\verify.ps1
```

This starts a private instance in a temporary folder - your real workspace and
`%LOCALAPPDATA%\BuildFlow` are never touched - and drives it over HTTP through
201 checks, covering loopback-only binding, token and origin rejection, project
create/rename/switch, saves and byte-for-byte reloads, external-edit conflicts,
global versus local templates, assembly and its prerequisites, snapshots,
package detection and import, N+1 creation, per-package Step 5 records, path
traversal rejection, persistence across a restart, single-instance enforcement,
the sign-in setting and a clean quit.

The suite is the safety net, not a substitute for using the app. The browser
workbench was also driven end to end by hand: creating a project, saving every
step, detecting and importing packages, building a Step 5 prompt, taking a
snapshot, adding the next package, and provoking a real external-edit conflict
by editing a file underneath a running instance.

## Known limitations

Honest list of what is not there:

- **Windows only.** The tray icon, native folder picker, single-instance mutex
  and sign-in shortcut all use Win32 directly. There is no build for macOS or
  Linux, and no plan for one.
- **The folder picker is the old shell dialog** (`SHBrowseForFolder`), not the
  modern one. It works, but it looks like Windows 7 and it does not let you type
  a new folder name into it. Typing the path into the Settings box works fine
  and is the recommended route.
- **No AI integration at all**, by design. You paste prompts into your chatbot
  and paste the answers back. There is no model call, no API key, no cost.
- **Not a multi-user application.** One workspace, one token, one user. It is
  not hardened for a shared machine.
- **No search across a workspace** in the UI. Obsidian is better at that, which
  is the point of storing plain Markdown.
- **No project export or import as an archive.** Copy the folder. If you want
  one, an "export project as a folder" button is the obvious addition.
- **Package detection is heading-based**, not semantic. It finds
  `## P01 - Title` and similar shapes. It cannot tell you that P03 is too big
  or that P02 and P04 overlap. Always read the preview before importing.
- **The Step 2.5 system context depends on a heading.** BuildFlow looks for a
  `## Reusable System Context` (or `## System Context`) section and takes
  everything under it up to the next heading of the same level. With no such
  heading it falls back to the whole document and says so.
- **No undo across restarts.** Within a session, a refused write keeps your
  text. Across sessions, the file on disk is the truth.
- **The token cannot be rotated from the UI.** Delete `api-token.txt` and
  restart BuildFlow to get a new one; any script using the old one stops
  working.
- **Not tested on Windows 10.** Development and the whole verification suite ran
  on Windows 11. The APIs used (`Shell_NotifyIconW`, `RtlGetVersion`,
  `SHBrowseForFolderW`, the `HKCU\...\Run` key) have been stable since Windows 7
  and 8 respectively, and BuildFlow warns at startup on anything older than
  Windows 10, but Windows 10 is unverified.
