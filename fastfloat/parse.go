package fastfloat

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ParseUint64BestEffort parses uint64 number s.
//
// It is equivalent to strconv.ParseUint(s, 10, 64), but is faster.
//
// 0 is returned if the number cannot be parsed.
// See also ParseUint64, which returns parse error if the number cannot be parsed.
func ParseUint64BestEffort(s string) uint64 {
	if len(s) == 0 {
		return 0
	}
	i := uint(0)
	d := uint64(0)
	j := i
	for i < uint(len(s)) {
		if s[i] >= '0' && s[i] <= '9' {
			d = d*10 + uint64(s[i]-'0')
			i++
			if i > 18 {
				// The integer part may be out of range for uint64.
				// Fall back to slow parsing.
				dd, err := strconv.ParseUint(s, 10, 64)
				if err != nil {
					return 0
				}
				return dd
			}
			continue
		}
		break
	}
	if i <= j {
		return 0
	}
	if i < uint(len(s)) {
		// Unparsed tail left.
		return 0
	}
	return d
}

// ParseUint64 parses uint64 from s.
//
// It is equivalent to strconv.ParseUint(s, 10, 64), but is faster.
//
// See also ParseUint64BestEffort.
func ParseUint64(s string) (uint64, error) {
	if len(s) == 0 {
		return 0, fmt.Errorf("cannot parse uint64 from empty string")
	}
	i := uint(0)
	d := uint64(0)
	j := i
	for i < uint(len(s)) {
		if s[i] >= '0' && s[i] <= '9' {
			d = d*10 + uint64(s[i]-'0')
			i++
			if i > 18 {
				// The integer part may be out of range for uint64.
				// Fall back to slow parsing.
				dd, err := strconv.ParseUint(s, 10, 64)
				if err != nil {
					return 0, err
				}
				return dd, nil
			}
			continue
		}
		break
	}
	if i <= j {
		return 0, fmt.Errorf("cannot parse uint64 from %q", s)
	}
	if i < uint(len(s)) {
		// Unparsed tail left.
		return 0, fmt.Errorf("unparsed tail left after parsing uint64 from %q: %q", s, s[i:])
	}
	return d, nil
}

// ParseInt64BestEffort parses int64 number s.
//
// It is equivalent to strconv.ParseInt(s, 10, 64), but is faster.
//
// 0 is returned if the number cannot be parsed.
// See also ParseInt64, which returns parse error if the number cannot be parsed.
func ParseInt64BestEffort(s string) int64 {
	if len(s) == 0 {
		return 0
	}
	i := uint(0)
	minus := s[0] == '-'
	if minus {
		i++
		if i >= uint(len(s)) {
			return 0
		}
	}

	d := int64(0)
	j := i
	for i < uint(len(s)) {
		if s[i] >= '0' && s[i] <= '9' {
			d = d*10 + int64(s[i]-'0')
			i++
			if i > 18 {
				// The integer part may be out of range for int64.
				// Fall back to slow parsing.
				dd, err := strconv.ParseInt(s, 10, 64)
				if err != nil {
					return 0
				}
				return dd
			}
			continue
		}
		break
	}
	if i <= j {
		return 0
	}
	if i < uint(len(s)) {
		// Unparsed tail left.
		return 0
	}
	if minus {
		d = -d
	}
	return d
}

// ParseInt64 parses int64 number s.
//
// It is equivalent to strconv.ParseInt(s, 10, 64), but is faster.
//
// See also ParseInt64BestEffort.
func ParseInt64(s string) (int64, error) {
	if len(s) == 0 {
		return 0, fmt.Errorf("cannot parse int64 from empty string")
	}
	i := uint(0)
	minus := s[0] == '-'
	if minus {
		i++
		if i >= uint(len(s)) {
			return 0, fmt.Errorf("cannot parse int64 from %q", s)
		}
	}

	d := int64(0)
	j := i
	for i < uint(len(s)) {
		if s[i] >= '0' && s[i] <= '9' {
			d = d*10 + int64(s[i]-'0')
			i++
			if i > 18 {
				// The integer part may be out of range for int64.
				// Fall back to slow parsing.
				dd, err := strconv.ParseInt(s, 10, 64)
				if err != nil {
					return 0, err
				}
				return dd, nil
			}
			continue
		}
		break
	}
	if i <= j {
		return 0, fmt.Errorf("cannot parse int64 from %q", s)
	}
	if i < uint(len(s)) {
		// Unparsed tail left.
		return 0, fmt.Errorf("unparsed tail left after parsing int64 form %q: %q", s, s[i:])
	}
	if minus {
		d = -d
	}
	return d, nil
}

