package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type State int

const (
	Pending State = iota
	Sized
	Absent
	Undetermined
	NotMeasured
)

type Row struct {
	Key     string
	Group   string
	Label   string
	Kind    Kind
	State   State
	Bytes   int64
	Partial bool   // some entries were unreadable; Bytes is a lower bound
	Note    string // reason for Undetermined / NotMeasured
	Path    string
}

// Event replaces (Replace) or upserts rows of one place.
type Event struct {
	Place   int
	Rows    []Row
	Replace bool
}

var errTimeout = errors.New("timed out")

func run(ctx context.Context, timeout time.Duration, argv ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 300 * time.Millisecond
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return "", errTimeout
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

func firstLine(s string) string {
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		if l := strings.TrimSpace(sc.Text()); l != "" {
			return l
		}
	}
	return ""
}

func anyOnPath(exes []string) bool {
	for _, e := range exes {
		if _, err := exec.LookPath(e); err == nil {
			return true
		}
	}
	return false
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func humanDur(d time.Duration) string {
	if d < time.Second {
		return d.String()
	}
	return strconv.FormatFloat(d.Seconds(), 'f', -1, 64) + "s"
}

// resolve returns the directory of a place: tool command, then environment
// variable, then per-OS default.
func resolve(ctx context.Context, p Place, timeout time.Duration) Row {
	r := Row{Group: p.Group, Label: p.Label, Kind: p.Kind}
	present := anyOnPath(p.Exes)
	base, cmdFailed := "", ""
	if present {
		for _, c := range p.PathCmds {
			if _, err := exec.LookPath(c[0]); err != nil {
				continue
			}
			out, err := run(ctx, timeout, c...)
			switch {
			case errors.Is(err, errTimeout):
				r.State, r.Note = Undetermined, "`"+strings.Join(c, " ")+"` timed out after "+humanDur(timeout)
				return r
			case err != nil || firstLine(out) == "":
				if !p.CmdOptional {
					cmdFailed = "`" + strings.Join(c, " ") + "` failed"
				}
				continue
			}
			base, cmdFailed = firstLine(out), ""
			break
		}
	}
	fromCmd := base != ""
	if base == "" && p.Env != "" {
		base = os.Getenv(p.Env)
	}
	guessed := false
	if base == "" && p.Default != nil {
		base, guessed = p.Default(hostOS), true
	}
	if base == "" {
		r.State, r.Note = Undetermined, "no path could be derived"
		return r
	}
	r.Path = filepath.Join(base, p.Sub)
	switch {
	case dirExists(r.Path):
		r.State = Pending
	case cmdFailed != "" && guessed && !fromCmd:
		r.State, r.Note, r.Path = Undetermined, cmdFailed, ""
	default:
		r.State = Absent
	}
	return r
}

var blockSize = int64(512)

// diskUsage sums allocated blocks, counting hard-linked files once.
func diskUsage(ctx context.Context, root string) (total int64, partial bool, err error) {
	type inode struct{ dev, ino uint64 }
	seen := map[inode]struct{}{}
	first := true
	werr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		if err != nil {
			if first {
				return err
			}
			partial = true
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		first = false
		info, ierr := d.Info()
		if ierr != nil {
			partial = true
			return nil
		}
		st, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			partial = true
			return nil
		}
		if !d.IsDir() && st.Nlink > 1 {
			k := inode{uint64(st.Dev), uint64(st.Ino)}
			if _, dup := seen[k]; dup {
				return nil
			}
			seen[k] = struct{}{}
		}
		total += int64(st.Blocks) * blockSize
		return nil
	})
	return total, partial, werr
}

func measureDir(ctx context.Context, r Row, timeout time.Duration) Row {
	mctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	n, partial, err := diskUsage(mctx, r.Path)
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		r.State, r.Note = NotMeasured, "timed out after "+humanDur(timeout)
	case err != nil:
		r.State, r.Note = NotMeasured, "unreadable"
	default:
		r.State, r.Bytes, r.Partial = Sized, n, partial
	}
	return r
}

// parseDockerSize reads strings like "1.234GB" or "0B" (decimal units).
func parseDockerSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	units := []struct {
		suf string
		mul float64
	}{{"TB", 1e12}, {"GB", 1e9}, {"MB", 1e6}, {"kB", 1e3}, {"KB", 1e3}, {"B", 1}}
	for _, u := range units {
		if strings.HasSuffix(s, u.suf) {
			f, err := strconv.ParseFloat(strings.TrimSuffix(s, u.suf), 64)
			if err != nil {
				return 0, err
			}
			return int64(f * u.mul), nil
		}
	}
	return 0, fmt.Errorf("unrecognised size %q", s)
}

