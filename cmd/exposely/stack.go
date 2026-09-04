package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/reaksmeykem/exposely/internal/models"
	"github.com/reaksmeykem/exposely/internal/settings"
	"github.com/reaksmeykem/exposely/internal/stacks"
	"github.com/reaksmeykem/exposely/internal/sysproc"
)

// cliStackRunner is a slim wrapper giving the CLI access to the same
// stacks.Manager the desktop app uses. The desktop App type is not
// importable from the CLI (it lives in package main of the Wails
// binary), so the CLI re-implements the three flows it needs: start
// (with MySQL data-dir bootstrap), stop, and status.
type cliStackRunner struct {
	store      *settings.Store
	appDataDir string
	manager    *stacks.Manager
}

func newCLIStackRunner(r *cliRunner) *cliStackRunner {
	return &cliStackRunner{
		store:      r.store,
		appDataDir: r.appDataDir,
		manager:    stacks.NewManager(),
	}
}

func (c *cliStackRunner) applyConfigs(settingsValue models.AppSettings) {
	stack := settingsValue.Stack
	if strings.TrimSpace(stack.NginxBinaryPath) != "" {
		nginxRoot := filepath.Dir(strings.TrimSpace(stack.NginxBinaryPath))
		confPath := filepath.Join(c.appDataDir, "stacks", "nginx", "nginx.conf")
		// Base site + every registered project vhost, matching the
		// desktop app, so regenerating the conf never drops vhosts.
		usePHP := strings.TrimSpace(stack.PHPCGIBinaryPath) != ""
		var sites []stacks.SiteConfig
		registry := stacks.LoadSiteRegistry(c.appDataDir)
		// The registry owns the "localhost" vhost when phpMyAdmin (or any
		// explicit entry) claims it; only add the default www site when
		// nothing else is bound to localhost, since nginx routes on the
		// first matching server block.
		hasLocalhost := false
		for _, entry := range registry.Sites {
			if strings.EqualFold(entry.ServerName, "localhost") {
				hasLocalhost = true
			}
		}
		if !hasLocalhost {
			sites = append(sites, stacks.SiteConfig{
				ServerName: "localhost",
				Root:       filepath.Join(c.appDataDir, "stacks", "www"),
				ListenPort: stack.EffectiveNginxPort(),
				PHP:        usePHP,
				PHPPort:    stack.EffectivePHPPort(),
				Index:      []string{"index.html", "index.php"},
			})
		}
		for _, entry := range registry.Sites {
			sites = append(sites, stacks.SiteConfig{
				ServerName: entry.ServerName,
				Root:       entry.Root,
				PHP:        usePHP && entry.PHP,
				PHPPort:    stack.EffectivePHPPort(),
				ListenPort: stack.EffectiveNginxPort(),
				Index:      []string{"index.html", "index.php"},
			})
		}
		conf := stacks.RenderNginxConf(nginxRoot, stack.EffectiveNginxPort(), sites)
		if err := stacks.WriteFile(confPath, conf); err == nil {
			c.manager.SetConfig(stacks.ServiceNginx, stacks.ServiceConfig{
				BinaryPath: stack.NginxBinaryPath,
				Args:       []string{"-p", nginxRoot, "-c", confPath},
			})
		}
	}
	if strings.TrimSpace(stack.PHPCGIBinaryPath) != "" {
		c.manager.SetConfig(stacks.ServicePHP, stacks.ServiceConfig{
			BinaryPath: stack.PHPCGIBinaryPath,
			Args:       stacks.PHPStartArgs(stack.EffectivePHPPort(), stack.EffectivePHPWorkers()),
		})
	}
	if strings.TrimSpace(stack.MySQLDBinaryPath) != "" {
		c.manager.SetConfig(stacks.ServiceMySQL, stacks.ServiceConfig{
			BinaryPath: stack.MySQLDBinaryPath,
			Args: stacks.MySQLStartArgs(stacks.MySQLDefaults{
				BaseDir: filepath.Dir(strings.TrimSpace(stack.MySQLDBinaryPath)),
				DataDir: filepath.Join(c.appDataDir, "stacks", "mysql", "data"),
				Port:    stack.EffectiveMySQLPort(),
			}),
			Env: []string{"MYSQL_HOME=" + filepath.Dir(strings.TrimSpace(stack.MySQLDBinaryPath))},
		})
	}
}

