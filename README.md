# BuildFlow

**Keep track of every project, through every phase of building with AI.**

When you work on several projects at once, each one leaves you with a different
set of things to remember. One is still in research. Another has a design but
needs its technology choices settled. A third is already being built, with
several work packages and answers spread across AI conversations.

Coming back to a project means finding your place again: what you decided,
what you have saved, what is still missing and what the next session needs.
That gets harder every time you switch projects or start a fresh chat.

BuildFlow gives each project a place to keep that context. It is a local
Windows and Linux (x86_64) workbench that organises your briefs, designs, technology decisions,
work packages, prompts and AI answers by phase. You can move between projects
and return to the saved record of each one, instead of relying on memory or
scrolling through old conversations.

## A place for every project and every phase

Each project follows the same six stages: research, high-level design, stack
and system context, low-level design, work packages and package coding prompts.
The structure is familiar every time you switch projects, while the notes,
decisions and outputs belong to the project you selected.

Open a project and its stage navigator shows where inputs, outputs and prompt
snapshots have been saved. Go into a phase to read the original notes, inspect
the answer and see the prompt behind the work. If the next prompt needs an
earlier decision that is missing, BuildFlow points out the gap. You have a
record to check before continuing.

As a project moves into implementation, each work package gets its own notes,
prompt snapshot and output. You can return to one package without mixing its
conversation with the others. The saved records help you see where you left
off; you still review the results and verify the code in your development
tools.

## Pick up the context when you switch back

Imagine spending the morning researching a client portal, the afternoon
planning an inventory app and the next day working on an internal reporting
tool. By the time you return to the portal, the details are no longer fresh.

In BuildFlow, you select the portal, open its research phase and read the
brief and answer you saved. When you are ready to make the design decisions,
BuildFlow assembles the relevant saved material into the next prompt. You can
review and edit it before sending it to your AI tool.

Later, when that project has work packages, Step 5 combines the selected
package and its notes with the effective implementation context from Step 3.
A fresh coding session receives the saved stack, conventions and
constraints for that project. Your inventory app keeps its own context and
package records, ready for when you switch back to it.

The stage workflow stays simple: **write → assemble → copy → ask your AI →
paste the answer back → save**. Settings now let you choose Groq Free mode or a
paid provider and model, and the local API has a provider-neutral chat endpoint
for BuildFlow's upcoming guided planning flow. The existing stage controls still
use copy and paste. Provider charges, quotas and data policies apply to every
request sent with your key.

## A routine you can reuse across projects

You can keep a shared set of prompt templates for the way you like to work,
then override a template for a project with different needs. A useful research
or design prompt can serve the next project too, while each project's inputs
and answers stay separate. Starter templates give you somewhere to begin.

All of this lives as plain UTF-8 Markdown in a workspace folder you choose.
You can open the same files in Obsidian, edit them in your usual editor, keep
them in Git or copy the folder for backup. If a file changes outside BuildFlow,
save conflict checks help you resolve the competing edits.

BuildFlow runs from one self-contained executable for Windows or Linux and
opens a local browser interface at **<http://127.0.0.1:8765>**. There is no
separate runtime, database or web server to install, and no BuildFlow cloud
account to configure. An API key is optional until you use the model API.
Closing the browser tab leaves the
application running in the system tray, ready to reopen when you return to your
projects.

## Quick start

