package main

import (
	"fmt"
	"io"
	"strings"
)

func writePlain(w io.Writer, s *Store) {
	rows := s.Flat()
	gw, lw, sw := 5, 5, 4
	for _, r := range rows {
		gw, lw = max(gw, len(r.Group)), max(lw, len(r.Label))
		sw = max(sw, len(r.StatusText()))
	}
	fmt.Fprintf(w, "%-*s  %-*s  %-8s  %-*s  %s\n", gw, "TOOL", lw, "PLACE", "KIND", sw, "SIZE", "PATH")
	var total int64
	for _, r := range rows {
		kind := "cache"
		if r.Kind == KindVersion {
			kind = "version"
		}
		fmt.Fprintf(w, "%-*s  %-*s  %-8s  %-*s  %s\n", gw, r.Group, lw, r.Label, kind, sw, r.StatusText(), displayPath(r.Path))
		if r.Kind == KindCache && r.State == Sized {
			total += r.Bytes
		}
	}
	fmt.Fprintln(w, strings.Repeat("-", gw+lw+sw+16))
	fmt.Fprintf(w, "total of cache rows with a size: %s\n", humanSize(total))
}
