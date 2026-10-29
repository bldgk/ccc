package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/bldgk/ccc/internal/entry"
	"github.com/bldgk/ccc/internal/registry"
)

const userRegistry = home + "/.config/ccc/registry"

// written returns the entry in the one file the run wrote.
func (r *run) written(t *testing.T, file string) entry.Entry {
	t.Helper()
	if created := r.fake.Created(); !slices.Equal(created, []string{file}) {
		t.Fatalf("the files written are %q, want only %q\nstderr: %s", created, file, r.stderr.String())
	}
	data, _ := r.fake.FileContent(file)
	e, err := registry.Parse(file[strings.LastIndex(file, "/")+1:], data)
	if err != nil {
		t.Fatalf("%s does not load: %v\n%s", file, err, data)
	}
	return e
}

func (r *run) refused(t *testing.T, args []string, reason string) {
	t.Helper()
	before := r.fake.Created()
	if status := r.app.run(args); status != exitFailed {
		t.Fatalf("%q: exit status %d, want %d\nstdout: %s", args, status, exitFailed, r.stdout.String())
	}
	if !strings.Contains(r.stderr.String(), reason) {
		t.Fatalf("%q: stderr %q lacks %q", args, r.stderr.String(), reason)
	}
	if after := r.fake.Created(); !slices.Equal(after, before) || r.stdout.Len() != 0 {
		t.Fatalf("%q: wrote %q and printed %q", args, after, r.stdout.String())
	}
}

func TestAddFolderWritesTheEntryAndPrintsItsRow(t *testing.T) {
	r := newRun(t)
	r.fake.AddDir("/work/build/out", 2500)
	if status := r.app.run([]string{"add", "/work/build/out"}); status != 0 {
		t.Fatalf("exit status %d: %s", status, r.stderr.String())
	}
	file := userRegistry + "/out.toml"
	if got, want := r.written(t, file), (entry.Entry{Kind: entry.Folder, Name: "out", Path: "/work/build/out"}); got.Path != want.Path || got.Name != want.Name || got.Kind != want.Kind {
		t.Fatalf("the entry is %#v, want %#v", got, want)
	}
	if got, want := r.stdout.String(), "wrote "+file+"\nout  /work/build/out    2.5 kB\n"; got != want {
		t.Fatalf("printed\n%s\nwant\n%s", got, want)
	}
	if r.cleaned() {
		t.Fatal("adding a folder cleaned something")
	}
}

func TestAddFolderStoresAPathInsideHomeWithATilde(t *testing.T) {
	r := newRun(t)
	if status := r.app.run([]string{"add", home + "/project/.next"}); status != 0 {
		t.Fatalf("exit status %d: %s", status, r.stderr.String())
	}
	file := userRegistry + "/.next.toml"
	if got := r.written(t, file); got.Path != "~/project/.next" || got.Name != ".next" {
		t.Fatalf("the entry is %#v", got)
	}
	want := "wrote " + file + "\n.next  " + home + "/project/.next  the directory does not exist yet\n"
	if got := r.stdout.String(); got != want {
		t.Fatalf("printed\n%s\nwant\n%s", got, want)
	}
}

func TestAddFolderMakesARelativePathAbsolute(t *testing.T) {
	r := newRun(t)
	if status := r.app.run([]string{"add", "web/../node_modules/"}); status != 0 {
		t.Fatalf("exit status %d: %s", status, r.stderr.String())
	}
	if got := r.written(t, userRegistry+"/node_modules.toml"); got.Path != "~/project/node_modules" {
		t.Fatalf("the entry is %#v", got)
	}
}

func TestAddFolderTakesAnotherName(t *testing.T) {
	for _, args := range [][]string{
		{"add", "/work/build/out", "--name", "artifacts"},
		{"add", "--name", "artifacts", "/work/build/out"},
		{"add", "--name=artifacts", "/work/build/out"},
	} {
		r := newRun(t)
		if status := r.app.run(args); status != 0 {
			t.Fatalf("%q: exit status %d: %s", args, status, r.stderr.String())
		}
		if got := r.written(t, userRegistry+"/artifacts.toml"); got.Name != "artifacts" || got.Path != "/work/build/out" {
			t.Fatalf("%q: the entry is %#v", args, got)
		}
	}
}

