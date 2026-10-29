package entry

import (
	"slices"
	"strings"
	"testing"
)

func cachePlace() Place {
	return Place{
		Name:  "cache",
		Holds: Cache,
		Path: &Path{
			Command: "tool cache dir",
			Env:     "TOOL_CACHE",
			Default: map[string]string{"macos": "~/Library/Caches/tool", "linux": "$XDG_CACHE_HOME/tool"},
		},
		Measure: Measure{By: DiskUsage},
		Clean:   Clean{Command: "tool cache clean"},
	}
}

func versionsPlace() Place {
	return Place{
		Name:    "versions",
		Holds:   Versions,
		Path:    &Path{Command: "tool home", Subdir: "versions"},
		List:    &List{Command: "tool list", Pattern: `^(\S+)$`},
		Measure: Measure{By: DiskUsage, Path: "<path>/<version>"},
		Clean:   Clean{Command: "tool uninstall <version>"},
	}
}

func pathlessPlace() Place {
	return Place{
		Name:     "images",
		Holds:    Cache,
		Requires: &Requires{Command: "tool info"},
		Measure:  Measure{By: Command, Command: "tool df", Pattern: `^Images (\S+)$`},
		Clean:    Clean{Before: "tool stop", Command: "tool prune --force"},
	}
}

func withoutExecutable() Entry {
	return Entry{Kind: Tool, Name: "plain", Places: []Place{{
		Name:    "cache",
		Holds:   Cache,
		Path:    &Path{Env: "PLAIN_CACHE"},
		Measure: Measure{By: DiskUsage},
	}}}
}

func tool(places ...Place) Entry {
	return Entry{Kind: Tool, Name: "tool", Executable: "tool", Places: places}
}

func TestValidEntries(t *testing.T) {
	entries := map[string]Entry{
		"tool with every kind of place": tool(cachePlace(), versionsPlace(), pathlessPlace()),
		"folder":                        {Kind: Folder, Name: "build", Path: "~/work/build"},
		"folder under a variable":       {Kind: Folder, Name: "build", Path: "${WORK}/build"},
		"tool without an executable, removed as a directory": {Kind: Tool, Name: "plain", Places: []Place{{
			Name:    "cache",
			Holds:   Cache,
			Path:    &Path{Env: "PLAIN_CACHE", Subdir: "data", Default: map[string]string{"linux": "/var/cache/plain"}},
			Measure: Measure{By: DiskUsage},
		}}},
		"directory place measured by a command": tool(Place{
			Name:    "cache",
			Holds:   Cache,
			Path:    &Path{Command: "tool cache dir"},
			Measure: Measure{By: Command, Command: "tool size <path>", Pattern: `(\d+)`, Path: "<path>/objects"},
		}),
	}
	for name, e := range entries {
		t.Run(name, func(t *testing.T) {
			if err := Validate(e); err != nil {
				t.Fatalf("valid entry rejected: %v", err)
			}
		})
	}
}

