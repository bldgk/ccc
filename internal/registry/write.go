package registry

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/bldgk/ccc/internal/entry"
)

// Files is what writing an entry needs from the machine: the file-creation part of the
// sys contract.
type Files interface {
	CreateFile(path string, data []byte) error
}

// FilePath is the file of the entry of that name in a registry directory. The name has to
// be a single path element, because it becomes the name of the file.
func FilePath(dir, name string) (string, error) {
	switch {
	case name == "":
		return "", errors.New("the name is empty")
	case name == "." || name == "..":
		return "", fmt.Errorf("the name %q is not the name of a file", name)
	case strings.ContainsAny(name, "/\x00") || strings.ContainsRune(name, filepath.Separator):
		return "", fmt.Errorf("the name %q contains a path separator", name)
	}
	return filepath.Join(dir, name+extension), nil
}

// Write creates the file of the entry in dir and returns its path. It never replaces a
// file: an entry of that name that is there already is an error.
func Write(files Files, dir string, e entry.Entry) (string, error) {
	data, err := Encode(e)
	if err != nil {
		return "", err
	}
	path, err := FilePath(dir, e.Name)
	if err != nil {
		return "", err
	}
	if err := files.CreateFile(path, data); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return "", fmt.Errorf("an entry named %q exists already: %s", e.Name, path)
		}
		return "", err
	}
	return path, nil
}

// Encode returns the text of the entry's file. The entry has to be valid, and the text
// has to read back as the same entry; otherwise nothing is returned.
func Encode(e entry.Entry) ([]byte, error) {
	if err := entry.Validate(e); err != nil {
		return nil, err
	}
	var b strings.Builder
	pair(&b, "kind", string(e.Kind))
	pair(&b, "name", e.Name)
	pair(&b, "executable", e.Executable)
	pair(&b, "path", e.Path)
	for _, place := range e.Places {
		b.WriteString("\n[[places]]\n")
		pair(&b, "name", place.Name)
		pair(&b, "holds", string(place.Holds))
		if place.Requires != nil {
			pair(&b, "requires.command", place.Requires.Command)
		}
		if place.Path != nil {
			pair(&b, "path.command", place.Path.Command)
			pair(&b, "path.env", place.Path.Env)
			pair(&b, "path.subdir", place.Path.Subdir)
			for _, os := range defaultKeys(place.Path.Default) {
				pair(&b, "path.default."+key(os), place.Path.Default[os])
			}
		}
		if place.List != nil {
			pair(&b, "list.command", place.List.Command)
			pattern(&b, "list.pattern", place.List.Pattern)
		}
		pair(&b, "measure.by", string(place.Measure.By))
		pair(&b, "measure.path", place.Measure.Path)
		pair(&b, "measure.command", place.Measure.Command)
		pattern(&b, "measure.pattern", place.Measure.Pattern)
		pair(&b, "clean.before", place.Clean.Before)
		pair(&b, "clean.command", place.Clean.Command)
	}
	data := []byte(b.String())

	back, err := Parse(e.Name+extension, data)
	if err != nil {
		return nil, fmt.Errorf("the entry cannot be written as a file: %w", err)
	}
	if !reflect.DeepEqual(canonical(back), canonical(e)) {
		return nil, errors.New("the entry cannot be written as a file: the file would read back as a different entry")
	}
	return data, nil
}

// canonical gives the two spellings of "nothing" in an entry one form, so that an entry
// and what its file reads back as compare by content.
func canonical(e entry.Entry) entry.Entry {
	if len(e.Places) == 0 {
		e.Places = nil
		return e
	}
	e.Places = slices.Clone(e.Places)
	for i, place := range e.Places {
		if place.Path != nil && len(place.Path.Default) == 0 {
			path := *place.Path
			path.Default = nil
			e.Places[i].Path = &path
		}
	}
	return e
}

// defaultKeys orders the operating systems as the entry model lists them, and any other
// key after them by name.
func defaultKeys(defaults map[string]string) []string {
	var keys, others []string
	for _, os := range entry.OperatingSystems {
		if _, ok := defaults[os]; ok {
			keys = append(keys, os)
		}
	}
	for os := range defaults {
		if !slices.Contains(entry.OperatingSystems, os) {
			others = append(others, os)
		}
	}
	slices.Sort(others)
	return append(keys, others...)
}

// pair writes one key with a basic string; an absent value writes nothing, because an
// absent key and an empty string mean different things to the loader.
func pair(b *strings.Builder, name, value string) {
	if value != "" {
		b.WriteString(name + " = " + basic(value) + "\n")
	}
}

// pattern writes a regular expression as a literal string, which keeps its backslashes
// as they are, whenever TOML allows that text in a literal string.
func pattern(b *strings.Builder, name, value string) {
	if value == "" {
		return
	}
	if literal(value) {
		b.WriteString(name + " = '" + value + "'\n")
		return
	}
	pair(b, name, value)
}

func literal(value string) bool {
	if !utf8.ValidString(value) {
		return false
	}
	for _, r := range value {
		if r == '\'' || r == 0x7f || (r < 0x20 && r != '\t') {
			return false
		}
	}
	return true
}

func basic(value string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range value {
		switch {
		case r == '"':
			b.WriteString(`\"`)
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\r':
			b.WriteString(`\r`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\u%04X`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func key(name string) string {
	bare := name != ""
	for _, r := range name {
		if !(r == '_' || r == '-' || (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')) {
			bare = false
		}
	}
	if bare {
		return name
	}
	return basic(name)
}
