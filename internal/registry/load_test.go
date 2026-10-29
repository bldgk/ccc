package registry

import (
	"errors"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/bldgk/ccc/internal/entry"
)

const validTool = `
kind = "tool"
name = "tool"
executable = "tool"

[[places]]
name = "cache"
holds = "cache"
requires.command = "tool info"
path.command = "tool cache dir"
path.env = "TOOL_CACHE"
path.subdir = "objects"
path.default.macos = "~/Library/Caches/tool/objects"
path.default.linux = "~/.cache/tool/objects"
measure.by = "disk-usage"
clean.before = "tool stop"
clean.command = "tool cache clean"

[[places]]
name = "versions"
holds = "versions"
path.command = "tool home"
list.command = "tool list"
list.pattern = '^(\S+)$'
measure.by = "disk-usage"
measure.path = "<path>/<version>"
clean.command = "tool uninstall <version>"

[[places]]
name = "images"
holds = "cache"
measure.by = "command"
measure.command = "tool df --format '{{.Type}} {{.Size}}'"
measure.pattern = '^Images (\S+)$'
clean.command = "tool prune --force"
`

func TestShippedEntriesAreValid(t *testing.T) {
	if _, err := Shipped(); err != nil {
		t.Fatal(err)
	}
}

func TestParseReadsEveryKey(t *testing.T) {
	e, err := Parse("tool.toml", []byte(validTool))
	if err != nil {
		t.Fatal(err)
	}
	if e.Kind != entry.Tool || e.Name != "tool" || e.Executable != "tool" || len(e.Places) != 3 {
		t.Fatalf("top level decoded as %+v", e)
	}
	cache := e.Places[0]
	if cache.Requires == nil || cache.Requires.Command != "tool info" {
		t.Errorf("requires decoded as %+v", cache.Requires)
	}
	wantPath := entry.Path{
		Command: "tool cache dir", Env: "TOOL_CACHE", Subdir: "objects",
		Default: map[string]string{"macos": "~/Library/Caches/tool/objects", "linux": "~/.cache/tool/objects"},
	}
	if cache.Path == nil || cache.Path.Command != wantPath.Command || cache.Path.Env != wantPath.Env ||
		cache.Path.Subdir != wantPath.Subdir || cache.Path.Default["macos"] != wantPath.Default["macos"] ||
		cache.Path.Default["linux"] != wantPath.Default["linux"] {
		t.Errorf("path decoded as %+v", cache.Path)
	}
	if cache.Clean != (entry.Clean{Before: "tool stop", Command: "tool cache clean"}) {
		t.Errorf("clean decoded as %+v", cache.Clean)
	}
	versions := e.Places[1]
	if versions.Holds != entry.Versions || versions.List == nil ||
		*versions.List != (entry.List{Command: "tool list", Pattern: `^(\S+)$`}) ||
		versions.Measure.Path != "<path>/<version>" {
		t.Errorf("versions place decoded as %+v", versions)
	}
	images := e.Places[2]
	if images.Path != nil {
		t.Errorf("a place without a path table decoded with one: %+v", images.Path)
	}
	if images.Measure != (entry.Measure{By: entry.Command, Command: "tool df --format '{{.Type}} {{.Size}}'", Pattern: `^Images (\S+)$`}) {
		t.Errorf("measure decoded as %+v", images.Measure)
	}
}

func TestParseRejectsUnknownKeys(t *testing.T) {
	cases := map[string]struct {
		text string
		key  string
	}{
		"top level": {
			"kind = \"folder\"\nname = \"f\"\npath = \"~/f\"\nowner = \"alice\"\n",
			"owner",
		},
		"in a place": {
			strings.Replace(validTool, `holds = "versions"`, "holds = \"versions\"\npriority = 3", 1),
			"places.priority",
		},
		"in a table of a place": {
			strings.Replace(validTool, `measure.by = "command"`, "measure.by = \"command\"\nmeasure.requires = \"tool info\"", 1),
			"places.measure.requires",
		},
		"a table that does not exist": {
			strings.Replace(validTool, `clean.before = "tool stop"`, `clean.after = "tool start"`, 1),
			"places.clean.after",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse("tool.toml", []byte(c.text))
			if name == "top level" {
				_, err = Parse("f.toml", []byte(c.text))
			}
			if err == nil {
				t.Fatal("accepted")
			}
			if !strings.Contains(err.Error(), "unknown key") || !strings.Contains(err.Error(), c.key) {
				t.Fatalf("got %q, want an unknown-key error naming %q", err, c.key)
			}
		})
	}
}

func TestParseRejects(t *testing.T) {
	cases := map[string]struct {
		file   string
		text   string
		reason string
	}{
		"name that differs from the file name": {"other.toml", validTool, `name "tool" does not equal the file name "other"`},
		"empty string value": {
			"tool.toml",
			strings.Replace(validTool, `clean.command = "tool cache clean"`, `clean.command = ""`, 1),
			"empty value: places[0].clean.command",
		},
		"wrong type": {
			"tool.toml",
			strings.Replace(validTool, `path.env = "TOOL_CACHE"`, `path.env = 7`, 1),
			"",
		},
		"not TOML":                {"tool.toml", "kind = \n", ""},
		"an entry rule is broken": {"f.toml", "kind = \"folder\"\nname = \"f\"\n", "path is required for a folder"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse(c.file, []byte(c.text))
			if err == nil {
				t.Fatal("accepted")
			}
			if !strings.Contains(err.Error(), c.reason) {
				t.Fatalf("got %q, want an error containing %q", err, c.reason)
			}
		})
	}
}