1. Get or [build `BuildFlow.exe`](BUILD.md), keep it at a fixed path and
   launch it. If Windows blocks the executable, see [Install](#install).
2. Choose a writable workspace folder, such as `D:\Projects\BuildFlow` or a
   folder inside your Obsidian vault.
3. Create a project and enter your idea, goals and constraints in Step 1.
4. Save your input, assemble the prompt, review it and copy it into your AI
   tool. Paste the answer into **Output** and save.
5. Repeat for Steps 2, 2.5 and 3. Keep each complete answer, including its
   structured footer; Step 3 carries the effective implementation context.
6. Generate and save the combined Step 4 answer first. Detect packages, review
   the preview and import selected packages. Then refine individual slots.
7. Select a package in Step 5, assemble the request and save a snapshot of the
   exact text you send. Save the returned coding prompt as that package's
   output. Copy that coding prompt into your coding agent to implement it.
8. Create your other projects in the same workspace. Use the project selector
   to switch between them and revisit each one's saved phases and packages.

---

## Contents

- [A place for every project and every phase](#a-place-for-every-project-and-every-phase)
- [Pick up the context when you switch back](#pick-up-the-context-when-you-switch-back)
- [A routine you can reuse across projects](#a-routine-you-can-reuse-across-projects)
- [Quick start](#quick-start)
- [Install](#install)
- [Install on Linux](#install-on-linux)
- [First run and choosing a workspace](#first-run-and-choosing-a-workspace)
- [Opening BuildFlow again](#opening-buildflow-again)
- [Quitting](#quitting)
- [Starting with Windows](#starting-with-windows)
- [Starting at login on Linux](#starting-at-login-on-linux)
- [The workflow](#the-workflow)
- [Workspace layout](#workspace-layout)
- [Prompt templates: global and local](#prompt-templates-global-and-local)
- [Language model settings](#language-model-settings)
- [Prompt snapshots](#prompt-snapshots)
- [Working in Obsidian at the same time](#working-in-obsidian-at-the-same-time)
- [Backup and restore](#backup-and-restore)
- [Using the REST API from a script](#using-the-rest-api-from-a-script)
- [Security model](#security-model)
- [Where files are kept](#where-files-are-kept)
- [Command line flags](#command-line-flags)
- [Building from source](#building-from-source)
- [Verification](#verification)
- [Known limitations](#known-limitations)

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

## Install on Linux

Download and extract `BuildFlow-linux-amd64.tar.gz` to a stable user-owned
location. The archive includes the executable and Linux instructions; no Go or
compiler is needed to run it.

```sh
mkdir -p "$HOME/.local/opt/buildflow"
tar -xzf BuildFlow-linux-amd64.tar.gz -C "$HOME/.local/opt/buildflow"
chmod +x "$HOME/.local/opt/buildflow/BuildFlow"
"$HOME/.local/opt/buildflow/BuildFlow"
```

BuildFlow opens <http://127.0.0.1:8765> on first run and listens on loopback
only. Type an absolute POSIX path or use the portal folder picker. The picker
requires a session D-Bus FileChooser portal version 3 or later; typing a path
works without it.

`xdg-open` opens browser links, folders, and logs. Clipboard copying uses
`wl-copy` on Wayland, or `xclip`/`xsel` on X11. A StatusNotifier tray host is
optional; if it is unavailable, the web interface still has a Quit control.
BuildFlow retries tray registration if the watcher starts or restarts. It does
not modify Omarchy or Hyprland configuration.

For login startup, keep the executable at a stable absolute path and enable
**Start at login** in Settings. The XDG entry runs BuildFlow minimized without
opening a browser. If you move the executable, disable and re-enable the setting
to update the recorded path. See [README-linux.md](README-linux.md) for removal,
application data paths, and helper installation details.

BuildFlow stores Linux settings in `$XDG_CONFIG_HOME/buildflow` (default
`~/.config/buildflow`) and logs/API token/provider keys in `$XDG_STATE_HOME/buildflow`
(default `~/.local/state/buildflow`). Relative XDG values are ignored. Set
`BUILDFLOW_HOME` for a unified isolated root; isolated runs keep their login
entry under that root and receive a separate loopback port.

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

## Starting at login on Linux

**Settings → Start at login**, or the tray menu item, writes
`$XDG_CONFIG_HOME/autostart/buildflow.desktop` (default
`~/.config/autostart/buildflow.desktop`). The entry uses an absolute quoted
executable path and `--minimized`, so login startup does not open the browser.

Keep the executable at a stable location. If it moves, turn the setting off and
on again to record the new path. With `BUILDFLOW_HOME` set, the startup entry is
stored under that isolated root instead of the normal user's autostart folder.
For install, helper, and removal instructions, see [README-linux.md](README-linux.md).

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
| **4** Work packages | Combined package mapping, then individual packages | Step 3, including all package contracts |
| **5** Package coding prompt | One coding prompt for **one** package; no implementation | selected saved package and Step 3 effective context |

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

Each entry in the `warnings` array of the assembly response carries a token,
source path, reason, severity and `required` flag. Projection diagnostics also
identify invalid links and missing data. Treat `required` as authoritative and
review blockers before continuing; optional empty notes are advisory.

### Structured answers and handoffs

Starter prompts request a complete readable answer followed by a terminal
`## BuildFlow Structured View` section. Its JSON fence sits between
`<!-- buildflow:view:start -->` and `<!-- buildflow:view:end -->`. Keep both the
narrative and footer when saving. The saved Markdown remains the source of truth;
Blocks and prompt projections are derived views and never rewrite it.

The current answer schema is **version 2**, independently of the `/v3` projection
profiles. Each envelope declares `schema_version`, `stage`, `scope` (`stage` or
`package`), `package_id` (null for stage scope), `state` (`ready` or `blocked`),
and typed `blocks`. The `{{contract.schema}}` placeholder inserts the exact
contract for the selected stage and scope. Types live in
[internal/structured/types.go](internal/structured/types.go); complete examples
for all six stages live in [testdata](internal/structured/testdata).

| Answer | Main blocks |
| --- | --- |
| Step 1 | Problem, goals, scenarios, requirements, constraints, options, evidence |
| Step 2 | Decision, design, interfaces, scope, delivery plan, settled decisions |
| Step 2.5 | Technology, libraries, tools, services, commands, costs, system context, contracts |
| Step 3 | Modules, contracts, effective implementation context, design changes, tests, acceptance, coverage |
| Step 4 | Packages, build batches, integration points, deliberately omitted work |
| Step 5 | Target package, coding prompt, acceptance checks |

Stable IDs connect requirements, components, modules, contracts, tests,
acceptance criteria and packages. Preserve IDs across stages; resolve references
against saved upstream answers. Evidence uses source and assumption IDs;
applicability links select the records relevant to a module or package. Invalid,
unresolved or conflicting links produce diagnostics. Required undetermined data
blocks projection instead of inventing a value. Every contract used or produced
by Step 4 must already exist in Step 3. Step 3's effective implementation context
carries accepted changes to Step 2.5 forward.

`{{handoff}}` selects a fixed field projection, compiled into Go in
[prompt_projection_specs.go](internal/workspace/prompt_projection_specs.go).
Templates and model output cannot change those field lists. This table describes
the selection; the Go registry is the exact list of leaf fields and predicates.

| Consumer | Saved sources and selection |
| --- | --- |
| Step 2 | Step 1 research fields and referenced evidence |
| Step 2.5 | Step 2 decision, requirements, design, scope and applicable common records |
| Step 3 | Step 2 design plus Step 2.5 technology, operations, contracts and evidence |
| Step 4 combined | Step 3 modules, contracts, effective context, tests, acceptance and coverage |
| Step 4 individual | Same profile narrowed by the saved combined package mapping; optional saved slot refinement |
| Step 5 | Selected saved package plus linked Step 3 modules, requirements, contract closure, effective context and selected commands |

Step 5 never substitutes a package from the combined answer for a missing saved
package file. Source hashes and selected record hashes support freshness checks;
changes to selected requirements or declarations require package review.

Handoffs render deterministic compact JSON with stable key order. Whitespace
outside strings is reduced; declarations, code, commands, paths, rules and other
string values remain intact. Sources show paths, hashes, selections and fallback
boundaries. Size estimates use roughly **4 characters per token** and warn at
**32,000 estimated tokens**, or **48,000 for Step 4**. These are warnings only;
BuildFlow never automatically trims content, including manually edited prompts.

Saving answers no longer copies them into downstream inputs. Inputs hold your
notes, and assembly reads the saved sources. Existing `buildflow:auto` tails
remain on disk but are excluded from user notes during assembly.

### Legacy Markdown and fallback limits

Answers without a valid current footer remain readable and editable. A supported
version 1 research footer can be adapted in memory; unsupported versions and
malformed footers expose diagnostics. Fallback selects registered Markdown
headings, ignoring headings inside fenced examples. When a required section is
not found, it includes the document once, with a warning. This can make prompts
much larger and cannot provide the same ID validation or package isolation as a
valid structured answer. Missing mappings or incompatible baselines may still
block assembly; fallback does not fabricate contracts or links. Save a corrected
complete answer to restore structured projections.

### Step 4: work packages as you go

Step 4 shows one slot per work package, and always one more than you have filled
in — the extra empty slot is how you add the next package:

- **Notes (input)** — your notes for that package, saved to
  `inputs/step-04/P01.md`.
- **Work package (output)** — the package content. Saving it creates
  `packages/P01.md` (with Obsidian front matter). Step 5 reads this saved
  package directly; its input remains your session notes.

A slot counts as filled once it holds anything — notes or a package — so saving
a package also opens the next empty slot. The single `outputs/step-04.md` file,
the "Split into work packages" detector and "Add next package" all still work
for existing projects.

### Step 4: splitting into packages

Generate the combined answer before assembling an individual package prompt.
Detection matches narrative package headings to structured records, preserving
the combined narrative, overview and footer. Each imported package gets only its applicable records and
provenance. Editing a slot does not change the combined source. Detection is a
preview; import checks the preview hashes before overwriting existing packages.

Legacy Markdown can still use headings:

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
| `empty` | no package document or prompt snapshot exists |
| `ready` | the package document exists, but no prompt snapshot is saved |
| `prompt-ready` | a prompt snapshot exists, but no non-empty output is saved |
| `output-saved` | non-empty output exists, without a valid ready coding-prompt artifact |
| `prompt-generated` | output contains a valid ready Step 5 coding-prompt artifact |

These statuses describe saved artifacts. Step 5 produces a coding prompt only;
implementation and verification happen in your coding tools.

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

## Language model settings

**Settings → Language model** lets you choose a mode, provider and chat model.
Use **Load models** to check the saved key and fetch the provider's current model
list. API keys are entered as password fields and are never returned by the
settings API.

- **Free** uses Groq Cloud. Groq's free account has request and token limits;
  the active model and account determine the current limits. BuildFlow does not
  switch to a paid provider when a limit is reached. For common GPT-OSS and Qwen
  chat models, the selector shows Groq's published free defaults (30 RPM,
  1,000 RPD, 8,000 TPM and 200,000 TPD); account limits can differ. See
  [Groq's rate-limit documentation](https://console.groq.com/docs/rate-limits).
- **Premium** uses your key for OpenAI, DeepSeek, Mistral or OpenRouter. The
  provider account controls billing and rate limits. Premium does not mean
  unlimited requests.

BuildFlow stores keys in `.env` under its private state folder, separate from
the Markdown workspace and Git projects. On Linux this is
`$XDG_STATE_HOME/buildflow/.env` (default `~/.local/state/buildflow/.env`); on
Windows it is `%LOCALAPPDATA%\BuildFlow\.env`. The file is restricted to the
current user where the operating system supports file permissions. The key is
plain text on disk, so protect your user account and backups accordingly.

BuildFlow's local REST API provides `GET /api/llm`, `PUT /api/llm`,
`PUT /api/llm/key`, `DELETE /api/llm/key?provider=...`,
`GET /api/llm/models?provider=...` and `POST /api/llm/chat`. The chat route uses
the selected provider and model. The current stage workbench still uses its
copy/paste loop; guided AI planning and approval screens are planned for v1.1.

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
| `{{user.project_name}}` `{{user.use_case}}` | project name and Step 1 use case |
| `{{user.notes}}` `{{user.package_notes}}` | stage notes and selected package notes |
| `{{user.target}}` | selected scope and package mapping |
| `{{handoff}}` | fixed consumer projection described above |
| `{{contract.schema}}` | exact output contract for this target |
| `{{input}}` | compatibility alias for this step's input notes |
| `{{output_step_01}}` … `{{output_step_04}}` | a saved output of any step |
| `{{previous_output}}` | the previous step's output, for the bottom of the prompt |
| `{{system_context}}` | the *Reusable System Context* section of Step 2.5 |
| `{{package_id}}` `{{package_title}}` | which package Step 5 is for |
| `{{package_content}}` | that package's file |
| `{{package_input}}` | your notes for that package |
| `{{missing_sources}}` | a list of what is still missing, or a note that nothing is |

A placeholder BuildFlow does not recognise is left exactly as written and
reported as a warning, rather than being silently emptied.

All six stages ship with starter prompts. Existing global templates and local
overrides are preserved when the executable changes. To adopt updated starters,
edit and save the active template through the normal template editor (or its PUT
endpoint), retaining any custom instructions. Remove an obsolete local override
if you want the updated global template to become active. There is no automatic
template migration.

Whole-output compatibility placeholders (`output_step_*`, `previous_output`,
`package_content`, `system_context`) bypass the fixed projection and warn about
prompt size. Prefer `{{handoff}}` in updated templates.

### Optional Generate summary

For a valid structured answer, Generate summary explains the saved blocks using
the exact saved prompt snapshot, or a labelled current assembly if no snapshot
exists. It uses only the configured **Free** mode through the existing settings
and chat endpoint. Missing configuration or Paid mode opens Settings; there is
no paid fallback. The summary is session-only, separate from the saved answer,
and is invalidated when its source changes. It is never a downstream handoff.

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

Settings, logs, the API token and any provider keys live separately in
`%LOCALAPPDATA%\BuildFlow\`. Back that up too if you want the chosen workspace
path, sign-in setting, token and provider keys to come with you.

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
| GET | `/api/llm` | selected model settings and provider key status; never returns key values |
| PUT | `/api/llm` | select Free/Premium mode, provider and model |
| PUT | `/api/llm/key` | add or replace a provider API key |
| DELETE | `/api/llm/key?provider=...` | remove a saved provider key |
| GET | `/api/llm/models?provider=...` | fetch the provider's current chat models |
| POST | `/api/llm/chat` | send messages through the selected provider and model |
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
| **Provider keys stay server-side** | Keys live in the app's `.env` state file, are masked in the settings page, and are never returned by the API. Model requests send their message content to the selected provider over HTTPS. |
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
  config.json        settings: workspace, startup, provider and model selection
  api-token.txt      the local REST API token
  .env               provider API keys (when configured)
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
BuildFlow[.exe] [options]

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
Windows result is a self-contained `.exe`; the Linux build uses a pure-Go D-Bus
module and is packaged by `build-linux.sh`.

## Verification

```powershell
powershell -ExecutionPolicy Bypass -File scripts\verify.ps1
```

This starts a private instance in a temporary folder - your real workspace and
`%LOCALAPPDATA%\BuildFlow` are never touched - and drives it over HTTP through
checks covering loopback-only binding, token and origin rejection, project
create/rename/switch, saves and byte-for-byte reloads, external-edit conflicts,
global versus local templates, assembly and its prerequisites, snapshots,
package detection and import, N+1 creation, per-package Step 5 records, path
traversal rejection, persistence across a restart, single-instance enforcement,
the sign-in setting and a clean quit.

The suite exercises legacy Markdown and the six structured stage fixtures,
combined import, individual package assembly, exact snapshots, source
preservation, package isolation and external-edit conflicts in an isolated
workspace. It updates active templates using the ordinary save endpoint.

Run the portable checks from the repository root:

```sh
go build ./...
go vet ./...
go test ./...
node --test web/ui_test.js
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o /tmp/buildflow-plan-check.exe .
```

The Node checks exercise UI behavior with a DOM test double. Also check the real
browser against the private verification workspace: visit every stage, inspect
Blocks/Full output/Edit, import the combined answer, select individual packages,
copy the generated coding prompt, reload snapshots and provoke an external edit
before saving. Do not run the suite against your everyday workspace.

## Known limitations

Honest list of what is not there:

- **Desktop integration depends on the platform.** Linux folder selection
  requires a FileChooser portal v3 or later; its tray requires a running
  StatusNotifier watcher; clipboard support requires `wl-copy` on Wayland or
  `xclip`/`xsel` on X11. BuildFlow remains usable through its web interface
  without these optional services.
- **The Windows folder picker is the older shell dialog** (`SHBrowseForFolder`)
  and does not let you type a new folder name into it. Typing a path into
  Settings works on every supported platform.
- **Guided planning is not wired into the stage screens yet.** You still copy
  prompts and paste answers. The provider selector and local chat API are the
  foundation for v1.1 planning and approval flows.
- **Not a multi-user application.** One workspace, one token, one user. It is
  not hardened for a shared machine.
- **No search across a workspace** in the UI. Obsidian is better at that, which
  is the point of storing plain Markdown.
- **No project export or import as an archive.** Copy the folder. If you want
  one, an "export project as a folder" button is the obvious addition.
- **Package detection needs review.** Narrative package headings identify
  candidates, with matching structured records when available. They do not
  prove the work is well scoped.
  Always read the preview before importing.
- **Legacy system context extraction depends on a heading.** The compatibility
  `{{system_context}}` token looks for a
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
