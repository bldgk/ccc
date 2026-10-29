package entry

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

var envName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type problems struct {
	prefix string
	list   []error
}

func (p *problems) add(format string, args ...any) {
	p.list = append(p.list, errors.New(p.prefix+fmt.Sprintf(format, args...)))
}

// Validate reports every rule the entry breaks, or nil when it breaks none.
func Validate(e Entry) error {
	p := &problems{}
	if e.Name == "" {
		p.add("name is required")
	}
	switch e.Kind {
	case Tool:
		validateTool(e, p)
	case Folder:
		validateFolder(e, p)
	case "":
		p.add("kind is required")
	default:
		p.add("kind %q is neither %q nor %q", e.Kind, Tool, Folder)
	}
	return errors.Join(p.list...)
}

func validateFolder(e Entry, p *problems) {
	if e.Path == "" {
		p.add("path is required for a folder")
	} else {
		checkExpandable(p, "path", e.Path)
	}
	if e.Executable != "" {
		p.add("executable is legal only for a tool")
	}
	if len(e.Places) > 0 {
		p.add("places is legal only for a tool")
	}
}

func validateTool(e Entry, p *problems) {
	if e.Path != "" {
		p.add("path is legal only for a folder")
	}
	if len(e.Places) == 0 {
		p.add("a tool needs at least one place")
	}
	seen := map[string]bool{}
	for i, place := range e.Places {
		if place.Name == "" {
			p.prefix = fmt.Sprintf("place %d: ", i+1)
			p.add("name is required")
		} else {
			p.prefix = fmt.Sprintf("place %q: ", place.Name)
			if seen[place.Name] {
				p.add("the name is used by another place of this entry")
			}
			seen[place.Name] = true
		}
		validatePlace(e, place, p)
	}
	p.prefix = ""
}

func validatePlace(e Entry, place Place, p *problems) {
	switch place.Holds {
	case Cache, Versions:
	case "":
		p.add("holds is required")
	default:
		p.add("holds %q is neither %q nor %q", place.Holds, Cache, Versions)
	}

	if place.Requires != nil {
		if place.Requires.Command == "" {
			p.add("requires.command is required when requires is given")
		} else {
			checkCommand(p, "requires.command", place.Requires.Command)
			checkNoPlaceholders(p, "requires.command", place.Requires.Command)
		}
	}

	if place.Path != nil {
		validatePath(e, *place.Path, p)
	} else {
		validatePathless(e, place, p)
	}

	validateList(place, p)
	validateMeasure(place, p)

	if place.Clean.Before != "" {
		checkCommand(p, "clean.before", place.Clean.Before)
	}
	if place.Clean.Command != "" {
		checkCommand(p, "clean.command", place.Clean.Command)
		// Without a declared executable the command could never be chosen over removing
		// the directory, so the entry would silently do something else than it says.
		if e.Executable == "" {
			p.add("clean.command needs the entry's executable")
		}
	}

	// Without a declared executable the tool never counts as installed, and a command
	// of an entry runs only for an installed tool.
	if e.Executable == "" {
		if place.Requires != nil {
			p.add("requires.command needs the entry's executable")
		}
		if place.List != nil && place.List.Command != "" {
			p.add("list.command needs the entry's executable")
		}
		if place.Measure.Command != "" {
			p.add("measure.command needs the entry's executable")
		}
		if place.Clean.Before != "" {
			p.add("clean.before needs the entry's executable")
		}
	}

	substituted := map[string]string{
		"measure.path":    place.Measure.Path,
		"measure.command": place.Measure.Command,
		"clean.before":    place.Clean.Before,
		"clean.command":   place.Clean.Command,
	}
	for _, key := range sortedKeys(substituted) {
		value := substituted[key]
		if place.Holds != Versions && strings.Contains(value, VersionPlaceholder) {
			p.add("%s: %s is legal only in a place that holds versions", key, VersionPlaceholder)
		}
		if place.Path == nil && strings.Contains(value, PathPlaceholder) {
			p.add("%s: %s needs a path table", key, PathPlaceholder)
		}
	}
}