func TestAddFolderWritesIntoTheConfiguredDirectory(t *testing.T) {
	r := newRun(t)
	r.fake.SetEnv("XDG_CONFIG_HOME", "/etc/xdg")
	if status := r.app.run([]string{"add", "/work/build/out"}); status != 0 {
		t.Fatalf("exit status %d: %s", status, r.stderr.String())
	}
	r.written(t, "/etc/xdg/ccc/registry/out.toml")
}

func TestAddFolderRefusesANameTheUserHas(t *testing.T) {
	r := newRun(t)
	if err := r.fake.CreateFile(userRegistry+"/out.toml", []byte("kind = \"folder\"\nname = \"out\"\npath = \"/work/other/out\"\n")); err != nil {
		t.Fatal(err)
	}
	r.refused(t, []string{"add", "/work/build/out"}, `you already have an entry named "out"`)
	data, _ := r.fake.FileContent(userRegistry + "/out.toml")
	if !strings.Contains(string(data), "/work/other/out") {
		t.Fatalf("the existing file changed:\n%s", data)
	}
}

func TestAddFolderAllowsTheNameOfAShippedEntry(t *testing.T) {
	shipped, err := registry.Shipped()
	if err != nil || len(shipped) == 0 {
		t.Fatalf("shipped entries: %d, %v", len(shipped), err)
	}
	name := shipped[0].Name
	r := newRun(t)
	if status := r.app.run([]string{"add", "/work/build/out", "--name", name}); status != 0 {
		t.Fatalf("exit status %d: %s", status, r.stderr.String())
	}
	if got := r.written(t, userRegistry+"/"+name+".toml"); got.Kind != entry.Folder || got.Name != name {
		t.Fatalf("the entry is %#v", got)
	}
	if want := "the new entry replaces the shipped entry " + name; !strings.Contains(r.stdout.String(), want) {
		t.Fatalf("the output lacks %q:\n%s", want, r.stdout.String())
	}
}

func TestAddFolderRefusesAShippedNameTakenFromThePath(t *testing.T) {
	shipped, err := registry.Shipped()
	if err != nil || len(shipped) == 0 {
		t.Fatalf("shipped entries: %d, %v", len(shipped), err)
	}
	r := newRun(t)
	r.refused(t, []string{"add", "/work/build/" + shipped[0].Name}, "is the name of a shipped entry")
}

func TestAddFolderRefusesWhatRemovalWouldRefuse(t *testing.T) {
	cases := map[string]string{
		"/":                  "the path is the root",
		home:                 "the path is the home directory",
		home + "/":           "the path is the home directory",
		"/home":              "the path contains the home directory",
		"/work":              "less than 2 levels below the root",
		"/work/link":         "the path is a symbolic link",
		"/work/archive.tar":  "the path is not a directory",
		"/work/a/../../work": "less than 2 levels below the root",
	}
	for path, reason := range cases {
		r := newRun(t)
		r.fake.AddSymlink("/work/link")
		r.fake.AddFile("/work/archive.tar", 10)
		r.refused(t, []string{"add", path}, reason)
	}
}

func TestAddFolderRefusesAPathAnEntryWouldReadDifferently(t *testing.T) {
	r := newRun(t)
	r.refused(t, []string{"add", "/work/build/$out"}, "contains $")
}

func TestAddFolderRefusesANameThatIsNotAFileName(t *testing.T) {
	r := newRun(t)
	r.refused(t, []string{"add", "/work/build/out", "--name", "../out"}, "path separator")
}

func TestAddWithoutAPathNeedsATerminal(t *testing.T) {
	for _, terminals := range [][2]bool{{false, false}, {true, false}, {false, true}} {
		r := newRun(t)
		r.app.inputTerminal, r.app.terminal = terminals[0], terminals[1]
		r.refused(t, []string{"add"}, "the form needs a terminal")
		if !strings.Contains(r.stderr.String(), "written by hand as a TOML file in "+userRegistry) {
			t.Fatalf("stderr does not say that a file can be written by hand: %q", r.stderr.String())
		}
		if len(r.forms) != 0 {
			t.Fatal("the form was opened")
		}
	}
}

