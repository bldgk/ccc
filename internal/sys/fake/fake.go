// Package fake is an implementation of the sys contract for tests: scripted commands and
// a file system that exists only in memory.
package fake

import (
	"context"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"
	"sync"

	"github.com/bldgk/ccc/internal/sys"
)

// Result is what one run of a scripted command does.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
	// Hang makes the run block until its context ends.
	Hang bool
	// Err makes the run fail to start.
	Err error
	// Effect runs when the command runs, before it returns.
	Effect func()
}

func Out(stdout string) Result { return Result{Stdout: stdout} }

func Fail(exitCode int, stderr string) Result {
	return Result{ExitCode: exitCode, Stderr: stderr}
}

func Hang() Result { return Result{Hang: true} }

type kind int

const (
	dir kind = iota
	file
	symlink
)

type node struct {
	kind       kind
	size       int64
	unreadable bool
}

type System struct {
	mu          sync.Mutex
	osName      string
	home        string
	homeErr     error
	env         map[string]string
	executables map[string]bool
	scripts     map[string][]Result
	nodes       map[string]*node
	statErr     map[string]error
	removeErr   map[string]error
	usageHangs  map[string]bool
	ran         [][]string
	removed     []string
	contents    map[string][]byte
}

var _ sys.System = (*System)(nil)

func New(osName, home string) *System {
	return &System{
		osName:      osName,
		home:        home,
		env:         map[string]string{},
		executables: map[string]bool{},
		scripts:     map[string][]Result{},
		nodes:       map[string]*node{},
		statErr:     map[string]error{},
		removeErr:   map[string]error{},
		usageHangs:  map[string]bool{},
		contents:    map[string][]byte{},
	}
}

func key(argv []string) string { return strings.Join(argv, "\x00") }

// Script sets what running argv does. Each run consumes one result; the last result
// repeats.
func (s *System) Script(argv []string, results ...Result) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scripts[key(argv)] = results
}

func (s *System) SetEnv(name, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.env[name] = value
}

func (s *System) SetHomeError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.homeErr = err
}

func (s *System) AddExecutable(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.executables[name] = true
}

func (s *System) RemoveExecutable(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.executables, name)
}

// AddDir creates a directory whose own content takes size bytes, and its parents.
func (s *System) AddDir(p string, size int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.addParents(p)
	s.nodes[p] = &node{kind: dir, size: size}
}

func (s *System) addParents(p string) {
	for parent := path.Dir(p); parent != "/" && parent != "."; parent = path.Dir(parent) {
		if _, ok := s.nodes[parent]; !ok {
			s.nodes[parent] = &node{kind: dir}
		}
	}
}

// FileContent returns what CreateFile wrote at p.
func (s *System) FileContent(p string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.contents[p]
	return append([]byte(nil), data...), ok
}

// Created returns the path of every file CreateFile wrote, sorted.
func (s *System) Created() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Sorted(maps.Keys(s.contents))
}

func (s *System) AddFile(p string, size int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nodes[p] = &node{kind: file, size: size}
}

func (s *System) AddSymlink(p string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nodes[p] = &node{kind: symlink}
}

// SetUnreadable makes every disk usage that includes p fail.
func (s *System) SetUnreadable(p string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nodes[p].unreadable = true
}

func (s *System) SetSize(p string, size int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nodes[p].size = size
}

// Delete removes p and everything under it, as a tool's own clean command would.
func (s *System) Delete(p string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.delete(p)
}

func (s *System) delete(p string) {
	for name := range s.nodes {
		if name == p || strings.HasPrefix(name, p+"/") {
			delete(s.nodes, name)
			delete(s.contents, name)
		}
	}
}

func (s *System) SetStatError(p string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statErr[p] = err
}

func (s *System) SetRemoveError(p string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removeErr[p] = err
}

// HangDiskUsage makes measuring p block until the context ends.
func (s *System) HangDiskUsage(p string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.usageHangs[p] = true
}

// Ran returns every command run so far, in order.
func (s *System) Ran() [][]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([][]string(nil), s.ran...)
}

// Removed returns every path whose removal was requested, in order.
func (s *System) Removed() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.removed...)
}

func (s *System) Run(ctx context.Context, argv []string) (sys.CommandResult, error) {
	s.mu.Lock()
	s.ran = append(s.ran, append([]string(nil), argv...))
	k := key(argv)
	results, ok := s.scripts[k]
	if !ok || len(results) == 0 {
		s.mu.Unlock()
		return sys.CommandResult{}, fmt.Errorf("fake: no script for command %q", argv)
	}
	result := results[0]
	if len(results) > 1 {
		s.scripts[k] = results[1:]
	}
	s.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return sys.CommandResult{}, err
	}
	if result.Hang {
		<-ctx.Done()
		return sys.CommandResult{}, ctx.Err()
	}
	if result.Err != nil {
		return sys.CommandResult{}, result.Err
	}
	if result.Effect != nil {
		result.Effect()
	}
	return sys.CommandResult{Stdout: result.Stdout, Stderr: result.Stderr, ExitCode: result.ExitCode}, nil
}

func (s *System) LookPath(name string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.executables[name] {
		return "/fake/bin/" + name, true
	}
	return "", false
}

func (s *System) Getenv(name string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.env[name]
	return value, ok
}

func (s *System) HomeDir() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.home, s.homeErr
}

func (s *System) OS() string { return s.osName }

func (s *System) Stat(p string) (sys.Info, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.statErr[p]; err != nil {
		return sys.Info{}, err
	}
	n, ok := s.nodes[p]
	if !ok {
		return sys.Info{}, &fs.PathError{Op: "stat", Path: p, Err: fs.ErrNotExist}
	}
	return sys.Info{Dir: n.kind == dir, Symlink: n.kind == symlink}, nil
}

func (s *System) DiskUsage(ctx context.Context, p string) (int64, error) {
	s.mu.Lock()
	hangs := s.usageHangs[p]
	s.mu.Unlock()
	if hangs {
		<-ctx.Done()
		return 0, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.nodes[p]
	if !ok {
		return 0, &fs.PathError{Op: "usage", Path: p, Err: fs.ErrNotExist}
	}
	if n.kind != dir {
		return 0, fmt.Errorf("%s is not a directory", p)
	}
	var total int64
	for name, child := range s.nodes {
		if name != p && !strings.HasPrefix(name, p+"/") {
			continue
		}
		if child.unreadable {
			return 0, &fs.PathError{Op: "open", Path: name, Err: fs.ErrPermission}
		}
		total += child.size
	}
	return total, nil
}

func (s *System) RemoveDir(p string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removed = append(s.removed, p)
	if err := s.removeErr[p]; err != nil {
		return err
	}
	n, ok := s.nodes[p]
	if !ok {
		return &fs.PathError{Op: "remove", Path: p, Err: fs.ErrNotExist}
	}
	if n.kind != dir {
		return fmt.Errorf("refusing to remove %q: the path is not a directory", p)
	}
	s.delete(p)
	return nil
}

func (s *System) CreateFile(p string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.nodes[p]; ok {
		return &fs.PathError{Op: "create", Path: p, Err: fs.ErrExist}
	}
	s.addParents(p)
	s.nodes[p] = &node{kind: file, size: int64(len(data))}
	s.contents[p] = append([]byte(nil), data...)
	return nil
}
