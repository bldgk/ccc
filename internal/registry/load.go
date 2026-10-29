// Package registry loads entries: the shipped ones and the user's.
package registry

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/bldgk/ccc/internal/entry"
	shipped "github.com/bldgk/ccc/registry"
)

const extension = ".toml"

// Problem is one source of entries that could not be used, and why.
type Problem struct {
	Source string
	Err    error
}

func (p Problem) Error() string { return p.Source + ": " + p.Err.Error() }

type Set struct {
	// Entries are sorted by name.
	Entries []entry.Entry
	// Problems are the user's files that were left out.
	Problems []Problem
}

// Load returns the shipped entries with the user's entries laid over them. The error is
// non-nil only when a shipped entry is invalid.
func Load() (Set, error) {
	base, err := Shipped()
	if err != nil {
		return Set{}, err
	}
	xdg, _ := os.LookupEnv("XDG_CONFIG_HOME")
	dir, err := UserDir(xdg, os.UserHomeDir)
	if err != nil {
		return Set{Entries: base, Problems: []Problem{{Source: "user registry", Err: err}}}, nil
	}
	user, problems := ReadDir(os.DirFS(dir))
	for i := range problems {
		problems[i].Source = filepath.Join(dir, problems[i].Source)
	}
	return Set{Entries: Overlay(base, user), Problems: problems}, nil
}

// Shipped returns the entries built into the program.
func Shipped() ([]entry.Entry, error) {
	entries, problems := ReadDir(shipped.Files)
	if len(problems) > 0 {
		errs := make([]error, len(problems))
		for i, p := range problems {
			errs[i] = p
		}
		return nil, fmt.Errorf("shipped registry: %w", errors.Join(errs...))
	}
	return entries, nil
}

// UserDir is the user's registry directory, given the value of XDG_CONFIG_HOME.
func UserDir(xdgConfigHome string, home func() (string, error)) (string, error) {
	if xdgConfigHome != "" {
		if !filepath.IsAbs(xdgConfigHome) {
			return "", fmt.Errorf("XDG_CONFIG_HOME %q is not an absolute path", xdgConfigHome)
		}
		return filepath.Join(xdgConfigHome, "ccc", "registry"), nil
	}
	h, err := home()
	if err != nil {
		return "", fmt.Errorf("home directory: %w", err)
	}
	return filepath.Join(h, ".config", "ccc", "registry"), nil
}

// ReadDir parses every .toml file at the top of fsys. A file that is not a valid entry
// becomes a problem and does not stop the others. A directory that does not exist holds
// no entries.
func ReadDir(fsys fs.FS) ([]entry.Entry, []Problem) {
	items, err := fs.ReadDir(fsys, ".")
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, []Problem{{Source: ".", Err: err}}
	}
	var (
		entries  []entry.Entry
		problems []Problem
	)
	for _, item := range items {
		name := item.Name()
		if item.IsDir() || !strings.HasSuffix(name, extension) {
			continue
		}
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			problems = append(problems, Problem{Source: name, Err: err})
			continue
		}
		e, err := Parse(name, data)
		if err != nil {
			problems = append(problems, Problem{Source: name, Err: err})
			continue
		}
		entries = append(entries, e)
	}
	sortByName(entries)
	return entries, problems
}

// Parse decodes and validates one entry file.
func Parse(fileName string, data []byte) (entry.Entry, error) {
	var e entry.Entry
	decoder := toml.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&e); err != nil {
		var strict *toml.StrictMissingError
		if errors.As(err, &strict) {
			return entry.Entry{}, fmt.Errorf("unknown key: %s", unknownKeys(strict))
		}
		return entry.Entry{}, err
	}

	var raw map[string]any
	if err := toml.Unmarshal(data, &raw); err != nil {
		return entry.Entry{}, err
	}
	if empty := emptyStrings("", raw); len(empty) > 0 {
		return entry.Entry{}, fmt.Errorf("empty value: %s", strings.Join(empty, ", "))
	}

	if err := entry.Validate(e); err != nil {
		return entry.Entry{}, err
	}
	if want := strings.TrimSuffix(fileName, extension); e.Name != want {
		return entry.Entry{}, fmt.Errorf("name %q does not equal the file name %q", e.Name, want)
	}
	return e, nil
}

// Overlay returns base with every entry of over added; an entry of over replaces the
// entry of base that has the same name as a whole.
func Overlay(base, over []entry.Entry) []entry.Entry {
	byName := map[string]entry.Entry{}
	for _, e := range base {
		byName[e.Name] = e
	}
	for _, e := range over {
		byName[e.Name] = e
	}
	merged := make([]entry.Entry, 0, len(byName))
	for _, e := range byName {
		merged = append(merged, e)
	}
	sortByName(merged)
	return merged
}

func sortByName(entries []entry.Entry) {
	slices.SortFunc(entries, func(a, b entry.Entry) int { return strings.Compare(a.Name, b.Name) })
}

func unknownKeys(strict *toml.StrictMissingError) string {
	keys := make([]string, 0, len(strict.Errors))
	for _, e := range strict.Errors {
		keys = append(keys, strings.Join(e.Key(), "."))
	}
	return strings.Join(keys, ", ")
}

// An empty string would be indistinguishable from an absent key after decoding, and
// an absent key has a meaning of its own: an absent clean.command removes a directory.
func emptyStrings(prefix string, value any) []string {
	var found []string
	switch v := value.(type) {
	case string:
		if v == "" {
			found = append(found, prefix)
		}
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, k := range keys {
			child := k
			if prefix != "" {
				child = prefix + "." + k
			}
			found = append(found, emptyStrings(child, v[k])...)
		}
	case []any:
		for i, item := range v {
			found = append(found, emptyStrings(fmt.Sprintf("%s[%d]", prefix, i), item)...)
		}
	}
	return found
}