func (c *cliStackRunner) startService(name string) error {
	settingsValue, err := c.store.Load()
	if err != nil {
		return err
	}
	c.applyConfigs(settingsValue)

	normalized := stacks.Service(strings.ToLower(strings.TrimSpace(name)))
	switch normalized {
	case stacks.ServiceNginx, stacks.ServicePHP:
	case stacks.ServiceMySQL:
		cfg, ok := c.manager.Config(stacks.ServiceMySQL)
		if !ok || strings.TrimSpace(cfg.BinaryPath) == "" {
			return errors.New("mysqld binary path is not configured (set stack.mysqlDBinaryPath in settings.json)")
		}
		dataDir := filepath.Join(c.appDataDir, "stacks", "mysql", "data")
		if err := stacks.EnsureMySQLDataDir(cfg.BinaryPath, dataDir); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown service %q (use nginx, php, or mysql)", name)
	}

	// Already running from a previous CLI invocation? The in-memory
	// Manager is fresh every run, so consult the PID file.
	if pid := stacks.LoadPID(c.appDataDir, normalized); stacks.ProcessAlive(pid) {
		fmt.Printf("%s already running (pid %d)\n", normalized, pid)
		return nil
	}

	status := c.manager.Start(normalized)
	if !status.Running {
		if status.LastError != "" {
			return errors.New(status.LastError)
		}
		return fmt.Errorf("failed to start %s", name)
	}
	if err := stacks.SavePID(c.appDataDir, normalized, status.PID); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not persist pid for %s: %v\n", normalized, err)
	}
	fmt.Printf("%s started (pid %d)\n", normalized, status.PID)
	return nil
}

func (c *cliStackRunner) startAll() error {
	settingsValue, err := c.store.Load()
	if err != nil {
		return err
	}
	c.applyConfigs(settingsValue)
	for _, name := range []string{"mysql", "php", "nginx"} {
		if err := c.startService(name); err != nil {
			return err
		}
	}
	return nil
}

func (c *cliStackRunner) stopAll() {
	stoppedAny := false
	for _, service := range []stacks.Service{stacks.ServiceNginx, stacks.ServicePHP, stacks.ServiceMySQL} {
		// Stop in-memory tracked processes first (desktop-app flow),
		// then any process recorded in a PID file by an earlier CLI run.
		c.manager.Stop(service)
		pid := stacks.LoadPID(c.appDataDir, service)
		if stacks.ProcessAlive(pid) {
			if err := stacks.KillPID(pid); err == nil {
				stoppedAny = true
				fmt.Printf("%s stopped (pid %d)\n", service, pid)
			} else {
				fmt.Fprintf(os.Stderr, "warning: could not stop %s (pid %d): %v\n", service, pid, err)
			}
		}
		stacks.ClearPID(c.appDataDir, service)
	}
	if !stoppedAny {
		fmt.Println("Managed stack services stopped (nginx, php, mysql).")
	}
}

// openDatabaseManager registers phpMyAdmin as a localhost vhost on the
// managed nginx, ensures the stack is running, and opens the browser.
// Mirrors the desktop app's OpenDatabaseManager flow without Wails.
func (c *cliStackRunner) openDatabaseManager() (string, error) {
	settingsValue, err := c.store.Load()
	if err != nil {
		return "", err
	}
	stackCfg := settingsValue.Stack
	nginxPath := strings.TrimSpace(stackCfg.NginxBinaryPath)
	if nginxPath == "" {
		return "", errors.New("the managed stack is not configured (set stack.nginxBinaryPath in settings.json)")
	}
	pmaDir, ok := stacks.EnsureOwnedPhpMyAdmin(c.appDataDir)
	if !ok {
		return "", errors.New("phpMyAdmin was not found on this machine. Install it (or use EnvKit/Laragon/XAMPP which bundle it) and try again")
	}

	registry := stacks.LoadSiteRegistry(c.appDataDir)
	if err := registry.Upsert(stacks.SiteEntry{
		ServerName:  "localhost",
		ProjectPath: pmaDir,
		Root:        pmaDir,
		PHP:         true,
	}); err != nil {
		return "", fmt.Errorf("could not register phpMyAdmin: %w", err)
	}
	_ = stacks.WriteFile(filepath.Join(pmaDir, "config.user.inc.php"), stacks.PhpMyAdminServerConfig("127.0.0.1", stackCfg.EffectiveMySQLPort()))

	// applyConfigs regenerates the conf from the registry (including the
	// phpMyAdmin entry just upserted) and configures the services.
	c.applyConfigs(settingsValue)

	if err := c.startService("mysql"); err != nil {
		return "", err
	}
	if err := c.startService("php"); err != nil {
		return "", err
	}
	// nginx: reload when our instance is already running, else start.
	if pid := stacks.LoadPID(c.appDataDir, stacks.ServiceNginx); stacks.ProcessAlive(pid) {
		nginxRoot := filepath.Dir(nginxPath)
		confPath := filepath.Join(c.appDataDir, "stacks", "nginx", "nginx.conf")
		if err := stacks.NginxConfigTest(nginxPath, nginxRoot, confPath); err != nil {
			return "", err
		}
		if err := stacks.ReloadNginx(nginxPath, nginxRoot, confPath); err != nil {
			return "", err
		}
	} else if err := c.startService("nginx"); err != nil {
		return "", err
	}

	port := stackCfg.EffectiveNginxPort()
	if err := stacks.WaitForPort("127.0.0.1", port, 5*time.Second); err != nil {
		return "", err
	}
	// Host header must be "localhost" to match the phpMyAdmin vhost.
	url := fmt.Sprintf("http://localhost:%d/index.php", port)
	if err := openBrowser(url); err != nil {
		return "", err
	}
	return url, nil
}

