//go:build darwin || linux

package host

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// tempDir is a temporary directory under its real path, so that a test compares paths
// the way the kernel reports them.
func tempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func mkdir(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func write(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func allocated(t *testing.T, paths ...string) int64 {
	t.Helper()
	var total int64
	for _, path := range paths {
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		total += int64(info.Sys().(*syscall.Stat_t).Blocks) * statBlockSize
	}
	return total
}

func hostWithHome(home string) *Host {
	h := New(DefaultSettings())
	h.home = func() (string, error) { return home, nil }
	return h
}

func TestDiskUsageSumsAllocatedBlocks(t *testing.T) {
	root := tempDir(t)
	sub := mkdir(t, filepath.Join(root, "sub"))
	small := filepath.Join(root, "small")
	big := filepath.Join(sub, "big")
	write(t, small, 1)
	write(t, big, 300_000)

	got, err := New(DefaultSettings()).DiskUsage(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if want := allocated(t, root, sub, small, big); got != want {
		t.Fatalf("got %d, want the allocated blocks %d", got, want)
	}
	if got < 300_000 {
		t.Fatalf("got %d, less than the content of the files", got)
	}
	if blocks := allocated(t, small); blocks <= 1 {
		t.Fatalf("a one-byte file is reported with %d allocated bytes: the test cannot tell blocks from length", blocks)
	}
}

func TestDiskUsageCountsAHardLinkedFileOnce(t *testing.T) {
	root := tempDir(t)
	first := filepath.Join(root, "first")
	write(t, first, 200_000)
	h := New(DefaultSettings())
	before, err := h.DiskUsage(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}

	sub := mkdir(t, filepath.Join(root, "sub"))
	if err := os.Link(first, filepath.Join(sub, "second")); err != nil {
		t.Fatal(err)
	}
	after, err := h.DiskUsage(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if want := allocated(t, root, sub, first); after != want {
		t.Fatalf("with a second link: got %d, want %d", after, want)
	}
	if after-before >= allocated(t, first) {
		t.Fatalf("the second link added %d bytes; the file takes %d", after-before, allocated(t, first))
	}
}

func TestDiskUsageDoesNotFollowSymbolicLinks(t *testing.T) {
	base := tempDir(t)
	outside := mkdir(t, filepath.Join(base, "outside"))
	write(t, filepath.Join(outside, "big"), 500_000)
	root := mkdir(t, filepath.Join(base, "root"))
	toDir := filepath.Join(root, "to-dir")
	toFile := filepath.Join(root, "to-file")
	if err := os.Symlink(outside, toDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "big"), toFile); err != nil {
		t.Fatal(err)
	}

	got, err := New(DefaultSettings()).DiskUsage(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if want := allocated(t, root, toDir, toFile); got != want {
		t.Fatalf("got %d, want only the directory and the links themselves, %d", got, want)
	}
	if got >= 500_000 {
		t.Fatalf("got %d: the link target was counted", got)
	}
}

func TestDiskUsageRefusesARootThatIsNotADirectory(t *testing.T) {
	base := tempDir(t)
	real := mkdir(t, filepath.Join(base, "real"))
	link := filepath.Join(base, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(base, "file")
	write(t, file, 10)
	h := New(DefaultSettings())

	if _, err := h.DiskUsage(context.Background(), link); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Errorf("symbolic link as the root: %v", err)
	}
	if _, err := h.DiskUsage(context.Background(), file); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("file as the root: %v", err)
	}
	if _, err := h.DiskUsage(context.Background(), filepath.Join(base, "absent")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("absent root: %v", err)
	}
}

func TestDiskUsageFailsOnAnUnreadablePart(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("the administrator can read every directory")
	}
	root := tempDir(t)
	write(t, filepath.Join(root, "readable"), 100_000)
	locked := mkdir(t, filepath.Join(root, "locked"))
	write(t, filepath.Join(locked, "hidden"), 100_000)
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })

	got, err := New(DefaultSettings()).DiskUsage(context.Background(), root)
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("got %d, %v; want a permission error", got, err)
	}
	if got != 0 {
		t.Fatalf("a partial sum %d came back with the error", got)
	}
}

func TestDiskUsageStopsWhenTheContextEnds(t *testing.T) {
	root := tempDir(t)
	write(t, filepath.Join(root, "file"), 10)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := New(DefaultSettings()).DiskUsage(ctx, root); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want the context's error", err)
	}
}

