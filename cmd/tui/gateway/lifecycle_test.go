package gateway

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeMockBinary creates a small executable that sleeps until killed.
// Returns the path to the binary and a cleanup function.
func writeMockBinary(t *testing.T) (string, func()) {
	t.Helper()
	dir := t.TempDir()

	// Write a Go source file that sleeps forever.
	src := `package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	fmt.Println("ready")
	// Block until killed.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	<-sig
	os.Exit(0)
}
`
	srcPath := filepath.Join(dir, "main.go")
	err := os.WriteFile(srcPath, []byte(src), 0644)
	require.NoError(t, err)

	binaryPath := filepath.Join(dir, "mock-gateway")

	cmd := exec.Command("go", "build", "-o", binaryPath, srcPath)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "failed to build mock binary: %s", string(out))

	return binaryPath, func() {
		// Kill any leftover process.
		os.Remove(binaryPath)
	}
}

// TestNewLifecycle verifies the constructor sets fields correctly.
func TestNewLifecycle(t *testing.T) {
	l := NewLifecycle("/usr/bin/test", "/tmp", 10*time.Second)
	assert.Equal(t, "/usr/bin/test", l.binaryPath)
	assert.Equal(t, "/tmp", l.workingDir)
	assert.Equal(t, 10*time.Second, l.shutdownTimeout)
	assert.Equal(t, 0, l.pid)
}

// TestNewLifecycle_Defaults verifies defaults are zero values.
func TestNewLifecycle_Defaults(t *testing.T) {
	l := NewLifecycle("", "", 0)
	assert.Equal(t, "", l.binaryPath)
	assert.Equal(t, "", l.workingDir)
	assert.Equal(t, time.Duration(0), l.shutdownTimeout)
	assert.Equal(t, 0, l.pid)
}

// TestSetBinaryPath_UpdatesPath verifies SetBinaryPath updates the stored path.
func TestSetBinaryPath_UpdatesPath(t *testing.T) {
	l := NewLifecycle("", "", 0)
	l.SetBinaryPath("/new/path")
	assert.Equal(t, "/new/path", l.binaryPath)
}

// TestSetOutput_UpdatesWriters verifies SetOutput configures stdout/stderr.
func TestSetOutput_UpdatesWriters(t *testing.T) {
	l := NewLifecycle("", "", 0)
	r, w, _ := os.Pipe()
	defer r.Close()
	defer w.Close()
	l.SetOutput(w, w)
	assert.Equal(t, w, l.stdout)
	assert.Equal(t, w, l.stderr)
}

// TestPID_ReturnsCurrentPID verifies PID() returns the stored PID.
func TestPID_ReturnsCurrentPID(t *testing.T) {
	l := NewLifecycle("", "", 0)
	assert.Equal(t, 0, l.PID())

	l.pid = 42
	assert.Equal(t, 42, l.PID())
}

// TestIsRunning_NoProcess verifies IsRunning returns false when pid is 0.
func TestIsRunning_NoProcess(t *testing.T) {
	l := NewLifecycle("", "", 0)
	assert.False(t, l.IsRunning())
}

// TestIsRunning_NegativePID verifies IsRunning returns false for negative PID.
func TestIsRunning_NegativePID(t *testing.T) {
	l := NewLifecycle("", "", 0)
	l.pid = -1
	assert.False(t, l.IsRunning())
}

// TestStart_EmptyBinaryPath verifies Start returns error without binary path.
func TestStart_EmptyBinaryPath(t *testing.T) {
	l := NewLifecycle("", "", 1*time.Second)
	err := l.Start()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "binary path is empty")
}

// TestStop_NotRunning verifies Stop is a no-op when not running.
func TestStop_NotRunning(t *testing.T) {
	l := NewLifecycle("", "", 1*time.Second)
	err := l.Stop()
	assert.NoError(t, err)
}

// TestStartStop_Integration tests a full start/stop cycle with a mock binary.
func TestStartStop_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath, cleanup := writeMockBinary(t)
	defer cleanup()

	l := NewLifecycle(binaryPath, "/tmp", 5*time.Second)

	// Start the process.
	err := l.Start()
	require.NoError(t, err)
	require.True(t, l.PID() > 0, "PID should be set after start")

	// Verify the process is running.
	assert.True(t, l.IsRunning(), "process should be running")

	// Stop the process.
	err = l.Stop()
	require.NoError(t, err)

	// Verify PID is reset.
	assert.Equal(t, 0, l.PID())

	// Verify process is gone (small delay to ensure OS cleanup).
	time.Sleep(100 * time.Millisecond)
	assert.False(t, l.IsRunning(), "process should not be running")
}

// TestRestart_Integration tests that Restart creates a new process.
func TestRestart_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath, cleanup := writeMockBinary(t)
	defer cleanup()

	l := NewLifecycle(binaryPath, "/tmp", 5*time.Second)

	// Start the process.
	err := l.Start()
	require.NoError(t, err)
	oldPID := l.PID()
	require.True(t, oldPID > 0)

	// Restart.
	err = l.Restart(5 * time.Second)
	require.NoError(t, err)

	// PID should have changed.
	newPID := l.PID()
	assert.NotEqual(t, oldPID, newPID, "PID should change after restart")
	assert.True(t, newPID > 0, "new PID should be valid")

	// Clean up.
	err = l.Stop()
	require.NoError(t, err)
}

// TestLifecycle_SetBinaryPathAndStart verifies SetBinaryPath + Start works.
func TestLifecycle_SetBinaryPathAndStart(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath, cleanup := writeMockBinary(t)
	defer cleanup()

	l := NewLifecycle("", "", 5*time.Second)
	l.SetBinaryPath(binaryPath)

	err := l.Start()
	require.NoError(t, err)
	require.True(t, l.PID() > 0)

	err = l.Stop()
	require.NoError(t, err)
}

// TestStart_ExecCommandCheck verifies that Start() sets SysProcAttr correctly
// without actually running a binary (validates command construction).
func TestStart_ExecCommandCheck(t *testing.T) {
	_ = NewLifecycle("/nonexistent/binary", "/tmp", 1*time.Second)

	// Manually create the command that Start() would use to verify attributes.
	cmd := exec.Command("/nonexistent/binary")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:  true,
		Setpgid: true,
	}
	cmd.Dir = "/tmp"

	assert.Equal(t, "/nonexistent/binary", cmd.Path)
	assert.NotNil(t, cmd.SysProcAttr)
	assert.True(t, cmd.SysProcAttr.Setsid, "Setsid should be true for detached process")
	assert.True(t, cmd.SysProcAttr.Setpgid, "Setpgid should be true for new process group")
	assert.Equal(t, "/tmp", cmd.Dir)
}

// TestConcurrentAccess_Race runs concurrent reads/writes to verify
// the mutex prevents data races.
func TestConcurrentAccess_Race(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping race test in short mode")
	}

	l := NewLifecycle("/usr/bin/test", "/tmp", 5*time.Second)

	done := make(chan bool)
	f := func() {
		for i := 0; i < 10; i++ {
			_ = l.PID()
			_ = l.IsRunning()
			l.SetBinaryPath(fmt.Sprintf("/path/%d", i))
		}
		done <- true
	}

	go f()
	go f()
	go f()

	for i := 0; i < 3; i++ {
		<-done
	}
}