func openBrowser(target string) error {
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	cmd.SysProcAttr = sysproc.Hidden()
	return cmd.Start()
}

func (c *cliStackRunner) printStatus() {	for _, st := range c.manager.AllStatus() {
		state := "stopped"
		if st.Running {
			state = fmt.Sprintf("running (pid %d)", st.PID)
		} else if pid := stacks.LoadPID(c.appDataDir, st.Service); stacks.ProcessAlive(pid) {
			// Started by a previous CLI run: the in-memory manager does
			// not know about it, but the PID file does.
			state = fmt.Sprintf("running (pid %d, from previous run)", pid)
		}
		line := fmt.Sprintf("%-6s %s", st.Service, state)
		if st.LastError != "" {
			line += " — " + st.LastError
		}
		fmt.Println(line)
	}
}

func runStackCommand(r *cliRunner, args []string) error {
	if len(args) < 1 {
		printStackUsage()
		return nil
	}
	stack := newCLIStackRunner(r)
	sub := strings.ToLower(strings.TrimSpace(args[0]))
	rest := args[1:]

	switch sub {
	case "start":
		if len(rest) < 1 {
			return stack.startAll()
		}
		return stack.startService(rest[0])
	case "stop":
		stack.stopAll()
		return nil
	case "status":
		stack.printStatus()
		return nil
	case "db":
		url, err := stack.openDatabaseManager()
		if err != nil {
			return err
		}
		fmt.Printf("phpMyAdmin opened: %s\n", url)
		return nil
	case "php-install":
		settingsValue, err := stack.store.Load()
		if err != nil {
			return err
		}
		dir, iniPath, err := stacks.InstallPHP(stack.appDataDir)
		if err != nil {
			return err
		}
		settingsValue.Stack.UseManagedPHP = true
		settingsValue.Stack.PHPCGIBinaryPath = filepath.Join(dir, "php-cgi.exe")
		if err := stack.store.Save(settingsValue); err != nil {
			return err
		}
		fmt.Printf("PHP installed at %s\nphp.ini: %s\nversion: %s\n", dir, iniPath, stacks.PHPVersionOf(dir))
		return nil
	case "help", "-h", "--help":
		printStackUsage()
		return nil
	default:
		printStackUsage()
		return fmt.Errorf("unknown stack command %q", sub)
	}
}

func printStackUsage() {
	fmt.Println(`Manage the Exposely stack (nginx + PHP + MySQL).

Usage:
  exposely stack start          start mysql, then php, then nginx
  exposely stack start nginx    start one service (nginx|php|mysql)
  exposely stack stop           stop all managed services
  exposely stack status         show per-service state
  exposely stack db             open phpMyAdmin for the managed database

Configure binary paths in the desktop app Settings tab or edit
%AppData%\Exposely\settings.json:

  "stack": {
    "nginxBinaryPath": "C:\\nginx\\nginx.exe",
    "phpCgiBinaryPath": "C:\\php\\php-cgi.exe",
    "mysqldBinaryPath": "C:\\mysql\\bin\\mysqld.exe",
    "nginxPort": 8090,
    "phpPort": 9000,
    "mysqlPort": 3306
  }`)
}
