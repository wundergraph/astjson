package astjson

import (
	"errors"
	"strconv"
	"strings"
)

// Validate validates JSON s.
func Validate(s string) error {
	s = skipWS(s)

	tail, err := validateValue(s)
	if err != nil {
		return errors.New("cannot parse JSON: " + err.Error() + "; unparsed tail: " + strconv.Quote(startEndString(tail)))
	}
	tail = skipWS(tail)
	if len(tail) > 0 {
		return errors.New("unexpected tail: " + strconv.Quote(startEndString(tail)))
	}
	return nil
}

// ValidateBytes validates JSON b.
func ValidateBytes(b []byte) error {
	return Validate(b2s(b))
}

func validateValue(s string) (string, error) {
	if len(s) == 0 {
		return s, errParseEmpty
	}

	if s[0] == '{' {
		tail, err := validateObject(s[1:])
		if err != nil {
			return tail, errors.New("cannot parse object: " + err.Error())
		}
		return tail, nil
	}
	if s[0] == '[' {
		tail, err := validateArray(s[1:])
		if err != nil {
			return tail, errors.New("cannot parse array: " + err.Error())
		}
		return tail, nil
	}
	if s[0] == '"' {
		sv, tail, err := validateString(s[1:])
		if err != nil {
			return tail, errors.New("cannot parse string: " + err.Error())
		}
		// Scan the string for control chars.
		for i := 0; i < len(sv); i++ {
			if sv[i] < 0x20 {
				return tail, errors.New("string cannot contain control char 0x" + strconv.FormatUint(uint64(sv[i]), 16))
			}
		}
		return tail, nil
	}
	if s[0] == 't' {
		if len(s) < len("true") || s[:len("true")] != "true" {
			return s, errors.New("unexpected value found: " + strconv.Quote(s))
		}
		return s[len("true"):], nil
	}
	if s[0] == 'f' {
		if len(s) < len("false") || s[:len("false")] != "false" {
			return s, errors.New("unexpected value found: " + strconv.Quote(s))
		}
		return s[len("false"):], nil
	}
	if s[0] == 'n' {
		if len(s) < len("null") || s[:len("null")] != "null" {
			return s, errors.New("unexpected value found: " + strconv.Quote(s))
		}
		return s[len("null"):], nil
	}

	tail, err := validateNumber(s)
	if err != nil {
		return tail, errors.New("cannot parse number: " + err.Error())
	}
	return tail, nil
}

func validateArray(s string) (string, error) {
	s = skipWS(s)
	if len(s) == 0 {
		return s, errParseMissingCloseBracket
	}
	if s[0] == ']' {
		return s[1:], nil
	}

	for {
		var err error

		s = skipWS(s)
		s, err = validateValue(s)
		if err != nil {
			return s, errors.New("cannot parse array value: " + err.Error())
		}

		s = skipWS(s)
		if len(s) == 0 {
			return s, errParseUnexpectedEndArray
		}
		if s[0] == ',' {
			s = s[1:]
			continue
		}
		if s[0] == ']' {
			s = s[1:]
			return s, nil
		}
		return s, errParseMissingCommaArray
	}
}

func validateObject(s string) (string, error) {
	s = skipWS(s)
	if len(s) == 0 {
		return s, errParseMissingCloseBrace
	}
	if s[0] == '}' {
		return s[1:], nil
	}

	for {
		var err error

		// Parse key.
		s = skipWS(s)
		if len(s) == 0 || s[0] != '"' {
			return s, errParseMissingOpenQuote
		}

		var key string
		key, s, err = validateKey(s[1:])
		if err != nil {
			return s, errors.New("cannot parse object key: " + err.Error())
		}
		// Scan the key for control chars.
		for i := 0; i < len(key); i++ {
			if key[i] < 0x20 {
				return s, errors.New("object key cannot contain control char 0x" + strconv.FormatUint(uint64(key[i]), 16))
			}
		}
		s = skipWS(s)
		if len(s) == 0 || s[0] != ':' {
			return s, errParseMissingColon
		}
		s = s[1:]

		// Parse value
		s = skipWS(s)
		s, err = validateValue(s)
		if err != nil {
			return s, errors.New("cannot parse object value: " + err.Error())
		}
		s = skipWS(s)
		if len(s) == 0 {
			return s, errParseUnexpectedEndObject
		}
		if s[0] == ',' {
			s = s[1:]
			continue
		}
		if s[0] == '}' {
			return s[1:], nil
		}
		return s, errParseMissingCommaObject
	}
}

