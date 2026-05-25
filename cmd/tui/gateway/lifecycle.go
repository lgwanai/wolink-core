package gateway

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// Lifecycle manages the gateway process lifecycle: build, start, stop, restart.
// The gateway runs as a detached child process — exiting the TUI does NOT stop it.
type Lifecycle struct {
	mu               sync.RWMutex
	binaryPath       string
	workingDir       string
	shutdownTimeout  time.Duration
	pid              int
	stdout           io.Writer
	stderr           io.Writer
}

// NewLifecycle creates a new GatewayLifecycle.
// binaryPath: path to the gateway binary (empty means build from source with default path).
// workingDir: working directory for the gateway process.
// shutdownTimeout: how long to wait for graceful shutdown after SIGTERM.
func NewLifecycle(binaryPath string, workingDir string, shutdownTimeout time.Duration) *Lifecycle {
	return &Lifecycle{
		binaryPath:      binaryPath,
		workingDir:      workingDir,
		shutdownTimeout: shutdownTimeout,
		pid:             0,
		stdout:          os.Stdout,
		stderr:          os.Stderr,
	}
}

// SetOutput sets custom stdout and stderr writers for the gateway process.
func (l *Lifecycle) SetOutput(stdout, stderr io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stdout = stdout
	l.stderr = stderr
}

// SetBinaryPath updates the gateway binary path.
func (l *Lifecycle) SetBinaryPath(path string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.binaryPath = path
}

// PID returns the current gateway process PID (0 if not running).
func (l *Lifecycle) PID() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.pid
}

// IsRunning checks whether the gateway process is still alive.
func (l *Lifecycle) IsRunning() bool {
	l.mu.RLock()
	pid := l.pid
	l.mu.RUnlock()

	if pid <= 0 {
		return false
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Signal 0 is a no-op that checks process existence.
	return proc.Signal(syscall.Signal(0)) == nil
}

// EnsureBuilt builds the gateway binary from source if no binary path is set.
// Runs `go build -o {binaryPath} ./cmd/` inside the working directory.
// If binaryPath is empty, defaults to "./bin/gateway".
func (l *Lifecycle) EnsureBuilt() error {
	path := l.binaryPath
	if path == "" {
		path = "./bin/gateway"
	}

	cmd := exec.Command("go", "build", "-o", path, "./cmd/")
	cmd.Dir = l.workingDir

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to capture build stderr: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("build start failed: %w", err)
	}

	errOutput, _ := io.ReadAll(stderr)

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("build failed: %w\nstderr:\n%s", err, string(errOutput))
	}

	// Store the absolute path for later use.
	l.mu.Lock()
	l.binaryPath = path
	l.mu.Unlock()
	return nil
}

// Start starts the gateway process as a detached child.
// Uses Setsid+Setpgid to create a new process group so the gateway
// continues running after the TUI exits.
func (l *Lifecycle) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	binaryPath := l.binaryPath
	if binaryPath == "" {
		return fmt.Errorf("binary path is empty; call EnsureBuilt() or SetBinaryPath() first")
	}

	cmd := exec.Command(binaryPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:  true, // New session — no controlling terminal
		Setpgid: true, // New process group
	}
	cmd.Dir = l.workingDir
	cmd.Stdout = l.stdout
	cmd.Stderr = l.stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("gateway start failed: %w", err)
	}

	// Release the process so it continues running even if the TUI exits.
	if err := cmd.Process.Release(); err != nil {
		return fmt.Errorf("process release failed: %w", err)
	}

	l.pid = cmd.Process.Pid
	return nil
}

// Stop sends SIGTERM to the gateway process and waits for it to exit
// within the shutdown timeout. If the timeout expires, it logs a warning
// but does NOT send SIGKILL (graceful shutdown design).
func (l *Lifecycle) Stop() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.pid <= 0 {
		return nil // Not running
	}

	proc, err := os.FindProcess(l.pid)
	if err != nil {
		l.pid = 0
		return nil // Process no longer exists
	}

	// Send SIGTERM for graceful shutdown.
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		l.pid = 0
		return nil // Process already dead
	}

	// Wait for process to exit within the timeout.
	done := make(chan bool, 1)
	go func() {
		proc.Wait()
		done <- true
	}()

	select {
	case <-done:
		// Process exited gracefully.
	case <-time.After(l.shutdownTimeout):
		// Timeout — log warning but do not SIGKILL.
		fmt.Fprintf(os.Stderr, "WARNING: gateway process (PID %d) did not exit within %v; leaving it running\n", l.pid, l.shutdownTimeout)
	}

	l.pid = 0
	return nil
}

// Restart stops the running gateway, rebuilds if needed, then starts again.
func (l *Lifecycle) Restart(shutdownTimeout time.Duration) error {
	if err := l.Stop(); err != nil {
		return fmt.Errorf("stop failed: %w", err)
	}

	// Rebuild from source if the binary path is the default one.
	if l.binaryPath == "" || l.binaryPath == "./bin/gateway" {
		if err := l.EnsureBuilt(); err != nil {
			return fmt.Errorf("rebuild failed: %w", err)
		}
	}

	if err := l.Start(); err != nil {
		return fmt.Errorf("start failed: %w", err)
	}

	return nil
}