func TestReadDirKeepsGoingPastAnInvalidFile(t *testing.T) {
	fsys := fstest.MapFS{
		"tool.toml":       {Data: []byte(validTool)},
		"broken.toml":     {Data: []byte("kind = \"folder\"\nname = \"broken\"\ncolour = \"red\"\n")},
		"build.toml":      {Data: []byte("kind = \"folder\"\nname = \"build\"\npath = \"~/work/build\"\n")},
		"notes.txt":       {Data: []byte("not an entry")},
		"nested/sub.toml": {Data: []byte("kind = \"folder\"\nname = \"sub\"\npath = \"~/sub\"\n")},
	}
	entries, problems := ReadDir(fsys)
	if len(entries) != 2 || entries[0].Name != "build" || entries[1].Name != "tool" {
		t.Fatalf("entries: %+v", entries)
	}
	if len(problems) != 1 || problems[0].Source != "broken.toml" || !strings.Contains(problems[0].Err.Error(), "colour") {
		t.Fatalf("problems: %+v", problems)
	}
}

func TestReadDirOfAMissingDirectoryHoldsNothing(t *testing.T) {
	sub, err := fstest.MapFS{}.Sub("absent")
	if err != nil {
		t.Fatal(err)
	}
	entries, problems := ReadDir(sub)
	if len(entries) != 0 || len(problems) != 0 {
		t.Fatalf("entries %+v, problems %+v", entries, problems)
	}
}

func TestOverlayReplacesTheShippedEntryAsAWhole(t *testing.T) {
	shipped, err := Parse("tool.toml", []byte(validTool))
	if err != nil {
		t.Fatal(err)
	}
	other := entry.Entry{Kind: entry.Folder, Name: "other", Path: "~/other"}
	user := entry.Entry{Kind: entry.Tool, Name: "tool", Executable: "tool2", Places: []entry.Place{{
		Name:    "only",
		Holds:   entry.Cache,
		Path:    &entry.Path{Env: "ONLY"},
		Measure: entry.Measure{By: entry.DiskUsage},
	}}}
	added := entry.Entry{Kind: entry.Folder, Name: "added", Path: "~/added"}

	merged := Overlay([]entry.Entry{shipped, other}, []entry.Entry{user, added})

	names := make([]string, len(merged))
	for i, e := range merged {
		names[i] = e.Name
	}
	if strings.Join(names, ",") != "added,other,tool" {
		t.Fatalf("names: %v", names)
	}
	got := merged[2]
	if got.Executable != "tool2" || len(got.Places) != 1 || got.Places[0].Name != "only" {
		t.Fatalf("the user's entry did not replace the shipped one as a whole: %+v", got)
	}
}

func TestUserDir(t *testing.T) {
	home := func() (string, error) { return "/home/alice", nil }
	noHome := func() (string, error) { return "", errors.New("no home") }

	if dir, err := UserDir("/home/alice/cfg", home); err != nil || dir != "/home/alice/cfg/ccc/registry" {
		t.Errorf("with XDG_CONFIG_HOME: %q, %v", dir, err)
	}
	if dir, err := UserDir("", home); err != nil || dir != "/home/alice/.config/ccc/registry" {
		t.Errorf("without XDG_CONFIG_HOME: %q, %v", dir, err)
	}
	if dir, err := UserDir("/home/alice/cfg", noHome); err != nil || dir != "/home/alice/cfg/ccc/registry" {
		t.Errorf("XDG_CONFIG_HOME must not need the home directory: %q, %v", dir, err)
	}
	if _, err := UserDir("cfg", home); err == nil {
		t.Error("a relative XDG_CONFIG_HOME was accepted")
	}
	if _, err := UserDir("", noHome); err == nil {
		t.Error("an unknown home directory was accepted")
	}
}

func TestLoadReadsTheUsersDirectory(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	dir, err := UserDir(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "mine.toml", "kind = \"folder\"\nname = \"mine\"\npath = \"~/mine\"\n")
	writeFile(t, dir, "bad.toml", "kind = \"folder\"\nname = \"bad\"\n")

	set, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range set.Entries {
		if e.Name == "mine" && e.Path == "~/mine" {
			found = true
		}
		if e.Name == "bad" {
			t.Error("the invalid file was loaded")
		}
	}
	if !found {
		t.Errorf("the user's entry is missing from %d entries", len(set.Entries))
	}
	shippedEntries, err := Shipped()
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Entries) != len(shippedEntries)+1 {
		t.Errorf("%d entries, want the %d shipped ones and one of the user", len(set.Entries), len(shippedEntries))
	}
	if len(set.Problems) != 1 || !strings.HasSuffix(set.Problems[0].Source, "bad.toml") ||
		!strings.Contains(set.Problems[0].Err.Error(), "path is required") {
		t.Errorf("problems: %+v", set.Problems)
	}
}

func TestAnInvalidUserFileLeavesTheShippedEntryInPlace(t *testing.T) {
	shipped, problems := ReadDir(fstest.MapFS{"tool.toml": {Data: []byte(validTool)}})
	if len(problems) != 0 {
		t.Fatal(problems)
	}
	user, problems := ReadDir(fstest.MapFS{
		"tool.toml": {Data: []byte("kind = \"tool\"\nname = \"tool\"\n")},
	})
	if len(user) != 0 || len(problems) != 1 || problems[0].Source != "tool.toml" {
		t.Fatalf("user entries %+v, problems %+v", user, problems)
	}

	merged := Overlay(shipped, user)
	if len(merged) != 1 || merged[0].Executable != "tool" || len(merged[0].Places) != 3 {
		t.Fatalf("the shipped entry did not stay: %+v", merged)
	}
}