func TestRemovalGuards(t *testing.T) {
	base := tempDir(t)
	home := mkdir(t, filepath.Join(base, "home", "alice"))
	h := hostWithHome(home)

	cache := mkdir(t, filepath.Join(home, "cache"))
	real := mkdir(t, filepath.Join(base, "real"))
	link := filepath.Join(base, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(base, "file")
	write(t, file, 1)
	homes := filepath.Join(base, "homes")
	if err := os.Symlink(filepath.Join(base, "home"), homes); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		path   string
		reason string
	}{
		{"the root", "/", "the path is the root"},
		{"the home directory", home, "the path is the home directory"},
		{"the parent of the home directory", filepath.Dir(home), "the path contains the home directory"},
		{"a far ancestor of the home directory", "/" + strings.Split(strings.TrimPrefix(home, "/"), "/")[0], "the path contains the home directory"},
		{"one level below the root", "/ccc-test-shallow", "less than 2 levels below the root"},
		{"a symbolic link", link, "the path is a symbolic link"},
		{"a relative path", "home/alice/cache", "the path is not absolute"},
		{"a path with ..", cache + "/../cache", "the path contains .."},
		{"a path with .. at the end", cache + "/..", "the path contains .."},
		{"a path that is not clean", cache + "/", "not in its clean form"},
		{"a path with a doubled separator", home + "//cache", "not in its clean form"},
		{"a file", file, "the path is not a directory"},
		{"a path that does not exist", filepath.Join(base, "absent", "dir"), "no such file"},
		{"the home directory through a linked parent", filepath.Join(homes, "alice"), "the path is the home directory"},
		{"the parent of the home directory through a link", homes, "the path is a symbolic link"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := h.checkRemoval(c.path)
			if err == nil {
				t.Fatalf("removal of %q was allowed", c.path)
			}
			if !strings.Contains(err.Error(), "refusing to remove") || !strings.Contains(err.Error(), c.reason) {
				t.Fatalf("got %q, want a refusal containing %q", err, c.reason)
			}
		})
	}

	if err := h.checkRemoval(cache); err != nil {
		t.Fatalf("a directory inside the home directory was refused: %v", err)
	}
}

func TestRemovalGuardNeedsTheHomeDirectory(t *testing.T) {
	base := tempDir(t)
	dir := mkdir(t, filepath.Join(base, "cache"))
	h := New(DefaultSettings())
	h.home = func() (string, error) { return "", errors.New("no home") }
	if err := h.checkRemoval(dir); err == nil || !strings.Contains(err.Error(), "home directory is unknown") {
		t.Fatalf("got %v, want a refusal because the home directory is unknown", err)
	}
	h.home = func() (string, error) { return "relative/home", nil }
	if err := h.checkRemoval(dir); err == nil || !strings.Contains(err.Error(), "home directory is unknown") {
		t.Fatalf("got %v, want a refusal because the home directory is not absolute", err)
	}
}

func TestRemoveDirRefusesAndLeavesEverythingInPlace(t *testing.T) {
	base := tempDir(t)
	home := mkdir(t, filepath.Join(base, "home", "alice"))
	keep := filepath.Join(home, "keep")
	write(t, keep, 10)
	real := mkdir(t, filepath.Join(base, "real"))
	inside := filepath.Join(real, "inside")
	write(t, inside, 10)
	link := filepath.Join(base, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	h := hostWithHome(home)

	for _, path := range []string{home, filepath.Dir(home), link, real + "/../real"} {
		if err := h.RemoveDir(path); err == nil || !strings.Contains(err.Error(), "refusing to remove") {
			t.Errorf("RemoveDir(%q): %v", path, err)
		}
	}
	for _, path := range []string{keep, inside, link} {
		if _, err := os.Lstat(path); err != nil {
			t.Errorf("%s is gone after a refused removal: %v", path, err)
		}
	}
}

func TestRemoveDirRemovesATree(t *testing.T) {
	base := tempDir(t)
	home := mkdir(t, filepath.Join(base, "home", "alice"))
	cache := mkdir(t, filepath.Join(home, "cache"))
	write(t, filepath.Join(mkdir(t, filepath.Join(cache, "a", "b")), "file"), 100)
	outside := mkdir(t, filepath.Join(base, "outside"))
	kept := filepath.Join(outside, "kept")
	write(t, kept, 100)
	if err := os.Symlink(outside, filepath.Join(cache, "to-outside")); err != nil {
		t.Fatal(err)
	}

	if err := hostWithHome(home).RemoveDir(cache); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(cache); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the directory is still there: %v", err)
	}
	if _, err := os.Lstat(kept); err != nil {
		t.Fatalf("removal went through a symbolic link: %v", err)
	}
	if _, err := os.Lstat(home); err != nil {
		t.Fatalf("the parent is gone: %v", err)
	}
}