// Exact powers of 10.
//
// This works faster than math.Pow10, since it avoids additional multiplication.
var float64pow10 = [...]float64{
	1e0, 1e1, 1e2, 1e3, 1e4, 1e5, 1e6, 1e7, 1e8, 1e9, 1e10, 1e11, 1e12, 1e13, 1e14, 1e15, 1e16,
}

// parseMantissaOverflow handles the case where the mantissa has too many digits
// for the fast float64pow10 table. It continues accumulating digits (up to 19 total),
// skips remaining fractional digits, parses any exponent, and uses eiselLemire64
// before falling back to strconv.ParseFloat.
//
// Parameters:
//   - s: the original input string
//   - d: the mantissa accumulated so far
//   - i, j, k: current position, start of integer digits, start of fractional digits
//   - minus: whether the number is negative
//
// Returns (result, didHandle). If didHandle is true, result is the parsed float.
// If didHandle is false, the caller should fall back to strconv.ParseFloat(s, 64).
func parseMantissaOverflow(s string, d uint64, i, j, k uint, minus bool) (float64, bool) {
	// Continue accumulating digits while d won't overflow uint64.
	// uint64 max is 18446744073709551615, so d*10+9 overflows when d > 1844674407370955161.
	const cutoff = 1844674407370955161
	for i < uint(len(s)) && s[i] >= '0' && s[i] <= '9' && d <= cutoff {
		d = d*10 + uint64(s[i]-'0')
		i++
	}
	mantDigits := int(i - k) // fractional digits accumulated into d
	// Skip remaining fractional digits beyond precision.
	for i < uint(len(s)) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	// Check for invalid trailing characters.
	if i < uint(len(s)) && s[i] != 'e' && s[i] != 'E' {
		return 0, false // invalid suffix; caller handles error
	}
	// Parse optional exponent.
	exp10 := -mantDigits
	if i < uint(len(s)) && (s[i] == 'e' || s[i] == 'E') {
		eExp, ok := parseExponent(s, i)
		if !ok {
			return 0, false // large/malformed exponent; caller falls back
		}
		exp10 += eExp
	}
	if result, ok := eiselLemire64(minus, d, exp10); ok {
		return result, true
	}
	return 0, false // eiselLemire64 declined; caller falls back
}

// parseExponent parses the exponent part of a float string starting
// at position i (which should point to 'e' or 'E'). It returns the exponent
// value and true on success, or 0 and false on failure.
func parseExponent(s string, i uint) (int, bool) {
	i++ // skip 'e' or 'E'
	if i >= uint(len(s)) {
		return 0, false
	}
	expMinus := false
	if s[i] == '+' || s[i] == '-' {
		expMinus = s[i] == '-'
		i++
		if i >= uint(len(s)) {
			return 0, false
		}
	}
	exp := 0
	j := i
	for i < uint(len(s)) {
		if s[i] >= '0' && s[i] <= '9' {
			exp = exp*10 + int(s[i]-'0')
			i++
			if exp > 400 {
				return 0, false // exponent too large
			}
			continue
		}
		break
	}
	if i <= j {
		return 0, false
	}
	if i < uint(len(s)) {
		return 0, false // unparsed tail
	}
	if expMinus {
		exp = -exp
	}
	return exp, true
}