func TestAddWithoutAPathOpensTheFormAndPrintsTheFile(t *testing.T) {
	r := newRun(t)
	r.app.inputTerminal, r.app.terminal = true, true
	r.formResult = userRegistry + "/alpha.toml"
	if status := r.app.run([]string{"add"}); status != 0 || len(r.forms) != 1 {
		t.Fatalf("exit status %d, forms %d: %s", status, len(r.forms), r.stderr.String())
	}
	if got, want := r.stdout.String(), "wrote "+r.formResult+"\n"; got != want {
		t.Fatalf("printed %q, want %q", got, want)
	}
}

func TestLeavingTheFormPrintsNothing(t *testing.T) {
	r := newRun(t)
	r.app.inputTerminal, r.app.terminal = true, true
	if status := r.app.run([]string{"add"}); status != 0 || r.stdout.Len() != 0 || r.stderr.Len() != 0 {
		t.Fatalf("exit status %d, stdout %q, stderr %q", status, r.stdout.String(), r.stderr.String())
	}
}

func TestTheFormWorksOnTheUsersRegistry(t *testing.T) {
	shipped, err := registry.Shipped()
	if err != nil || len(shipped) == 0 {
		t.Fatalf("shipped entries: %d, %v", len(shipped), err)
	}
	r := newRun(t)
	r.app.inputTerminal, r.app.terminal = true, true
	if status := r.app.run([]string{"add"}); status != 0 || len(r.forms) != 1 {
		t.Fatalf("exit status %d, forms %d", status, len(r.forms))
	}
	form := r.forms[0]

	e := cacheTool("beta")
	text, err := form.Text(e)
	if err != nil {
		t.Fatal(err)
	}
	if taken, err := form.UserHas("beta"); taken || err != nil {
		t.Fatalf("before saving: the user has the entry %v, %v", taken, err)
	}
	file, err := form.Save(e)
	if err != nil {
		t.Fatal(err)
	}
	if data, _ := r.fake.FileContent(userRegistry + "/beta.toml"); file != userRegistry+"/beta.toml" || string(data) != text {
		t.Fatalf("saved %q with\n%s\nwant the text shown\n%s", file, data, text)
	}
	if taken, err := form.UserHas("beta"); !taken || err != nil {
		t.Fatalf("after saving: the user has the entry %v, %v", taken, err)
	}
	if _, err := form.Save(e); err == nil {
		t.Fatal("the entry was saved over itself")
	}
	if !form.Shipped(shipped[0].Name) || form.Shipped("beta") {
		t.Fatal("the form does not tell a shipped name from a new one")
	}
	if taken, err := form.UserHas(shipped[0].Name); taken || err != nil {
		t.Fatalf("a shipped name counts as the user's: %v, %v", taken, err)
	}
}

func TestAddWithWrongArgumentsPrintsUsage(t *testing.T) {
	for _, args := range [][]string{
		{"add", "--name", "x"},
		{"add", "/work/a/b", "/work/c/d"},
		{"add", "/work/a/b", "--name"},
		{"add", "/work/a/b", "--force"},
		{"add", "/work/a/b", "--name", "x", "--name", "y"},
	} {
		r := newRun(t)
		r.app.inputTerminal, r.app.terminal = true, true
		if status := r.app.run(args); status != exitUsage || r.stderr.String() != usage {
			t.Fatalf("%q: exit status %d, stderr %q", args, status, r.stderr.String())
		}
		if len(r.fake.Created()) != 0 || len(r.forms) != 0 {
			t.Fatalf("%q: something was written or opened", args)
		}
	}
}

func TestHelpListsBothWaysToAdd(t *testing.T) {
	for _, want := range []string{"ccc add <path> [--name <name>]", "ccc add  "} {
		if !strings.Contains(usage, want) {
			t.Fatalf("the usage lacks %q", want)
		}
	}
}
