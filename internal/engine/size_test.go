package engine

import (
	"math"
	"strings"
	"testing"
)

func TestParseSize(t *testing.T) {
	cases := map[string]int64{
		"0":                   0,
		"4096":                4096,
		" 4096\n":             4096,
		"9223372036854775807": math.MaxInt64,
		"0B":                  0,
		"512 B":               512,
		"350kB":               350_000,
		"350 kB":              350_000,
		"12MB":                12_000_000,
		"4.2GB":               4_200_000_000,
		"4.2 GB":              4_200_000_000,
		"1.5TB":               1_500_000_000_000,
		"1KiB":                1024,
		"1.5KiB":              1536,
		"16 MiB":              16 * 1024 * 1024,
		"1.5GiB":              1_610_612_736,
		"2TiB":                2 * 1024 * 1024 * 1024 * 1024,
		"0.001kB":             1,
		"1.0004kB":            1000,
		"1.0005kB":            1001,
		"0.5B":                1,
		"0.4B":                0,
		"7.999999999GB":       7_999_999_999,
		"8388607.999TiB":      9223372035755264180,
	}
	for text, want := range cases {
		got, err := ParseSize(text)
		if err != nil {
			t.Errorf("%q: %v", text, err)
			continue
		}
		if got != want {
			t.Errorf("%q: got %d, want %d", text, got, want)
		}
	}
}

func TestParseSizeRejects(t *testing.T) {
	cases := map[string]string{
		"":                    "is not a size",
		"GB":                  "is not a size",
		"-5MB":                "is not a size",
		"1,5GB":               "is not a size",
		"1.GB":                "is not a size",
		".5GB":                "is not a size",
		"1e3":                 "is not a size",
		"12 parsecs":          "unknown unit",
		"12gb":                "unknown unit",
		"12K":                 "unknown unit",
		"1.5":                 "not a whole number of bytes",
		"9223372036854775808": "too large",
		"8388608TiB":          "too large",
		"10000000000GB":       "too large",
	}
	for text, reason := range cases {
		got, err := ParseSize(text)
		if err == nil {
			t.Errorf("%q: accepted as %d", text, got)
			continue
		}
		if !strings.Contains(err.Error(), reason) {
			t.Errorf("%q: got %q, want an error containing %q", text, err, reason)
		}
	}
}
