package main

import (
	"fmt"
	"strings"
)

type Store struct {
	Places []Place
	Rows   [][]Row
}

func NewStore(ps []Place) *Store {
	s := &Store{Places: ps, Rows: make([][]Row, len(ps))}
	for i, p := range ps {
		s.Rows[i] = []Row{{Key: p.Label, Group: p.Group, Label: p.Label, Kind: p.Kind, State: Pending}}
	}
	return s
}

func (s *Store) Apply(e Event) {
	if e.Replace {
		s.Rows[e.Place] = e.Rows
		return
	}
	for _, nr := range e.Rows {
		for j := range s.Rows[e.Place] {
			if s.Rows[e.Place][j].Key == nr.Key {
				s.Rows[e.Place][j] = nr
			}
		}
	}
}

func (s *Store) Flat() []Row {
	var out []Row
	for _, rs := range s.Rows {
		out = append(out, rs...)
	}
	return out
}

func (s *Store) Pending() int {
	n := 0
	for _, r := range s.Flat() {
		if r.State == Pending {
			n++
		}
	}
	return n
}

func humanSize(n int64) string {
	const k = 1024.0
	f := float64(n)
	for _, u := range []string{"B", "KB", "MB", "GB", "TB"} {
		if f < k || u == "TB" {
			if u == "B" {
				return fmt.Sprintf("%d B", n)
			}
			if f >= 100 {
				return fmt.Sprintf("%.0f %s", f, u)
			}
			return fmt.Sprintf("%.1f %s", f, u)
		}
		f /= k
	}
	return ""
}

func (r Row) SizeText() string {
	s := humanSize(r.Bytes)
	if r.Partial {
		s = "≥ " + s
	}
	return s
}

func (r Row) StatusText() string {
	switch r.State {
	case Absent:
		return "not present"
	case Undetermined:
		return "could not be determined" + noteSuffix(r.Note)
	case NotMeasured:
		return "not measured" + noteSuffix(r.Note)
	case Pending:
		return "measuring"
	}
	return r.SizeText()
}

func noteSuffix(n string) string {
	if n == "" {
		return ""
	}
	return " (" + n + ")"
}

func displayPath(p string) string {
	if h := home(); h != "" && strings.HasPrefix(p, h) {
		return "~" + strings.TrimPrefix(p, h)
	}
	return p
}