// ParseBestEffort parses floating-point number s.
//
// It is equivalent to strconv.ParseFloat(s, 64), but is faster.
//
// 0 is returned if the number cannot be parsed.
// See also Parse, which returns parse error if the number cannot be parsed.
func ParseBestEffort(s string) float64 {
	if len(s) == 0 {
		return 0
	}
	i := uint(0)
	minus := s[0] == '-'
	if minus {
		i++
		if i >= uint(len(s)) {
			return 0
		}
	}

	// the integer part might be elided to remain compliant
	// with https://go.dev/ref/spec#Floating-point_literals
	if s[i] == '.' && (i+1 >= uint(len(s)) || s[i+1] < '0' || s[i+1] > '9') {
		return 0
	}

	d := uint64(0)
	j := i
	for i < uint(len(s)) {
		if s[i] >= '0' && s[i] <= '9' {
			d = d*10 + uint64(s[i]-'0')
			i++
			if i > 18 {
				// The integer part may be out of range for uint64.
				// Fall back to slow parsing.
				f, err := strconv.ParseFloat(s, 64)
				if err != nil && !math.IsInf(f, 0) {
					return 0
				}
				return f
			}
			continue
		}
		break
	}
	if i <= j && s[i] != '.' {
		s = s[i:]
		s = strings.TrimPrefix(s, "+")
		// "infinity" is needed for OpenMetrics support.
		// See https://github.com/OpenObservability/OpenMetrics/blob/master/OpenMetrics.md
		if strings.EqualFold(s, "inf") || strings.EqualFold(s, "infinity") {
			if minus {
				return -inf
			}
			return inf
		}
		if strings.EqualFold(s, "nan") {
			return nan
		}
		return 0
	}
	f := float64(d)
	if i >= uint(len(s)) {
		// Fast path - just integer.
		if minus {
			f = -f
		}
		return f
	}

	numFracDigits := 0
	if s[i] == '.' {
		// Parse fractional part.
		i++
		if i >= uint(len(s)) {
			// the fractional part may be elided to remain compliant
			// with https://go.dev/ref/spec#Floating-point_literals
			return f
		}
		k := i
		for i < uint(len(s)) {
			if s[i] >= '0' && s[i] <= '9' {
				d = d*10 + uint64(s[i]-'0')
				i++
				if i-j >= uint(len(float64pow10)) {
					// The mantissa is out of range for the fast table.
					// Try Eisel-Lemire with more digits before falling back.
					if result, ok := parseMantissaOverflow(s, d, i, j, k, minus); ok {
						return result
					}
					f, err := strconv.ParseFloat(s, 64)
					if err != nil && !math.IsInf(f, 0) {
						return 0
					}
					return f
				}
				continue
			}
			break
		}
		numFracDigits = int(i - k)
		// Convert the entire mantissa to a float at once to avoid rounding errors.
		f = float64(d) / float64pow10[numFracDigits]
		if i >= uint(len(s)) {
			// Fast path - parsed fractional number.
			if minus {
				f = -f
			}
			return f
		}
	}
	if s[i] == 'e' || s[i] == 'E' {
		// Parse exponent part.
		i++
		if i >= uint(len(s)) {
			return 0
		}
		expMinus := false
		if s[i] == '+' || s[i] == '-' {
			expMinus = s[i] == '-'
			i++
			if i >= uint(len(s)) {
				return 0
			}
		}
		exp := int16(0)
		j := i
		for i < uint(len(s)) {
			if s[i] >= '0' && s[i] <= '9' {
				exp = exp*10 + int16(s[i]-'0')
				i++
				if exp > 300 {
					// The exponent may be too big for float64.
					// Fall back to standard parsing.
					f, err := strconv.ParseFloat(s, 64)
					if err != nil && !math.IsInf(f, 0) {
						return 0
					}
					return f
				}
				continue
			}
			break
		}
		if i <= j {
			return 0
		}
		if expMinus {
			exp = -exp
		}
		// Use Eisel-Lemire for precise exponent computation.
		exp10 := int(exp) - numFracDigits
		if result, ok := eiselLemire64(minus, d, exp10); ok {
			if i >= uint(len(s)) {
				return result
			}
			// Unparsed tail - invalid number.
			return 0
		}
		f *= math.Pow10(int(exp))
		if i >= uint(len(s)) {
			if minus {
				f = -f
			}
			return f
		}
	}
	return 0
}

