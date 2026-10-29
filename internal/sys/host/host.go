//go:build darwin || linux

// Package host is the real implementation of the sys contract.
package host

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"

	"github.com/bldgk/ccc/internal/sys"
)

// Settings are the knobs of the real implementation.
type Settings struct {
	// KillGrace is how long a command that was killed at its timeout may keep its
	// output open before waiting for it is given up.
	KillGrace time.Duration
}

func DefaultSettings() Settings {
	return Settings{KillGrace: 500 * time.Millisecond}
}

type Host struct {
	settings Settings
	home     func() (string, error)
}

var _ sys.System = (*Host)(nil)

func New(settings Settings) *Host {
	return &Host{settings: settings, home: os.UserHomeDir}
}

func (h *Host) Run(ctx context.Context, argv []string) (sys.CommandResult, error) {
	if len(argv) == 0 {
		return sys.CommandResult{}, errors.New("empty command")
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	// The command gets its own process group so that a timeout also ends what it started.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = h.settings.KillGrace
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := sys.CommandResult{Stdout: stdout.String(), Stderr: stderr.String()}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return result, ctxErr
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		result.ExitCode = exit.ExitCode()
		if result.ExitCode < 0 {
			return result, err
		}
		return result, nil
	}
	return result, err
}

func (h *Host) LookPath(name string) (string, bool) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", false
	}
	return path, true
}

func (h *Host) Getenv(name string) (string, bool) {
	return os.LookupEnv(name)
}

func (h *Host) HomeDir() (string, error) {
	return h.home()
}

func (h *Host) OS() string {
	if runtime.GOOS == "darwin" {
		return sys.MacOS
	}
	return sys.Linux
}

func (h *Host) Stat(path string) (sys.Info, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return sys.Info{}, err
	}
	return sys.Info{Dir: info.IsDir(), Symlink: info.Mode()&os.ModeSymlink != 0}, nil
}