func validatePath(e Entry, path Path, p *problems) {
	if path.Command == "" && path.Env == "" && len(path.Default) == 0 {
		p.add("path gives no source: one of path.command, path.env, path.default is required")
	}
	if path.Command != "" {
		checkCommand(p, "path.command", path.Command)
		checkNoPlaceholders(p, "path.command", path.Command)
		if e.Executable == "" {
			p.add("path.command needs the entry's executable")
		}
	}
	if path.Env != "" && !envName.MatchString(path.Env) {
		p.add("path.env %q is not the name of an environment variable", path.Env)
	}
	if path.Subdir != "" {
		if path.Command == "" && path.Env == "" {
			p.add("path.subdir needs path.command or path.env")
		}
		if strings.HasPrefix(path.Subdir, "/") {
			p.add("path.subdir %q must be relative", path.Subdir)
		}
		if slices.Contains(strings.Split(path.Subdir, "/"), "..") {
			p.add("path.subdir %q must not contain ..", path.Subdir)
		}
		checkNoPlaceholders(p, "path.subdir", path.Subdir)
	}
	for _, os := range sortedKeys(path.Default) {
		key := "path.default." + os
		if !slices.Contains(OperatingSystems, os) {
			p.add("%s: unknown operating system, expected one of %s", key, strings.Join(OperatingSystems, ", "))
			continue
		}
		checkExpandable(p, key, path.Default[os])
	}
}

func validatePathless(e Entry, place Place, p *problems) {
	const what = "a place without a path table"
	if place.Measure.By != Command {
		p.add("%s must have measure.by = %q", what, Command)
	}
	if place.Clean.Command == "" {
		p.add("%s must have clean.command", what)
	}
	if e.Executable == "" {
		p.add("%s needs the entry's executable", what)
	}
	if place.Holds == Versions {
		p.add("a place that holds versions needs a path table")
	}
}

func validateList(place Place, p *problems) {
	if place.Holds == Versions {
		if place.List == nil || place.List.Command == "" {
			p.add("list.command is required in a place that holds versions")
		}
		if place.List == nil || place.List.Pattern == "" {
			p.add("list.pattern is required in a place that holds versions")
		}
	} else if place.List != nil {
		p.add("list is legal only in a place that holds versions")
	}
	if place.List == nil {
		return
	}
	if place.List.Command != "" {
		checkCommand(p, "list.command", place.List.Command)
		checkNoPlaceholders(p, "list.command", place.List.Command)
	}
	if place.List.Pattern != "" {
		checkPattern(p, "list.pattern", place.List.Pattern)
	}
}

func validateMeasure(place Place, p *problems) {
	m := place.Measure
	switch m.By {
	case DiskUsage:
		if m.Command != "" {
			p.add("measure.command is legal only with measure.by = %q", Command)
		}
		if m.Pattern != "" {
			p.add("measure.pattern is legal only with measure.by = %q", Command)
		}
	case Command:
		if m.Command == "" {
			p.add("measure.command is required with measure.by = %q", Command)
		} else {
			checkCommand(p, "measure.command", m.Command)
		}
		if m.Pattern == "" {
			p.add("measure.pattern is required with measure.by = %q", Command)
		} else {
			checkPattern(p, "measure.pattern", m.Pattern)
		}
	case "":
		p.add("measure.by is required")
	default:
		p.add("measure.by %q is neither %q nor %q", m.By, DiskUsage, Command)
	}

	if m.Path != "" && place.Path != nil {
		// The measured directory is also the directory that removal deletes, so it has
		// to stay inside what the place resolved to.
		if m.Path != PathPlaceholder && !strings.HasPrefix(m.Path, PathPlaceholder+"/") {
			p.add("measure.path must start with %s", PathPlaceholder)
		}
		if slices.Contains(strings.Split(m.Path, "/"), "..") {
			p.add("measure.path must not contain ..")
		}
	}
	if m.Path != "" && place.Path == nil {
		p.add("measure.path needs a path table")
	}
	// Every version row of a place would otherwise share one directory, and removing
	// one version would remove them all.
	if place.Holds == Versions && place.Path != nil && !strings.Contains(m.Path, VersionPlaceholder) {
		p.add("measure.path must contain %s in a place that holds versions", VersionPlaceholder)
	}
}

func checkCommand(p *problems, key, command string) {
	if _, err := SplitCommand(command); err != nil {
		p.add("%s: %v", key, err)
	}
}

func checkPattern(p *problems, key, pattern string) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		p.add("%s: %v", key, err)
		return
	}
	if n := re.NumSubexp(); n != 1 {
		p.add("%s must have exactly one group, it has %d", key, n)
	}
}

func checkNoPlaceholders(p *problems, key, value string) {
	for _, placeholder := range []string{PathPlaceholder, VersionPlaceholder} {
		if strings.Contains(value, placeholder) {
			p.add("%s: %s is not replaced in this key", key, placeholder)
		}
	}
}

func checkExpandable(p *problems, key, value string) {
	if value == "" {
		p.add("%s is empty", key)
		return
	}
	if !strings.HasPrefix(value, "/") && value != "~" && !strings.HasPrefix(value, "~/") && !strings.HasPrefix(value, "$") {
		p.add("%s %q must start with /, ~/ or an environment variable", key, value)
	}
	checkNoPlaceholders(p, key, value)
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
