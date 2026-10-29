// Package sys is the contract between ccc and the machine it runs on.
package sys

import "context"

const (
	MacOS = "macos"
	Linux = "linux"
)

type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type Info struct {
	Dir     bool
	Symlink bool
}

type Runner interface {
	// Run starts argv directly, without a shell and without input, and waits for it.
	// A command that ran and exited with a non-zero status is not an error: the status
	// is in the result. The error is non-nil when the command could not be started or
	// ctx ended first; in the second case it is ctx's error.
	Run(ctx context.Context, argv []string) (CommandResult, error)
	// LookPath reports whether an executable of that name is on PATH, and where.
	LookPath(name string) (string, bool)
}

type Environment interface {
	// Getenv reports the value of a variable and whether it is set at all.
	Getenv(name string) (string, bool)
	HomeDir() (string, error)
	// OS is MacOS or Linux.
	OS() string
}

type Files interface {
	// Stat describes what is at path itself, without following a symbolic link. The
	// error wraps fs.ErrNotExist when nothing is there.
	Stat(path string) (Info, error)
	// DiskUsage is the number of bytes allocated to the directory and everything in
	// it. It never returns a partial sum: when any part cannot be read it fails.
	DiskUsage(ctx context.Context, path string) (int64, error)
	// RemoveDir deletes the directory and everything in it. The caller passes the
	// path exactly as detection resolved it; an implementation refuses a path that is
	// not safe to delete.
	RemoveDir(path string) error
	// CreateFile writes data to a new file at path and creates the directories above
	// it that are missing. It never replaces anything: when something is at path
	// already, the error wraps fs.ErrExist and nothing is changed.
	CreateFile(path string, data []byte) error
}

type System interface {
	Runner
	Environment
	Files
}
