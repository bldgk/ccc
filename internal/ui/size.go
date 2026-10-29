package ui

import "strconv"

var sizeUnits = []string{"kB", "MB", "GB", "TB"}

// FormatSize writes a number of bytes in decimal units, as the tools themselves print
// sizes: powers of 1000, one decimal below a hundred of the unit, none from there on,
// and a fraction of zero left out.
func FormatSize(bytes int64) string {
	sign := ""
	n := uint64(bytes)
	if bytes < 0 {
		sign = "-"
		n = -n
	}
	if n < 1000 {
		return sign + strconv.FormatUint(n, 10) + " B"
	}
	divisor := uint64(1)
	for i, unit := range sizeUnits {
		divisor *= 1000
		whole, rest := n/divisor, n%divisor
		tenths := whole*10 + (rest*10+divisor/2)/divisor
		if tenths < 1000 {
			text := strconv.FormatUint(tenths/10, 10)
			if tenths%10 != 0 {
				text += "." + strconv.FormatUint(tenths%10, 10)
			}
			return sign + text + " " + unit
		}
		if rest*2 >= divisor {
			whole++
		}
		// A value that rounds up to a thousand of this unit is one of the next unit.
		if whole < 1000 || i == len(sizeUnits)-1 {
			return sign + strconv.FormatUint(whole, 10) + " " + unit
		}
	}
	panic("unreachable: the last unit always returns")
}
