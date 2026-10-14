// Command BuildFlow is a local Windows workbench for a manual, prompt-driven AI
// build workflow. It embeds its web interface, exposes a loopback-only REST API
// and stores every document as UTF-8 Markdown in a workspace folder you choose,
// so the same files read cleanly in Obsidian.
//
// Run BuildFlow.exe          start the background app and open the browser
// Run BuildFlow.exe --console  same, attached to a console, for troubleshooting
// Run BuildFlow.exe --minimized  start quietly in the notification area
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"buildflow/internal/appdata"
	"buildflow/internal/logging"
	"buildflow/internal/server"
	"buildflow/internal/winutil"
	"buildflow/internal/workspace"
	"buildflow/web"
)

const mutexName = "BuildFlow.SingleInstance"

func main() {
	os.Exit(run())
}

func run() int {
	var (
		showConsole  = flag.Bool("console", false, "attach to the parent console and echo logs")
		minimized    = flag.Bool("minimized", false, "start without opening the browser")
		noBrowser    = flag.Bool("no-browser", false, "do not open the browser on start")
		portFlag     = flag.Int("port", 0, "override the loopback port (default from config.json, usually "+strconv.Itoa(appdata.DefaultPort)+")")
		workspaceArg = flag.String("workspace", "", "workspace folder to use on start")
		versionFlag  = flag.Bool("version", false, "print the version and exit")
	)
	flag.CommandLine.SetOutput(os.Stdout)
	flag.Usage = func() {
		fmt.Fprintf(os.Stdout, "BuildFlow %s - local build workflow workbench\n\n", server.Version)
		fmt.Fprintln(os.Stdout, "Usage: BuildFlow.exe [options]")
		fmt.Fprintln(os.Stdout, "  Serve the workbench on http://127.0.0.1:"+strconv.Itoa(appdata.DefaultPort))
		fmt.Fprintln(os.Stdout, "\nOptions:")
		flag.PrintDefaults()
		fmt.Fprintln(os.Stdout, "\nData locations (per user, no administrator rights needed):")
		fmt.Fprintln(os.Stdout, "  %LOCALAPPDATA%\\BuildFlow\\config.json    settings")
		fmt.Fprintln(os.Stdout, "  %LOCALAPPDATA%\\BuildFlow\\logs\\           rotating log files")
		fmt.Fprintln(os.Stdout, "  %LOCALAPPDATA%\\BuildFlow\\api-token.txt  local REST API token")
	}
	flag.Parse()

	if *versionFlag {
		fmt.Println("BuildFlow " + server.Version)
		return 0
	}

	console := *showConsole
	if console {
		winutil.AttachConsole()
	}

	// ---- per-user application data ------------------------------------
	store, err := appdata.Open()
	if err != nil {
		fatal("BuildFlow cannot prepare its settings folder", err, console)
		return 1
	}
	settings := store.Settings()
	if *portFlag > 0 && *portFlag != settings.Port {
		_ = store.Update(func(s *appdata.Settings) { s.Port = *portFlag })
		settings = store.Settings()
	}

	logger, err := logging.New(store.LogDir(), logging.LevelInfo, console)
	if err != nil {
		fatal("BuildFlow cannot open its log file", err, console)
		return 1
	}
	defer logger.Close()

	if *workspaceArg != "" {
		if err := store.SetWorkspacePath(*workspaceArg); err != nil {
			logger.Errorf("ignoring invalid --workspace value: %v", err)
		} else {
			settings = store.Settings()
		}
	}

	logger.Infof("BuildFlow %s starting (pid %d, console=%v)", server.Version, os.Getpid(), console)
	logger.Infof("settings folder: %s", store.Path())
	if err := checkWindowsVersion(logger); err != nil {
		logger.Warnf("%v", err)
	}

	// ---- one copy only -------------------------------------------------
	instance, already, err := winutil.AcquireInstance(mutexName)
	if err != nil {
		logger.Errorf("single instance check failed: %v", err)
		winutil.ErrorBox("BuildFlow", "BuildFlow could not create its single-instance lock:\n\n"+err.Error())
		return 1
	}
	if already {
		logger.Infof("another copy is already running; opening its interface and exiting")
		url := "http://127.0.0.1:" + strconv.Itoa(settings.Port)
		if !waitForServer(settings.Port, 8*time.Second, logger) {
			winutil.ErrorBox("BuildFlow is already running",
				"Another copy of BuildFlow is already running, but its web server on "+url+
					" is not answering.\n\nThis usually means a stale process or a port conflict. "+
					"Check the log file:\n"+store.Path("logs", "buildflow.log"))
			return 0
		}
		// Do not wait for the browser: a second copy has done its job and must
		// exit promptly, otherwise it looks like a second instance is running.
		go func() {
			if err := winutil.OpenURL(url); err != nil {
				logger.Warnf("could not open the browser: %v", err)
			}
		}()
		logger.Infof("handed the request to the running instance at %s", url)
		return 0
	}
	defer instance.Release()

	// ---- workspace ------------------------------------------------------
	svc := workspace.NewService()
	if settings.WorkspacePath != "" {
		scaffold, err := svc.EnsureWorkspace(settings.WorkspacePath)
		if err != nil {
			logger.Errorf("workspace %q is unusable: %v", settings.WorkspacePath, err)
		} else {
			logger.Infof("workspace ready: %s (%d new entries, %d already present)",
				scaffold.Root, len(scaffold.Created), len(scaffold.Existing))
		}
	} else {
		logger.Infof("no workspace chosen yet; the interface will ask for one")
	}

	// ---- API token ------------------------------------------------------
	token, err := ensureToken(store, settings.Port)
	if err != nil {
		logger.Errorf("API token problem: %v", err)
		winutil.ErrorBox("BuildFlow", "BuildFlow could not create its local API token:\n\n"+err.Error())
		return 1
	}

	// ---- server ---------------------------------------------------------
	quit := make(chan string, 4)
	shutdownOnce := &sync.Once{}
	requestShutdown := func(reason string) {
		shutdownOnce.Do(func() {
			logger.Infof("shutting down: %s", reason)
			select {
			case quit <- reason:
			default:
			}
		})
	}

	paths := func() server.DiagnosticsPaths {
		return server.DiagnosticsPaths{
			Config: store.ConfigPath(),
			Log:    logger.Path(),
			Token:  store.TokenPath(),
			Home:   store.Path(),
		}
	}
	urlFor := func() string { return "http://127.0.0.1:" + strconv.Itoa(settings.Port) }

	app := &server.App{
		Store:        store,
		Log:          logger,
		Port:         settings.Port,
		BaseURL:      urlFor(),
		Started:      time.Now(),
		OpenBrowser:  func() error { return winutil.OpenURL(urlFor()) },
		OpenURL:      winutil.OpenURL,
		OpenFolder:   winutil.OpenFolder,
		PickFolder:   func(title string) (string, error) { return winutil.PickFolder(title) },
		Autostart:    winutil.SetAutostart,
		AutostartNow: winutil.AutostartEnabled,
		AutostartCmd: winutil.AutostartCommand,
		Paths:        paths,
		CopyToClip:   winutil.CopyToClipboard,
		Version:      server.Version,
		Quit: func() error {
			requestShutdown("quit requested from the interface")
			return nil
		},
	}

	srv := server.New(app, svc, token, web.Assets(), nil)

	// ListenRetry absorbs the brief window where a just-exited copy is still
	// releasing the port, which is what a user double-clicking the shortcut
	// twice in a row actually hits.
	listener, err := srv.ListenRetry(4 * time.Second)
	if err != nil {
		// The mutex was ours, yet something already serves this port. If that
		// something is a working BuildFlow, this is simply a second start:
		// hand over and leave quietly instead of accusing the user of a
		// conflict they did not cause.
		if winutil.IsPortReachable(settings.Port) {
			logger.Infof("another BuildFlow is already serving %s; handing over to it", urlFor())
			go func() {
				if oerr := winutil.OpenURL(urlFor()); oerr != nil {
					logger.Warnf("could not open the browser: %v", oerr)
				}
			}()
			return 0
		}
		logger.Errorf("cannot start the web server: %v", err)
		logger.Errorf("configuration folder: %s", store.Path())
		logger.Errorf("port in use? another program may own 127.0.0.1:%d", settings.Port)
		winutil.ErrorBox("BuildFlow cannot start",
			"BuildFlow could not open its local web server.\n\n"+
				"Reason: "+err.Error()+"\n\n"+
				"Most likely another program is using port "+strconv.Itoa(settings.Port)+
				". Close it and start BuildFlow again, or change the port in:\n"+
				store.ConfigPath()+"\n\n"+
				"A log was written to:\n"+logger.Path())
		return 1
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := srv.Serve(ctx, listener); err != nil {
			logger.Errorf("web server stopped with an error: %v", err)
		}
	}()

	// ---- notification area ---------------------------------------------
	tray := winutil.NewTray()
	tray.On("open", func() {
		if err := winutil.OpenURL(urlFor()); err != nil {
			logger.Warnf("could not open the browser: %v", err)
		}
	})
	tray.On("copy-url", func() {
		if err := winutil.CopyToClipboard(urlFor()); err != nil {
			logger.Warnf("could not copy the URL: %v", err)
		}
	})
	tray.On("autostart", func() {
		next := !winutil.AutostartEnabled()
		if err := winutil.SetAutostart(next); err != nil {
			logger.Errorf("could not change start with Windows: %v", err)
			winutil.ErrorBox("BuildFlow", "Could not change the sign-in setting:\n\n"+err.Error())
			return
		}
		_ = store.Update(func(s *appdata.Settings) { s.StartWithWindows = next })
		refreshTray(tray, store, svc)
	})
	tray.On("workspace", func() {
		root := svc.Root()
		if root == "" {
			winutil.InfoBox("BuildFlow", "No workspace folder is selected yet.\n\nOpen "+
				urlFor()+" and use Settings to choose one.")
			return
		}
		if err := winutil.OpenFolder(root); err != nil {
			logger.Warnf("could not open the workspace folder: %v", err)
		}
	})
	tray.On("log", func() {
		if err := winutil.OpenTextFile(logger.Path()); err != nil {
			winutil.ErrorBox("BuildFlow", "Could not open the log file:\n\n"+err.Error())
		}
	})
	tray.On("quit", func() { requestShutdown("quit from the notification area") })
	tray.On("session-end", func() {
		logger.Infof("Windows sign-out detected")
		_ = store.Save()
	})
	refreshTray(tray, store, svc)

	trayOK := false
	go func() {
		defer wg.Done()
		if err := tray.Start(); err != nil {
			logger.Warnf("notification area icon unavailable (%v); use the Quit button on "+urlFor(), err)
			return
		}
		trayOK = true
	}()

	// ---- browser --------------------------------------------------------
	openBrowser := settings.OpenBrowserOnStart && !*minimized && !*noBrowser
	if openBrowser {
		go func() {
			// Give the listener a moment so the page is ready when it appears.
			time.Sleep(350 * time.Millisecond)
			if err := winutil.OpenURL(urlFor()); err != nil {
				logger.Warnf("could not open the browser automatically (%v); open %s yourself", err, urlFor())
				winutil.InfoBox("BuildFlow is running",
					"BuildFlow is running in the notification area, but it could not open your browser.\n\n"+
						"Open this address manually:\n"+urlFor())
			}
		}()
	} else {
		logger.Infof("not opening the browser on start (minimized=%v noBrowser=%v setting=%v)",
			*minimized, *noBrowser, settings.OpenBrowserOnStart)
	}

	// ---- signals --------------------------------------------------------
	sig := make(chan os.Signal, 2)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintln(os.Stdout, "  BuildFlow "+server.Version+" is running")
	fmt.Fprintln(os.Stdout, "  Interface : "+urlFor())
	fmt.Fprintln(os.Stdout, "  Workspace : "+orDash(svc.Root()))
	fmt.Fprintln(os.Stdout, "  Settings  : "+store.ConfigPath())
	fmt.Fprintln(os.Stdout, "  Log       : "+logger.Path())
	fmt.Fprintln(os.Stdout, "  Quit      : notification area icon, or POST /api/app/quit, or Ctrl+C here")
	if !trayOK {
		fmt.Fprintln(os.Stdout, "  Note      : the notification area icon is still starting; if it never")
		fmt.Fprintln(os.Stdout, "              appears, use the Quit button on the web page.")
	}
	fmt.Fprintln(os.Stdout, "")

	var reason string
	select {
	case reason = <-quit:
	case s := <-sig:
		reason = "signal " + s.String()
	}

	cancel()
	_ = listener.Close()
	tray.Stop()
	_ = store.Save()
	logger.Infof("BuildFlow stopped (%s)", reason)
	logger.Close()
	wg.Wait()
	return 0
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "(not set yet - choose one in Settings)"
	}
	return s
}