func TestRemoveDirRemovesAReadOnlyTree(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("the administrator is not stopped by read-only directories")
	}
	base := tempDir(t)
	home := mkdir(t, filepath.Join(base, "home", "alice"))
	cache := mkdir(t, filepath.Join(home, "modcache"))
	deep := mkdir(t, filepath.Join(cache, "module@v1", "pkg"))
	write(t, filepath.Join(deep, "source.go"), 100)
	write(t, filepath.Join(cache, "module@v1", "go.mod"), 100)

	outside := filepath.Join(base, "outside")
	write(t, outside, 100)
	if err := os.Chmod(outside, 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(outside, filepath.Join(deep, "linked")); err != nil {
		t.Fatal(err)
	}

	readOnly := []string{filepath.Join(deep, "source.go"), deep, filepath.Join(cache, "module@v1"), cache}
	for _, path := range readOnly {
		mode := os.FileMode(0o555)
		if !strings.HasSuffix(path, "pkg") && !strings.HasSuffix(path, "v1") && path != cache {
			mode = 0o444
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, path := range readOnly {
			os.Chmod(path, 0o755)
		}
	})
	if err := os.RemoveAll(filepath.Join(cache, "module@v1")); err == nil {
		t.Fatal("the tree is not read-only: plain removal succeeded")
	}

	if err := hostWithHome(home).RemoveDir(cache); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(cache); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the directory is still there: %v", err)
	}
	info, err := os.Lstat(outside)
	if err != nil {
		t.Fatalf("a file linked from the tree is gone: %v", err)
	}
	if info.Mode().Perm() != 0o444 {
		t.Fatalf("the mode of a file linked from the tree changed to %v", info.Mode().Perm())
	}
}

func TestRunReportsOutputAndExitStatus(t *testing.T) {
	h := New(DefaultSettings())
	result, err := h.Run(context.Background(), []string{"sh", "-c", "echo out; echo err >&2; exit 3"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Stdout != "out\n" || result.Stderr != "err\n" || result.ExitCode != 3 {
		t.Fatalf("got %+v", result)
	}
}

func TestRunPassesArgumentsWithoutAShell(t *testing.T) {
	h := New(DefaultSettings())
	result, err := h.Run(context.Background(), []string{"echo", "$HOME", "*", "a  b", "|", "x"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Stdout != "$HOME * a  b | x\n" {
		t.Fatalf("got %q", result.Stdout)
	}
}

func TestRunGivesNoInput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := New(DefaultSettings()).Run(ctx, []string{"cat"})
	if err != nil {
		t.Fatalf("a command that reads its input did not end at once: %v", err)
	}
	if result.Stdout != "" || result.ExitCode != 0 {
		t.Fatalf("got %+v", result)
	}
}

func TestRunEndsACommandAtTheTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := New(DefaultSettings()).Run(ctx, []string{"sh", "-c", "sleep 30 & sleep 30"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v, want the deadline error", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("the command was not ended at the timeout: %s", elapsed)
	}
}

func TestRunFailsForAnUnknownProgram(t *testing.T) {
	if _, err := New(DefaultSettings()).Run(context.Background(), []string{"ccc-test-no-such-program"}); err == nil {
		t.Fatal("no error")
	}
}

func TestStatDoesNotFollowASymbolicLink(t *testing.T) {
	base := tempDir(t)
	dir := mkdir(t, filepath.Join(base, "dir"))
	link := filepath.Join(base, "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	h := New(DefaultSettings())
	if info, err := h.Stat(dir); err != nil || !info.Dir || info.Symlink {
		t.Errorf("directory: %+v, %v", info, err)
	}
	if info, err := h.Stat(link); err != nil || info.Dir || !info.Symlink {
		t.Errorf("link: %+v, %v", info, err)
	}
	if _, err := h.Stat(filepath.Join(base, "absent")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("absent: %v", err)
	}
}

func TestCreateFileWritesANewFileAndItsDirectories(t *testing.T) {
	path := filepath.Join(tempDir(t), "config", "registry", "entry.toml")
	if err := hostWithHome(tempDir(t)).CreateFile(path, []byte("first")); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "first" {
		t.Fatalf("the file holds %q, %v", data, err)
	}
}

func TestCreateFileNeverReplacesWhatIsThere(t *testing.T) {
	path := filepath.Join(tempDir(t), "entry.toml")
	h := hostWithHome(tempDir(t))
	if err := h.CreateFile(path, []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := h.CreateFile(path, []byte("second")); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("the error is %v, want one that wraps fs.ErrExist", err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "first" {
		t.Fatalf("the file holds %q, %v", data, err)
	}
}
