package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/reaksmeykem/exposely/internal/models"
	"github.com/reaksmeykem/exposely/internal/settings"
	"github.com/reaksmeykem/exposely/internal/stacks"
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
		sites := []stacks.SiteConfig{{
			ServerName: "localhost",
			Root:       nginxRoot,
			ListenPort: stack.EffectiveNginxPort(),
			Index:      []string{"index.html", "index.php"},
		}}
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

	status := c.manager.Start(normalized)
	if !status.Running {
		if status.LastError != "" {
			return errors.New(status.LastError)
		}
		return fmt.Errorf("failed to start %s", name)
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
	c.manager.StopAll()
	fmt.Println("Managed stack services stopped (nginx, php, mysql).")
}

func (c *cliStackRunner) printStatus() {
	for _, st := range c.manager.AllStatus() {
		state := "stopped"
		if st.Running {
			state = fmt.Sprintf("running (pid %d)", st.PID)
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