// waitForServer polls the loopback port until something accepts a connection.
// A copy that has only just claimed the single-instance lock still needs a
// moment to reach Listen, so a brief miss is normal and must not be reported as
// a stale process.
func waitForServer(port int, within time.Duration, logger *logging.Logger) bool {
	deadline := time.Now().Add(within)
	for {
		if winutil.IsPortReachable(port) {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func fatal(title string, err error, console bool) {
	if console {
		fmt.Fprintf(os.Stderr, "%s: %v\n", title, err)
	}
	winutil.ErrorBox(title, err.Error())
}

// ensureToken loads the per-user API token, creating it on first run.
func ensureToken(store *appdata.Store, port int) (string, error) {
	path := store.TokenPath()
	if raw, err := os.ReadFile(path); err == nil {
		// The token is the only line that is not a comment.
		for _, line := range strings.Split(string(raw), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if len(line) >= 32 && !strings.ContainsAny(line, " \t") {
				return line, nil
			}
			break
		}
	}
	token, genErr := workspace.GenerateToken()
	if genErr != nil {
		return "", genErr
	}
	// The token is the only line that is not a comment, so a script can find it
	// with a simple "skip lines starting with #" filter.
	body := "# BuildFlow local REST API token.\n" +
		"#\n" +
		"# Send this value in the " + server.TokenHeader + " header with every\n" +
		"# /api request, for example:\n" +
		"#\n" +
		"#   curl -H \"" + server.TokenHeader + ": <token>\" http://127.0.0.1:" +
		strconv.Itoa(port) + "/api/projects\n" +
		"#\n" +
		"# This file is a secret: any process that can read it can control\n" +
		"# BuildFlow. It is only useful for scripts you run yourself.\n" +
		"#\n" +
		token + "\n"
	if err := workspace.WriteFileAtomic(path, []byte(body)); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return token, nil
}

func refreshTray(tray *winutil.Tray, store *appdata.Store, svc *workspace.Service) {
	items := []winutil.TrayItem{
		{ID: "open", Label: "Open BuildFlow", Enabled: true},
		{ID: "copy-url", Label: "Copy local address", Enabled: true},
		{ID: "-", Label: "", Enabled: true},
		{ID: "autostart", Label: "Start with Windows", Checked: winutil.AutostartEnabled(), Enabled: true},
	}
	root := svc.Root()
	items = append(items, winutil.TrayItem{ID: "workspace", Label: "Open workspace folder", Enabled: root != ""})
	if root != "" {
		items[len(items)-1].Label = "Open workspace: " + filepath.Base(root)
	}
	items = append(items,
		winutil.TrayItem{ID: "log", Label: "Open log file", Enabled: true},
		winutil.TrayItem{ID: "-", Label: "", Enabled: true},
		winutil.TrayItem{ID: "quit", Label: "Quit BuildFlow", Enabled: true},
	)
	tray.SetItems(items)
}

// checkWindowsVersion warns when running on a Windows build older than the
// supported baseline. It never blocks startup.
func checkWindowsVersion(logger *logging.Logger) error {
	major, minor, build, err := winutil.WindowsVersion()
	if err != nil {
		return nil // not worth failing over
	}
	if major < 10 {
		return fmt.Errorf("this Windows build is version %d.%d; BuildFlow is tested on Windows 11 and Windows 10",
			major, minor)
	}
	logger.Debugf("Windows %d.%d build %d", major, minor, build)
	return nil
}
