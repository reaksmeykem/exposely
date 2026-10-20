// Package terminal runs a single project command in the background and
// streams its output line by line. It deliberately avoids a full PTY: the
// UI only needs a console that can run long-lived commands such as
// `npm run dev`, send occasional stdin lines, and be stopped again.
package terminal

import (
	"bufio"
	"errors"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/reaksmeykem/exposely/internal/sysproc"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07]*\x07`)

// StripANSI removes terminal colour/control sequences so the output reads
// as plain text in the UI.
func StripANSI(value string) string {
	return ansiPattern.ReplaceAllString(value, "")
}

// Event is one line of output from a running command.
type Event struct {
	Stream string
	Line   string
}

// Exit reports that the command finished.
type Exit struct {
	Code int
	Err  error
}

// Status describes the currently running command.
type Status struct {
	Running bool
	PID     int
	Command string
	Dir     string
}

// Runner owns at most one running command.
type Runner struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	command string
	dir     string
	running bool
}

// Start launches command inside dir. Output is delivered line by line to
// onEvent and completion to onExit (both called from goroutines).
func (r *Runner) Start(dir, command string, onEvent func(Event), onExit func(Exit)) (int, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return 0, errors.New("command is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.running {
		return 0, errors.New("a terminal command is already running")
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", command)
		cmd.SysProcAttr = sysproc.Hidden()
	} else {
		cmd = exec.Command("sh", "-lc", command)
	}
	cmd.Dir = dir
	cmd.Env = os.Environ()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 0, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return 0, err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return 0, err
	}
	if err := cmd.Start(); err != nil {
		return 0, err
	}

	r.cmd = cmd
	r.stdin = stdin
	r.command = command
	r.dir = dir
	r.running = true

	go streamLines("stdout", stdout, onEvent)
	go streamLines("stderr", stderr, onEvent)
	go func() {
		waitErr := cmd.Wait()
		r.mu.Lock()
		if r.cmd == cmd {
			r.running = false
			r.cmd = nil
			r.stdin = nil
		}
		r.mu.Unlock()
		code := 0
		if cmd.ProcessState != nil {
			code = cmd.ProcessState.ExitCode()
		}
		if onExit != nil {
			onExit(Exit{Code: code, Err: waitErr})
		}
	}()

	return cmd.Process.Pid, nil
}

func streamLines(stream string, pipe io.ReadCloser, onEvent func(Event)) {
	defer pipe.Close()
	scanner := bufio.NewScanner(pipe)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := StripANSI(strings.TrimRight(scanner.Text(), "\r"))
		if onEvent != nil {
			onEvent(Event{Stream: stream, Line: line})
		}
	}
}

// Stop terminates the whole process tree so shells and their children
// (npm -> node, artisan -> php) do not survive the stop button.
func (r *Runner) Stop() error {
	r.mu.Lock()
	cmd := r.cmd
	r.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	if runtime.GOOS == "windows" {
		return exec.Command("taskkill", "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F").Run()
	}
	return cmd.Process.Kill()
}

// SendInput writes one line to the command's stdin.
func (r *Runner) SendInput(input string) error {
	r.mu.Lock()
	stdin := r.stdin
	running := r.running
	r.mu.Unlock()
	if !running || stdin == nil {
		return errors.New("no terminal command is running")
	}
	_, err := io.WriteString(stdin, input+"\n")
	return err
}

// Status returns a snapshot of the current command.
func (r *Runner) Status() Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	status := Status{
		Running: r.running,
		Command: r.command,
		Dir:     r.dir,
	}
	if r.cmd != nil && r.cmd.Process != nil {
		status.PID = r.cmd.Process.Pid
	}
	return status
}
