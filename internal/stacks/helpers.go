package stacks

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/reaksmeykem/exposely/internal/sysproc"
)

// MySQLDefaults holds the generated bootstrap settings for a managed
// mysqld instance.
type MySQLDefaults struct {
	// BaseDir is the MySQL installation root (where mysqld.exe lives).
	BaseDir string
	// DataDir stores the database files. Usually under Exposely's own
	// app-data directory so we never touch an existing install.
	DataDir string
	// Port mysqld listens on (default 3306).
	Port int
	// RootPassword for --initialize-insecure root account setup. Empty
	// means "empty password" which is fine for local dev.
	RootPassword string
}

// EnsureMySQLDataDir initialises a fresh data directory when one does
// not exist yet. It supports both Oracle MySQL and MariaDB:
//
//   - MySQL: `mysqld --initialize-insecure --datadir=...`
//   - MariaDB (ships with Laragon / EnvKit and does not implement
//     --initialize): `mysql_install_db.exe --datadir=...` from the
//     same bin directory.
//
// It is idempotent: when the data dir already contains a `mysql` system
// schema directory the call is a no-op and returns nil.
func EnsureMySQLDataDir(mysqldPath, dataDir string) error {
	if strings.TrimSpace(mysqldPath) == "" {
		return fmt.Errorf("mysqld path is empty")
	}
	if strings.TrimSpace(dataDir) == "" {
		return fmt.Errorf("mysql data dir is empty")
	}

	// Already initialised? The server creates a `mysql` schema directory
	// plus ibdata files; the schema dir is the reliable marker.
	if _, err := os.Stat(filepath.Join(dataDir, "mysql")); err == nil {
		return nil
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}

	datadirFlag := "--datadir=" + forwardSlashes(dataDir)
	binDir := filepath.Dir(mysqldPath)
	installer := filepath.Join(binDir, "mysql_install_db.exe")
	if _, err := os.Stat(installer); err == nil {
		// MariaDB on Windows. --password empty = root with no password,
		// matching MySQL's --initialize-insecure behaviour.
		cmd := exec.Command(installer, datadirFlag, "--password=")
		cmd.SysProcAttr = sysproc.Hidden()
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("mysql_install_db failed: %v: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}

	// Oracle MySQL. --no-defaults stops mysqld from reading a my.ini that
	// belongs to another install (EnvKit / Laragon) and pointing our new
	// data dir at the wrong basedir.
	cmd := exec.Command(mysqldPath, "--no-defaults", "--initialize-insecure", datadirFlag)
	cmd.SysProcAttr = sysproc.Hidden()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mysqld --initialize-insecure failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// MySQLStartArgs builds the recommended mysqld command-line for a
// managed dev instance: console mode (so the process stays attached and
// our Manager can stop it), the generated data dir, and the port.
func MySQLStartArgs(d MySQLDefaults) []string {
	port := d.Port
	if port <= 0 {
		port = 3306
	}
	args := []string{
		"--console",
		"--basedir=" + forwardSlashes(d.BaseDir),
		"--datadir=" + forwardSlashes(d.DataDir),
		"--port=" + fmt.Sprint(port),
		"--bind-address=127.0.0.1",
	}
	if strings.TrimSpace(d.RootPassword) != "" {
		// Not a real secret at start time; documented MySQL bootstrap flag.
		args = append(args, "--initialize-root-password="+d.RootPassword)
	}
	return args
}

// PHPStartArgs builds the php-cgi command line for FastCGI on the given
// loopback port. One php-cgi process serves one request at a time, so we
// spawn several workers by default (matching php-fpm's pool behaviour as
// closely as a bare php-cgi can).
func PHPStartArgs(port int, workers int) []string {
	if port <= 0 {
		port = 9000
	}
	if workers <= 0 {
		workers = 4
	}
	// php-cgi has no native multi-worker mode; each worker is one
	// process bound to its own port (9000..9000+workers-1). The Manager
	// config is expected to register each worker as its own ServicePHP
	// instance or a single one — here we return args for the FIRST
	// worker; callers that want a pool can offset ports and call again.
	return []string{"-b", fmt.Sprintf("127.0.0.1:%d", port)}
}

// PHPWorkerPorts returns the loopback ports a PHP worker pool of the
// supplied size should listen on, starting at basePort.
func PHPWorkerPorts(basePort, workers int) []int {
	if basePort <= 0 {
		basePort = 9000
	}
	if workers <= 0 {
		workers = 1
	}
	ports := make([]int, 0, workers)
	for i := 0; i < workers; i++ {
		ports = append(ports, basePort+i)
	}
	return ports
}

// WaitForPort is a tiny readiness helper: it retries a TCP dial until the
// deadline. Callers use it after Start so the UI does not report success
// before nginx/mysqld actually bound their ports. It intentionally lives
// here rather than in Manager so it can be reused by CLI and desktop.
func WaitForPort(host string, port int, timeout time.Duration) error {
	if port <= 0 || port > 65535 {
		return fmt.Errorf("invalid port %d", port)
	}
	deadline := time.Now().Add(timeout)
	addr := net.JoinHostPort(host, fmt.Sprint(port))
	for {
		conn, err := net.Dial("tcp", addr)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for %s: %w", addr, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func forwardSlashes(path string) string {
	return strings.ReplaceAll(filepath.ToSlash(strings.TrimSpace(path)), `\`, `/`)
}