// Parse parses floating-point number s.
//
// It is equivalent to strconv.ParseFloat(s, 64), but is faster.
//
// See also ParseBestEffort.
func Parse(s string) (float64, error) {
	if len(s) == 0 {
		return 0, fmt.Errorf("cannot parse float64 from empty string")
	}
	i := uint(0)
	minus := s[0] == '-'
	if minus {
		i++
		if i >= uint(len(s)) {
			return 0, fmt.Errorf("cannot parse float64 from %q", s)
		}
	}

	// the integer part might be elided to remain compliant
	// with https://go.dev/ref/spec#Floating-point_literals
	if s[i] == '.' && (i+1 >= uint(len(s)) || s[i+1] < '0' || s[i+1] > '9') {
		return 0, fmt.Errorf("missing integer and fractional part in %q", s)
	}

	d := uint64(0)
	j := i
	for i < uint(len(s)) {
		if s[i] >= '0' && s[i] <= '9' {
			d = d*10 + uint64(s[i]-'0')
			i++
			if i > 18 {
				// The integer part may be out of range for uint64.
				// Fall back to slow parsing.
				f, err := strconv.ParseFloat(s, 64)
				if err != nil && !math.IsInf(f, 0) {
					return 0, err
				}
				return f, nil
			}
			continue
		}
		break
	}
	if i <= j && s[i] != '.' {
		ss := s[i:]
		ss = strings.TrimPrefix(ss, "+")
		// "infinity" is needed for OpenMetrics support.
		// See https://github.com/OpenObservability/OpenMetrics/blob/master/OpenMetrics.md
		if strings.EqualFold(ss, "inf") || strings.EqualFold(ss, "infinity") {
			if minus {
				return -inf, nil
			}
			return inf, nil
		}
		if strings.EqualFold(ss, "nan") {
			return nan, nil
		}
		return 0, fmt.Errorf("unparsed tail left after parsing float64 from %q: %q", s, ss)
	}
	f := float64(d)
	if i >= uint(len(s)) {
		// Fast path - just integer.
		if minus {
			f = -f
		}
		return f, nil
	}

	numFracDigits := 0
	if s[i] == '.' {
		// Parse fractional part.
		i++
		if i >= uint(len(s)) {
			// the fractional part might be elided to remain compliant
			// with https://go.dev/ref/spec#Floating-point_literals
			return f, nil
		}
		k := i
		for i < uint(len(s)) {
			if s[i] >= '0' && s[i] <= '9' {
				d = d*10 + uint64(s[i]-'0')
				i++
				if i-j >= uint(len(float64pow10)) {
					// The mantissa is out of range for the fast table.
					// Try Eisel-Lemire with more digits before falling back.
					if result, ok := parseMantissaOverflow(s, d, i, j, k, minus); ok {
						return result, nil
					}
					f, err := strconv.ParseFloat(s, 64)
					if err != nil && !math.IsInf(f, 0) {
						return 0, fmt.Errorf("cannot parse mantissa in %q: %s", s, err)
					}
					return f, nil
				}
				continue
			}
			break
		}
		numFracDigits = int(i - k)
		// Convert the entire mantissa to a float at once to avoid rounding errors.
		f = float64(d) / float64pow10[numFracDigits]
		if i >= uint(len(s)) {
			// Fast path - parsed fractional number.
			if minus {
				f = -f
			}
			return f, nil
		}
	}
	if s[i] == 'e' || s[i] == 'E' {
		// Parse exponent part.
		i++
		if i >= uint(len(s)) {
			return 0, fmt.Errorf("cannot parse exponent in %q", s)
		}
		expMinus := false
		if s[i] == '+' || s[i] == '-' {
			expMinus = s[i] == '-'
			i++
			if i >= uint(len(s)) {
				return 0, fmt.Errorf("cannot parse exponent in %q", s)
			}
		}
		exp := int16(0)
		j := i
		for i < uint(len(s)) {
			if s[i] >= '0' && s[i] <= '9' {
				exp = exp*10 + int16(s[i]-'0')
				i++
				if exp > 300 {
					// The exponent may be too big for float64.
					// Fall back to standard parsing.
					f, err := strconv.ParseFloat(s, 64)
					if err != nil && !math.IsInf(f, 0) {
						return 0, fmt.Errorf("cannot parse exponent in %q: %s", s, err)
					}
					return f, nil
				}
				continue
			}
			break
		}
		if i <= j {
			return 0, fmt.Errorf("cannot parse exponent in %q", s)
		}
		if expMinus {
			exp = -exp
		}
		// Use Eisel-Lemire for precise exponent computation.
		exp10 := int(exp) - numFracDigits
		if result, ok := eiselLemire64(minus, d, exp10); ok {
			if i >= uint(len(s)) {
				return result, nil
			}
			// Unparsed tail - invalid number.
			return 0, fmt.Errorf("cannot parse float64 from %q", s)
		}
		f *= math.Pow10(int(exp))
		if i >= uint(len(s)) {
			if minus {
				f = -f
			}
			return f, nil
		}
	}
	return 0, fmt.Errorf("cannot parse float64 from %q", s)
}

var inf = math.Inf(1)
var nan = math.NaN()