// validateKey is similar to validateString, but is optimized
// for typical object keys, which are quite small and have no escape sequences.
func validateKey(s string) (string, string, error) {
	for i := 0; i < len(s); i++ {
		if s[i] == '"' {
			// Fast path - the key doesn't contain escape sequences.
			return s[:i], s[i+1:], nil
		}
		if s[i] == '\\' {
			// Slow path - the key contains escape sequences.
			return validateString(s)
		}
	}
	return "", s, errParseMissingCloseQuote
}

func validateString(s string) (string, string, error) {
	// Try fast path - a string without escape sequences.
	if n := strings.IndexByte(s, '"'); n >= 0 && strings.IndexByte(s[:n], '\\') < 0 {
		return s[:n], s[n+1:], nil
	}

	// Slow path - escape sequences are present.
	rs, tail, _, err := parseRawStringInfo(s)
	if err != nil {
		return rs, tail, err
	}
	for {
		n := strings.IndexByte(rs, '\\')
		if n < 0 {
			return rs, tail, nil
		}
		n++
		ch := rs[n]
		rs = rs[n+1:]
		switch ch {
		case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
			// Valid escape sequences - see http://json.org/
			continue
		case 'u':
			if len(rs) < 4 {
				return rs, tail, errors.New(`too short escape sequence: \u` + rs)
			}
			xs := rs[:4]
			_, err := strconv.ParseUint(xs, 16, 16)
			if err != nil {
				return rs, tail, errors.New(`invalid escape sequence \u` + xs + ": " + err.Error())
			}
			rs = rs[4:]
		default:
			return rs, tail, errors.New(`unknown escape sequence \` + string(ch))
		}
	}
}

func validateNumber(s string) (string, error) {
	if len(s) == 0 {
		return s, errValidateZeroLenNumber
	}
	if s[0] == '-' {
		s = s[1:]
		if len(s) == 0 {
			return s, errValidateMissingAfterMinus
		}
	}
	i := 0
	for i < len(s) {
		if s[i] < '0' || s[i] > '9' {
			break
		}
		i++
	}
	if i <= 0 {
		return s, errors.New("expecting 0..9 digit, got " + string(s[0]))
	}
	if s[0] == '0' && i != 1 {
		return s, errValidateUnexpectedZero
	}
	if i >= len(s) {
		return "", nil
	}
	if s[i] == '.' {
		// Validate fractional part
		s = s[i+1:]
		if len(s) == 0 {
			return s, errValidateMissingFractional
		}
		i = 0
		for i < len(s) {
			if s[i] < '0' || s[i] > '9' {
				break
			}
			i++
		}
		if i == 0 {
			return s, errors.New("expecting 0..9 digit in fractional part, got " + string(s[0]))
		}
		if i >= len(s) {
			return "", nil
		}
	}
	if s[i] == 'e' || s[i] == 'E' {
		// Validate exponent part
		s = s[i+1:]
		if len(s) == 0 {
			return s, errValidateMissingExponent
		}
		if s[0] == '-' || s[0] == '+' {
			s = s[1:]
			if len(s) == 0 {
				return s, errValidateMissingExponent
			}
		}
		i = 0
		for i < len(s) {
			if s[i] < '0' || s[i] > '9' {
				break
			}
			i++
		}
		if i == 0 {
			return s, errors.New("expecting 0..9 digit in exponent part, got " + string(s[0]))
		}
		if i >= len(s) {
			return "", nil
		}
	}
	return s[i:], nil
}
