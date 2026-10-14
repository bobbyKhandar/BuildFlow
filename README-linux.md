# BuildFlow for Linux x86_64

BuildFlow is a local, browser-based workbench for a manual, prompt-driven AI
workflow. Its workspace is plain UTF-8 Markdown. The Linux build is a single
standalone executable and does not require Go, Node.js, a database, or a
compiler at runtime.

## Install and launch

Extract the archive into a stable user-owned location, then launch the binary:

```sh
mkdir -p "$HOME/.local/opt/buildflow"
tar -xzf BuildFlow-linux-amd64.tar.gz -C "$HOME/.local/opt/buildflow"
chmod +x "$HOME/.local/opt/buildflow/BuildFlow"
"$HOME/.local/opt/buildflow/BuildFlow"
```

BuildFlow opens its local interface at <http://127.0.0.1:8765> on first run.
The app listens only on loopback. Use Settings to choose a workspace or type
any absolute POSIX path, including paths containing spaces or Unicode. Existing
workspace Markdown files are used in place and are not rewritten because the
application is running on Linux.

Keep the executable at a stable absolute path if you enable login startup. The
XDG autostart entry records the current executable path. If you move the binary,
turn login startup off and on again in BuildFlow Settings so that the entry is
refreshed. Login startup uses `--minimized` and does not open a browser.

## Optional desktop helpers

- `xdg-open` opens browser links, the workspace folder, and the log file.
- On Wayland, `wl-copy` provides clipboard copying.
- On X11, install either `xclip` or `xsel` for clipboard copying.
- A session D-Bus service implementing `org.freedesktop.portal.FileChooser`
  version 3 or later provides the native folder picker. If the portal is not
  available, type an absolute workspace path in Settings instead.
- A StatusNotifier watcher and tray host provide the tray icon and actions.
  BuildFlow retries registration when the watcher appears or restarts. The web
  interface remains available when no tray host is running, including its Quit
  control.

BuildFlow does not edit Omarchy or Hyprland configuration. Omarchy's bar can
display StatusNotifier items when its tray widget is enabled.

## Application data

- Settings: `$XDG_CONFIG_HOME/buildflow`, defaulting to `~/.config/buildflow`.
- Logs and API token: `$XDG_STATE_HOME/buildflow`, defaulting to
  `~/.local/state/buildflow`.
- Relative or empty XDG directory values are ignored in favor of those defaults.
- `BUILDFLOW_HOME=/absolute/path` puts settings, logs, token, startup entry, and
  instance identity under one isolated root. The app does not alter the normal
  user's autostart entry when this override is set. Each isolated root gets its
  own random loopback port on its first run.

The API token is private to your user and should not be shared. The workspace
can live anywhere you have write access; it is independent of BuildFlow's
settings and state folders.

## Remove

Turn **Start at login** off in Settings, then remove the extracted application
directory. To remove application data as well, remove `~/.config/buildflow` and
`~/.local/state/buildflow` (or their XDG overrides). This does not remove your
workspace. For an isolated run, remove only the matching `BUILDFLOW_HOME` root.

## Build from source

From the repository root, run `./build-linux.sh`. It runs formatting, vet, and
the Go test suite, checks both Linux and Windows builds, and writes
`dist/BuildFlow-linux-amd64.tar.gz`.
