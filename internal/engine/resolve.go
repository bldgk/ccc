package engine

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/bldgk/ccc/internal/entry"
)

func (e *Engine) onPath(t target) bool {
	if t.executable == "" {
		return false
	}
	_, found := e.sys.LookPath(t.executable)
	return found
}

// run executes one command of an entry and returns what it printed. Failing to start,
// a non-zero exit status and the timeout are all errors that carry the tool's message.
func (e *Engine) run(ctx context.Context, argv []string, timeout time.Duration) (string, error) {
	shown := strings.Join(argv, " ")
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	result, err := e.sys.Run(runCtx, argv)
	if err != nil {
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) && ctx.Err() == nil {
			return "", fmt.Errorf("`%s` timed out after %s", shown, timeout)
		}
		return "", fmt.Errorf("`%s`: %w", shown, err)
	}
	if result.ExitCode != 0 {
		message := strings.TrimSpace(result.Stderr)
		if message == "" {
			message = strings.TrimSpace(result.Stdout)
		}
		if message == "" {
			return "", fmt.Errorf("`%s`: exit status %d", shown, result.ExitCode)
		}
		return "", fmt.Errorf("`%s`: exit status %d: %s", shown, result.ExitCode, message)
	}
	return result.Stdout, nil
}

// command splits a command of an entry and fills in the placeholders. Substitution
// happens after splitting, so a path with a space stays one argument.
func command(text, placePath, version string) ([]string, error) {
	argv, err := entry.SplitCommand(text)
	if err != nil {
		return nil, fmt.Errorf("`%s`: %w", text, err)
	}
	for i := range argv {
		argv[i] = substitute(argv[i], placePath, version)
	}
	return argv, nil
}

func substitute(text, placePath, version string) string {
	return strings.NewReplacer(
		entry.PathPlaceholder, placePath,
		entry.VersionPlaceholder, version,
	).Replace(text)
}

// resolvePath finds the directory of a place. It returns the path of the first source
// that gave one, or an empty path and why each source gave nothing.
func (e *Engine) resolvePath(ctx context.Context, t target, onPath bool) (string, []string) {
	if t.kind == entry.Folder {
		p, err := e.expand(t.folderPath)
		if err != nil {
			return "", []string{err.Error()}
		}
		return p, nil
	}

	spec := t.place.Path
	var reasons []string
	note := func(source string, err error) {
		reasons = append(reasons, source+": "+err.Error())
	}

	if spec.Command != "" {
		p, err := e.pathFromCommand(ctx, t, onPath)
		if err == nil {
			return path.Join(p, spec.Subdir), nil
		}
		note("path.command", err)
	}
	if spec.Env != "" {
		p, err := e.pathFromEnv(spec.Env)
		if err == nil {
			return path.Join(p, spec.Subdir), nil
		}
		note("path.env", err)
	}
	os := e.sys.OS()
	if fallback, ok := spec.Default[os]; ok {
		p, err := e.expand(fallback)
		if err == nil {
			return p, nil
		}
		note("path.default."+os, err)
	} else {
		reasons = append(reasons, "path.default: no default for "+os)
	}
	return "", reasons
}

func (e *Engine) pathFromCommand(ctx context.Context, t target, onPath bool) (string, error) {
	if !onPath {
		return "", fmt.Errorf("%s is not on PATH", t.executable)
	}
	argv, err := command(t.place.Path.Command, "", "")
	if err != nil {
		return "", err
	}
	out, err := e.run(ctx, argv, e.settings.CommandTimeout)
	if err != nil {
		return "", err
	}
	line, _, _ := strings.Cut(out, "\n")
	line = strings.TrimSpace(line)
	if line == "" {
		return "", fmt.Errorf("`%s` printed nothing", strings.Join(argv, " "))
	}
	return absolute(line)
}

func (e *Engine) pathFromEnv(name string) (string, error) {
	value, set := e.sys.Getenv(name)
	if !set || value == "" {
		return "", fmt.Errorf("%s is not set", name)
	}
	return absolute(value)
}

func absolute(p string) (string, error) {
	if !path.IsAbs(p) {
		return "", fmt.Errorf("%q is not an absolute path", p)
	}
	return path.Clean(p), nil
}

// expand replaces a leading ~ with the home directory and $NAME or ${NAME} with the
// value of that variable. A variable that is not set is an error: the text then names
// no path at all.
func (e *Engine) expand(text string) (string, error) {
	var out strings.Builder
	rest := text
	if rest == "~" || strings.HasPrefix(rest, "~/") {
		home, err := e.sys.HomeDir()
		if err != nil {
			return "", fmt.Errorf("home directory: %w", err)
		}
		if home == "" {
			return "", errors.New("home directory is not known")
		}
		out.WriteString(home)
		rest = rest[1:]
	}
	for {
		i := strings.IndexByte(rest, '$')
		if i < 0 {
			out.WriteString(rest)
			break
		}
		out.WriteString(rest[:i])
		rest = rest[i+1:]

		var name string
		if strings.HasPrefix(rest, "{") {
			end := strings.IndexByte(rest, '}')
			if end < 0 {
				return "", fmt.Errorf("%q has a ${ that is not closed", text)
			}
			name, rest = rest[1:end], rest[end+1:]
			if !isVariableName(name) {
				return "", fmt.Errorf("%q: %q is not the name of a variable", text, name)
			}
		} else {
			n := 0
			for n < len(rest) && isVariableChar(rest[n], n == 0) {
				n++
			}
			if n == 0 {
				out.WriteByte('$')
				continue
			}
			name, rest = rest[:n], rest[n:]
		}
		value, set := e.sys.Getenv(name)
		if !set || value == "" {
			return "", fmt.Errorf("%s is not set", name)
		}
		out.WriteString(value)
	}
	return absolute(out.String())
}

func isVariableChar(c byte, first bool) bool {
	switch {
	case c == '_', c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z':
		return true
	case c >= '0' && c <= '9':
		return !first
	}
	return false
}

func isVariableName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		if !isVariableChar(name[i], i == 0) {
			return false
		}
	}
	return true
}

// rowDir is the directory one row stands for: measure.path with its placeholders filled
// in, or the path of the place. It never lies outside the path of the place.
func rowDir(t target, placePath, version string) (string, error) {
	if placePath == "" {
		return "", nil
	}
	if t.place.Measure.Path == "" {
		return placePath, nil
	}
	dir := path.Clean(substitute(t.place.Measure.Path, placePath, version))
	if dir != placePath && !strings.HasPrefix(dir, strings.TrimSuffix(placePath, "/")+"/") {
		return "", fmt.Errorf("measure.path gives %q, which is outside %q", dir, placePath)
	}
	if strings.Contains(t.place.Measure.Path, entry.VersionPlaceholder) && dir == placePath {
		return "", fmt.Errorf("measure.path gives the whole place %q for version %q", dir, version)
	}
	return dir, nil
}
