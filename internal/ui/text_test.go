package ui

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"
)

func TestFormatSize(t *testing.T) {
	cases := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{999, "999 B"},
		{1000, "1 kB"},
		{1049, "1 kB"},
		{1050, "1.1 kB"},
		{1500, "1.5 kB"},
		{99_949, "99.9 kB"},
		{99_950, "100 kB"},
		{999_499, "999 kB"},
		{999_500, "1 MB"},
		{1_000_000, "1 MB"},
		{20_480_000, "20.5 MB"},
		{999_999_999, "1 GB"},
		{1_500_000_000, "1.5 GB"},
		{4_200_000_000, "4.2 GB"},
		{250_000_000_000, "250 GB"},
		{1_000_000_000_000, "1 TB"},
		{5_000_000_000_000_000, "5000 TB"},
		{-1500, "-1.5 kB"},
	}
	for _, c := range cases {
		if got := FormatSize(c.bytes); got != c.want {
			t.Errorf("FormatSize(%d) = %q, want %q", c.bytes, got, c.want)
		}
	}
}

func TestWriteRowsShowsAReasonInsteadOfASize(t *testing.T) {
	_, eng := fixture(t)
	var out bytes.Buffer
	WriteRows(Output(&out, []string{}), Collect(context.Background(), eng))

	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != rowCount {
		t.Fatalf("got %d lines, want %d:\n%s", len(lines), rowCount, out.String())
	}
	want := []string{
		"alpha    cache              1.5 kB",
		"broken   cache              2 kB",
		"chain    toolchains     v1  300 B",
		"chain    toolchains     v2  400 B",
		"dark     cache              not measured (",
		"lost     cache              could not be determined (path.command: `lost where`: exit status 3: no configuration",
		"scratch  /work/scratch      700 B",
	}
	for i, prefix := range want {
		if !strings.HasPrefix(lines[i], prefix) {
			t.Errorf("line %d is %q, want it to start with %q", i, lines[i], prefix)
		}
	}
	for _, i := range []int{rowDark, rowLost} {
		if strings.Contains(lines[i], " B") {
			t.Errorf("line %d shows a size for a row that has none: %q", i, lines[i])
		}
	}
}

func TestCleanEverything(t *testing.T) {
	f, eng := fixture(t)
	var out bytes.Buffer
	totals := CleanEverything(context.Background(), eng, Output(&out, []string{}))

	if want := (Totals{Freed: 2200, Failed: 2, Unknown: 1}); totals != want {
		t.Fatalf("totals %+v, want %+v", totals, want)
	}
	want := []string{
		"alpha    cache          freed 1.5 kB",
		"broken   cache          failed: `broken clean`: exit status 1: boom: the cache is locked",
		"dark     cache          cleaned, freed size unknown: the size before cleaning is not known: ",
		"lost     cache          failed: the place could not be determined: ",
		"scratch  /work/scratch  freed 700 B",
		"total freed 2.2 kB, 2 failed, 1 with freed size unknown",
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d:\n%s", len(lines), len(want), out.String())
	}
	for i, prefix := range want {
		if !strings.HasPrefix(lines[i], prefix) {
			t.Errorf("line %d is %q, want it to start with %q", i, lines[i], prefix)
		}
	}
	if strings.Contains(out.String(), "\x1b") {
		t.Errorf("output that is not a terminal carries colour: %q", out.String())
	}
	for _, command := range cleanCommandsRan(f) {
		if strings.Contains(command, "uninstall") {
			t.Errorf("clean everything ran %q: a version was touched", command)
		}
	}
	if got, want := f.Removed(), []string{home + "/.cache/dark", "/work/scratch"}; !slices.Equal(got, want) {
		t.Errorf("removed %q, want %q", got, want)
	}
}
