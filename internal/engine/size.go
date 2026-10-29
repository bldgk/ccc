package engine

import (
	"fmt"
	"math/big"
	"regexp"
	"strings"
)

// The units tools print sizes in. The decimal ones are powers of 1000, the binary ones
// powers of 1024.
var sizeUnits = map[string]int64{
	"":    1,
	"B":   1,
	"kB":  1000,
	"MB":  1000 * 1000,
	"GB":  1000 * 1000 * 1000,
	"TB":  1000 * 1000 * 1000 * 1000,
	"KiB": 1 << 10,
	"MiB": 1 << 20,
	"GiB": 1 << 30,
	"TiB": 1 << 40,
}

var sizeForm = regexp.MustCompile(`^([0-9]+)(?:\.([0-9]+))?\s*([A-Za-z]*)$`)

// ParseSize reads a size as a tool prints it: a whole number of bytes, or a number with
// a unit, with or without a space and with or without a decimal fraction. A fraction of
// a byte is rounded to the nearest byte.
func ParseSize(text string) (int64, error) {
	m := sizeForm.FindStringSubmatch(strings.TrimSpace(text))
	if m == nil {
		return 0, fmt.Errorf("%q is not a size", text)
	}
	whole, fraction, unit := m[1], m[2], m[3]
	factor, ok := sizeUnits[unit]
	if !ok {
		return 0, fmt.Errorf("%q has an unknown unit %q", text, unit)
	}
	if unit == "" && fraction != "" {
		return 0, fmt.Errorf("%q is not a whole number of bytes and has no unit", text)
	}

	// Exact arithmetic: 4.2 * 10^9 is not 4200000000 in floating point.
	value, _ := new(big.Rat).SetString(whole + "." + fraction + "0")
	value.Mul(value, new(big.Rat).SetInt64(factor))
	half := big.NewRat(1, 2)
	rounded := new(big.Int).Quo(value.Add(value, half).Num(), value.Denom())
	if !rounded.IsInt64() {
		return 0, fmt.Errorf("%q is too large", text)
	}
	return rounded.Int64(), nil
}