type dockerRow struct{ Type, Size string }

// dockerSizes runs `docker system df` once per call; the two Docker places
// each call it, which is fine for a spike.
func dockerSizes(ctx context.Context, timeout time.Duration) (map[string]int64, error) {
	out, err := run(ctx, timeout, "docker", "system", "df", "--format", "{{json .}}")
	if err != nil {
		return nil, err
	}
	res := map[string]int64{}
	for _, line := range strings.Split(out, "\n") {
		var d dockerRow
		if json.Unmarshal([]byte(line), &d) != nil || d.Type == "" {
			continue
		}
		n, perr := parseDockerSize(d.Size)
		if perr != nil {
			return nil, perr
		}
		res[d.Type] = n
	}
	return res, nil
}

func measureDocker(ctx context.Context, p Place, timeout time.Duration) Row {
	r := Row{Group: p.Group, Label: p.Label, Kind: p.Kind}
	if !anyOnPath(p.Exes) {
		r.State = Absent
		return r
	}
	sizes, err := dockerSizes(ctx, timeout)
	switch {
	case errors.Is(err, errTimeout):
		r.State, r.Note = NotMeasured, "timed out after "+humanDur(timeout)
		return r
	case err != nil:
		r.State, r.Note = NotMeasured, "daemon not running"
		return r
	}
	typ := map[string]string{"images": "Images", "build cache": "Build Cache"}[p.Label]
	n, ok := sizes[typ]
	if !ok {
		r.State, r.Note = Undetermined, "no "+typ+" line in `docker system df`"
		return r
	}
	r.State, r.Bytes = Sized, n
	return r
}

// rustupHome asks rustup; no fallback exists for installed toolchains.
func measureRustup(ctx context.Context, idx int, p Place, timeout time.Duration, emit func(Event)) {
	fail := func(st State, note string) {
		emit(Event{Place: idx, Replace: true, Rows: []Row{{Key: "x", Group: p.Group, Label: p.Label, Kind: p.Kind, State: st, Note: note}}})
	}
	if !anyOnPath(p.Exes) {
		fail(Absent, "")
		return
	}
	list, err := run(ctx, timeout, "rustup", "toolchain", "list")
	if errors.Is(err, errTimeout) {
		fail(Undetermined, "`rustup toolchain list` timed out after "+humanDur(timeout))
		return
	}
	if err != nil {
		fail(Undetermined, "`rustup toolchain list` failed")
		return
	}
	var names []string
	for _, l := range strings.Split(list, "\n") {
		if f := strings.Fields(l); len(f) > 0 && !strings.HasPrefix(l, "no installed") {
			names = append(names, f[0])
		}
	}
	if len(names) == 0 {
		fail(Absent, "")
		return
	}
	hm, err := run(ctx, timeout, "rustup", "show", "home")
	if err != nil || firstLine(hm) == "" {
		fail(Undetermined, "`rustup show home` failed")
		return
	}
	var rows []Row
	for _, n := range names {
		rows = append(rows, Row{Key: n, Group: p.Group, Label: n, Kind: KindVersion, State: Pending, Path: filepath.Join(firstLine(hm), "toolchains", n)})
	}
	emit(Event{Place: idx, Replace: true, Rows: rows})
	var wg sync.WaitGroup
	for _, r := range rows {
		wg.Add(1)
		go func() {
			defer wg.Done()
			emit(Event{Place: idx, Rows: []Row{measureDir(ctx, r, timeout)}})
		}()
	}
	wg.Wait()
}

// Start launches every place in parallel; the channel closes when all are done.
func Start(ctx context.Context, places []Place, timeout time.Duration) <-chan Event {
	ch := make(chan Event, 64)
	var wg sync.WaitGroup
	emit := func(e Event) { ch <- e }
	for i, p := range places {
		wg.Add(1)
		go func() {
			defer wg.Done()
			switch p.Special {
			case "rustup":
				measureRustup(ctx, i, p, timeout, emit)
			case "docker":
				r := measureDocker(ctx, p, timeout)
				r.Key = p.Label
				emit(Event{Place: i, Rows: []Row{r}})
			default:
				r := resolve(ctx, p, timeout)
				r.Key = p.Label
				if r.State == Pending {
					r = measureDir(ctx, r, timeout)
				}
				emit(Event{Place: i, Rows: []Row{r}})
			}
		}()
	}
	go func() { wg.Wait(); close(ch) }()
	return ch
}