func TestValidationRules(t *testing.T) {
	cases := []struct {
		name   string
		entry  func() Entry
		reason string
	}{
		{"kind missing", func() Entry { e := tool(cachePlace()); e.Kind = ""; return e }, "kind is required"},
		{"kind unknown", func() Entry { e := tool(cachePlace()); e.Kind = "service"; return e }, `kind "service"`},
		{"name missing", func() Entry { e := tool(cachePlace()); e.Name = ""; return e }, "name is required"},
		{"tool with a path", func() Entry { e := tool(cachePlace()); e.Path = "~/x"; return e }, "path is legal only for a folder"},
		{"tool without places", func() Entry { return tool() }, "at least one place"},
		{"folder without a path", func() Entry { return Entry{Kind: Folder, Name: "f"} }, "path is required for a folder"},
		{"folder with places", func() Entry {
			return Entry{Kind: Folder, Name: "f", Path: "~/x", Places: []Place{cachePlace()}}
		}, "places is legal only for a tool"},
		{"folder with an executable", func() Entry {
			return Entry{Kind: Folder, Name: "f", Path: "~/x", Executable: "tool"}
		}, "executable is legal only for a tool"},
		{"folder with a relative path", func() Entry {
			return Entry{Kind: Folder, Name: "f", Path: "work/build"}
		}, "must start with"},
		{"folder path with a placeholder", func() Entry {
			return Entry{Kind: Folder, Name: "f", Path: "~/<path>"}
		}, "<path> is not replaced"},

		{"place without a name", func() Entry { p := cachePlace(); p.Name = ""; return tool(p) }, "place 1: name is required"},
		{"two places with one name", func() Entry { return tool(cachePlace(), cachePlace()) }, "used by another place"},
		{"holds missing", func() Entry { p := cachePlace(); p.Holds = ""; return tool(p) }, "holds is required"},
		{"holds unknown", func() Entry { p := cachePlace(); p.Holds = "logs"; return tool(p) }, `holds "logs"`},

		{"requires without a command", func() Entry { p := cachePlace(); p.Requires = &Requires{}; return tool(p) }, "requires.command is required"},
		{"requires with a placeholder", func() Entry {
			p := cachePlace()
			p.Requires = &Requires{Command: "tool check <path>"}
			return tool(p)
		}, "requires.command: <path> is not replaced"},

		{"path table without a source", func() Entry { p := cachePlace(); p.Path = &Path{}; return tool(p) }, "path gives no source"},
		{"path.command without an executable", func() Entry {
			e := tool(cachePlace())
			e.Executable = ""
			e.Places[0].Clean = Clean{}
			return e
		}, "path.command needs the entry's executable"},
		{"path.command with an open quote", func() Entry {
			p := cachePlace()
			p.Path.Command = "tool 'cache dir"
			return tool(p)
		}, "path.command: a quote is not closed"},
		{"path.env that is not a name", func() Entry { p := cachePlace(); p.Path.Env = "$TOOL CACHE"; return tool(p) }, "path.env"},
		{"subdir without command or env", func() Entry {
			p := cachePlace()
			p.Path = &Path{Subdir: "x", Default: map[string]string{"linux": "/var/cache/tool"}}
			return tool(p)
		}, "path.subdir needs path.command or path.env"},
		{"absolute subdir", func() Entry { p := cachePlace(); p.Path.Subdir = "/etc"; return tool(p) }, "must be relative"},
		{"subdir that climbs", func() Entry { p := cachePlace(); p.Path.Subdir = "a/../.."; return tool(p) }, "must not contain .."},
		{"default for an unknown system", func() Entry {
			p := cachePlace()
			p.Path.Default["plan9"] = "/cache"
			return tool(p)
		}, "path.default.plan9: unknown operating system"},
		{"relative default", func() Entry {
			p := cachePlace()
			p.Path.Default["linux"] = "cache/tool"
			return tool(p)
		}, "path.default.linux"},

		{"pathless place measured by disk usage", func() Entry {
			p := pathlessPlace()
			p.Measure = Measure{By: DiskUsage}
			return tool(p)
		}, `must have measure.by = "command"`},
		{"pathless place without clean.command", func() Entry {
			p := pathlessPlace()
			p.Clean.Command = ""
			return tool(p)
		}, "must have clean.command"},
		{"pathless place without an executable", func() Entry {
			e := tool(pathlessPlace())
			e.Executable = ""
			return e
		}, "a place without a path table needs the entry's executable"},
		{"pathless place that uses <path>", func() Entry {
			p := pathlessPlace()
			p.Clean.Command = "tool prune <path>"
			return tool(p)
		}, "clean.command: <path> needs a path table"},
		{"pathless place with measure.path", func() Entry {
			p := pathlessPlace()
			p.Measure.Path = "/var/lib/tool"
			return tool(p)
		}, "measure.path needs a path table"},
		{"pathless place that holds versions", func() Entry {
			p := pathlessPlace()
			p.Holds = Versions
			p.List = &List{Command: "tool list", Pattern: `(\S+)`}
			return tool(p)
		}, "a place that holds versions needs a path table"},

		{"list in a cache place", func() Entry {
			p := cachePlace()
			p.List = &List{Command: "tool list", Pattern: `(\S+)`}
			return tool(p)
		}, "list is legal only in a place that holds versions"},
		{"versions without list.command", func() Entry {
			p := versionsPlace()
			p.List.Command = ""
			return tool(p)
		}, "list.command is required"},
		{"versions without list.pattern", func() Entry {
			p := versionsPlace()
			p.List.Pattern = ""
			return tool(p)
		}, "list.pattern is required"},
		{"versions without a list table", func() Entry { p := versionsPlace(); p.List = nil; return tool(p) }, "list.command is required"},
		{"list.pattern that does not compile", func() Entry {
			p := versionsPlace()
			p.List.Pattern = `^(\S+$`
			return tool(p)
		}, "list.pattern:"},
		{"list.pattern without a group", func() Entry {
			p := versionsPlace()
			p.List.Pattern = `^\S+$`
			return tool(p)
		}, "list.pattern must have exactly one group, it has 0"},
		{"list.pattern with two groups", func() Entry {
			p := versionsPlace()
			p.List.Pattern = `^(\S+) (\S+)$`
			return tool(p)
		}, "list.pattern must have exactly one group, it has 2"},
		{"versions rows that share one directory", func() Entry {
			p := versionsPlace()
			p.Measure.Path = ""
			return tool(p)
		}, "measure.path must contain <version>"},
		{"<version> in a cache place", func() Entry {
			p := cachePlace()
			p.Clean.Command = "tool uninstall <version>"
			return tool(p)
		}, "clean.command: <version> is legal only in a place that holds versions"},
		{"<version> in measure.path of a cache place", func() Entry {
			p := cachePlace()
			p.Measure.Path = "<path>/<version>"
			return tool(p)
		}, "measure.path: <version> is legal only"},

		{"measure.by missing", func() Entry { p := cachePlace(); p.Measure.By = ""; return tool(p) }, "measure.by is required"},
		{"measure.by unknown", func() Entry { p := cachePlace(); p.Measure.By = "guess"; return tool(p) }, `measure.by "guess"`},
		{"command measure without a command", func() Entry {
			p := pathlessPlace()
			p.Measure.Command = ""
			return tool(p)
		}, "measure.command is required"},
		{"command measure without a pattern", func() Entry {
			p := pathlessPlace()
			p.Measure.Pattern = ""
			return tool(p)
		}, "measure.pattern is required"},
		{"measure.pattern that does not compile", func() Entry {
			p := pathlessPlace()
			p.Measure.Pattern = `[`
			return tool(p)
		}, "measure.pattern:"},
		{"measure.pattern with two groups", func() Entry {
			p := pathlessPlace()
			p.Measure.Pattern = `(\d+)(\w+)`
			return tool(p)
		}, "measure.pattern must have exactly one group, it has 2"},
		{"measure.command with disk usage", func() Entry {
			p := cachePlace()
			p.Measure.Command = "tool df"
			return tool(p)
		}, "measure.command is legal only"},
		{"measure.pattern with disk usage", func() Entry {
			p := cachePlace()
			p.Measure.Pattern = `(\d+)`
			return tool(p)
		}, "measure.pattern is legal only"},
		{"measure.path outside the place", func() Entry {
			p := cachePlace()
			p.Measure.Path = "/var/cache"
			return tool(p)
		}, "measure.path must start with <path>"},
		{"measure.path that climbs", func() Entry {
			p := cachePlace()
			p.Measure.Path = "<path>/../other"
			return tool(p)
		}, "measure.path must not contain .."},

		{"clean.before with an open quote", func() Entry {
			p := cachePlace()
			p.Clean.Before = `tool "stop`
			return tool(p)
		}, "clean.before: a quote is not closed"},
		{"requires without an executable", func() Entry {
			e := withoutExecutable()
			e.Places[0].Requires = &Requires{Command: "plain ready"}
			return e
		}, "requires.command needs the entry's executable"},
		{"measure.command without an executable", func() Entry {
			e := withoutExecutable()
			e.Places[0].Measure = Measure{By: Command, Command: "plain size", Pattern: `(\d+)`}
			return e
		}, "measure.command needs the entry's executable"},
		{"clean.before without an executable", func() Entry {
			e := withoutExecutable()
			e.Places[0].Clean.Before = "plain stop"
			return e
		}, "clean.before needs the entry's executable"},
		{"list.command without an executable", func() Entry {
			e := withoutExecutable()
			e.Places[0].Holds = Versions
			e.Places[0].List = &List{Command: "plain list", Pattern: `(\S+)`}
			e.Places[0].Measure.Path = "<path>/<version>"
			return e
		}, "list.command needs the entry's executable"},
		{"clean.command without an executable", func() Entry {
			e := tool(cachePlace())
			e.Executable = ""
			e.Places[0].Path.Command = ""
			return e
		}, "clean.command needs the entry's executable"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := Validate(c.entry())
			if err == nil {
				t.Fatalf("accepted, want a problem containing %q", c.reason)
			}
			if !strings.Contains(err.Error(), c.reason) {
				t.Fatalf("problem %q does not contain %q", err, c.reason)
			}
		})
	}
}

