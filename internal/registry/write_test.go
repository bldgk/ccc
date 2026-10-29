package registry

import (
	"reflect"
	"strings"
	"testing"

	"github.com/bldgk/ccc/internal/entry"
	"github.com/bldgk/ccc/internal/sys"
	"github.com/bldgk/ccc/internal/sys/fake"
)

const userDir = "/home/alice/.config/ccc/registry"

func place(change func(*entry.Place)) entry.Entry {
	p := entry.Place{
		Name:    "cache",
		Holds:   entry.Cache,
		Measure: entry.Measure{By: entry.DiskUsage},
	}
	change(&p)
	return entry.Entry{Kind: entry.Tool, Name: "tool", Executable: "tool", Places: []entry.Place{p}}
}

func TestWhatIsWrittenLoadsBackAsTheSameEntry(t *testing.T) {
	entries := map[string]entry.Entry{
		"a folder": {Kind: entry.Folder, Name: "scratch", Path: "~/work/scratch"},
		"a cache found by a command": place(func(p *entry.Place) {
			p.Path = &entry.Path{Command: "tool cache dir", Subdir: "objects"}
			p.Clean = entry.Clean{Before: "tool stop", Command: "tool cache clean"}
		}),
		"a cache found by a variable": place(func(p *entry.Place) {
			p.Path = &entry.Path{Env: "TOOL_CACHE"}
		}),
		"a cache found by a default": place(func(p *entry.Place) {
			p.Path = &entry.Path{Default: map[string]string{sys.Linux: "~/.cache/tool", sys.MacOS: "$HOME/Library/Caches/tool"}}
			p.Requires = &entry.Requires{Command: "tool info"}
		}),
		"installed versions": place(func(p *entry.Place) {
			p.Name = "versions"
			p.Holds = entry.Versions
			p.Path = &entry.Path{Command: "tool home", Env: "TOOL_HOME", Default: map[string]string{sys.Linux: "~/.tool"}}
			p.List = &entry.List{Command: "tool list", Pattern: `^(\S+)\s`}
			p.Measure = entry.Measure{By: entry.DiskUsage, Path: "<path>/<version>"}
			p.Clean = entry.Clean{Command: "tool uninstall <version>"}
		}),
		"a place measured by a command, without a path": place(func(p *entry.Place) {
			p.Measure = entry.Measure{By: entry.Command, Command: "tool df --format '{{.Size}}'", Pattern: `^Total (\S+)`}
			p.Clean = entry.Clean{Command: "tool prune --force"}
		}),
		"text that needs escaping": place(func(p *entry.Place) {
			p.Name = `the "big" one`
			p.Path = &entry.Path{Command: `tool path --sep "\" 'quoted arg'`}
			p.Measure = entry.Measure{By: entry.Command, Command: "tool size", Pattern: `^it's (\d+)`}
			p.Clean = entry.Clean{Command: "tool clean"}
		}),
	}
	for name, e := range entries {
		data, err := Encode(e)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		back, err := Parse(e.Name+".toml", data)
		if err != nil {
			t.Fatalf("%s: the text does not load: %v\n%s", name, err, data)
		}
		if !reflect.DeepEqual(back, e) {
			t.Fatalf("%s: loaded back\n%#v\nwant\n%#v\ntext:\n%s", name, back, e, data)
		}
	}
}

func TestTheFileHasTheShapeOfAShippedEntry(t *testing.T) {
	e := place(func(p *entry.Place) {
		p.Name = "versions"
		p.Holds = entry.Versions
		p.Path = &entry.Path{Command: "tool home", Default: map[string]string{sys.Linux: "~/.tool", sys.MacOS: "~/Library/tool"}}
		p.List = &entry.List{Command: "tool list", Pattern: `^(\S+)$`}
		p.Measure = entry.Measure{By: entry.DiskUsage, Path: "<path>/<version>"}
		p.Clean = entry.Clean{Command: "tool uninstall <version>"}
	})
	want := `kind = "tool"
name = "tool"
executable = "tool"

[[places]]
name = "versions"
holds = "versions"
path.command = "tool home"
path.default.macos = "~/Library/tool"
path.default.linux = "~/.tool"
list.command = "tool list"
list.pattern = '^(\S+)$'
measure.by = "disk-usage"
measure.path = "<path>/<version>"
clean.command = "tool uninstall <version>"
`
	data, err := Encode(e)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want {
		t.Fatalf("the text is\n%s\nwant\n%s", data, want)
	}
}

func TestAnInvalidEntryIsNotWritten(t *testing.T) {
	f := fake.New(sys.Linux, "/home/alice")
	e := entry.Entry{Kind: entry.Folder, Name: "scratch"}
	if _, err := Write(f, userDir, e); err == nil || !strings.Contains(err.Error(), "path is required") {
		t.Fatalf("the error is %v, want the rule the entry breaks", err)
	}
	if created := f.Created(); len(created) != 0 {
		t.Fatalf("files were written: %v", created)
	}
}

func TestWriteCreatesTheFileOfTheEntry(t *testing.T) {
	f := fake.New(sys.Linux, "/home/alice")
	e := entry.Entry{Kind: entry.Folder, Name: "scratch", Path: "/work/scratch"}
	path, err := Write(f, userDir, e)
	if err != nil {
		t.Fatal(err)
	}
	if want := userDir + "/scratch.toml"; path != want {
		t.Fatalf("the file is %q, want %q", path, want)
	}
	data, _ := f.FileContent(path)
	back, err := Parse("scratch.toml", data)
	if err != nil || !reflect.DeepEqual(back, e) {
		t.Fatalf("the file holds %q, which loads as %#v, %v", data, back, err)
	}
}

func TestWriteNeverReplacesAFile(t *testing.T) {
	f := fake.New(sys.Linux, "/home/alice")
	first := entry.Entry{Kind: entry.Folder, Name: "scratch", Path: "/work/first"}
	path, err := Write(f, userDir, first)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := f.FileContent(path)

	second := entry.Entry{Kind: entry.Folder, Name: "scratch", Path: "/work/second"}
	if _, err := Write(f, userDir, second); err == nil || !strings.Contains(err.Error(), "exists already") {
		t.Fatalf("the error is %v, want a refusal that names the existing entry", err)
	}
	if after, _ := f.FileContent(path); string(after) != string(before) {
		t.Fatalf("the file changed from\n%s\nto\n%s", before, after)
	}
}

func TestANameThatIsNotAFileNameIsRefused(t *testing.T) {
	for _, name := range []string{"", ".", "..", "a/b", "../escape"} {
		if path, err := FilePath(userDir, name); err == nil {
			t.Fatalf("the name %q gives the file %q, want a refusal", name, path)
		}
	}
	f := fake.New(sys.Linux, "/home/alice")
	e := entry.Entry{Kind: entry.Folder, Name: "../escape", Path: "/work/scratch"}
	if _, err := Write(f, userDir, e); err == nil {
		t.Fatal("an entry whose name leaves the registry directory was written")
	}
	if created := f.Created(); len(created) != 0 {
		t.Fatalf("files were written: %v", created)
	}
}