func TestValidateReportsEveryProblem(t *testing.T) {
	p := cachePlace()
	p.Holds = ""
	p.Measure.By = ""
	err := Validate(tool(p))
	if err == nil {
		t.Fatal("accepted")
	}
	for _, want := range []string{"holds is required", "measure.by is required"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%q is missing from %q", want, err)
		}
	}
}

func TestSplitCommand(t *testing.T) {
	cases := []struct {
		command string
		want    []string
	}{
		{"go env GOCACHE", []string{"go", "env", "GOCACHE"}},
		{"  tool \t  cache   dir ", []string{"tool", "cache", "dir"}},
		{"docker system df --format '{{.Type}} {{.Size}}'", []string{"docker", "system", "df", "--format", "{{.Type}} {{.Size}}"}},
		{`tool --name "a b" c`, []string{"tool", "--name", "a b", "c"}},
		{`tool "it's" 'say "hi"'`, []string{"tool", "it's", `say "hi"`}},
		{`tool pre"mid dle"post`, []string{"tool", "premid dlepost"}},
		{`tool '' x`, []string{"tool", "", "x"}},
		{`tool a|b >c $HOME *.txt \n`, []string{"tool", "a|b", ">c", "$HOME", "*.txt", `\n`}},
	}
	for _, c := range cases {
		got, err := SplitCommand(c.command)
		if err != nil {
			t.Errorf("%q: %v", c.command, err)
			continue
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("%q: got %q, want %q", c.command, got, c.want)
		}
	}
}

func TestSplitCommandRejects(t *testing.T) {
	for command, reason := range map[string]string{
		"":            "empty",
		"   ":         "empty",
		"tool 'open":  "quote is not closed",
		`tool "open`:  "quote is not closed",
		`'' argument`: "empty program name",
	} {
		_, err := SplitCommand(command)
		if err == nil || !strings.Contains(err.Error(), reason) {
			t.Errorf("%q: got %v, want an error containing %q", command, err, reason)
		}
	}
}
