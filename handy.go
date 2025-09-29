package astjson

// GetString returns string value for the field identified by keys path
// in JSON data.
//
// Array indexes may be represented as decimal numbers in keys.
//
// An empty string is returned on error. Use Parser for proper error handling.
//
// Parser is faster for obtaining multiple fields from JSON.
func GetString(data []byte, keys ...string) string {
	var p Parser
	v, err := p.ParseBytes(data)
	if err != nil {
		return ""
	}
	sb := v.GetStringBytes(keys...)
	str := string(sb)
	return str
}

// GetBytes returns string value for the field identified by keys path
// in JSON data.
//
// Array indexes may be represented as decimal numbers in keys.
//
// nil is returned on error. Use Parser for proper error handling.
//
// Parser is faster for obtaining multiple fields from JSON.
func GetBytes(data []byte, keys ...string) []byte {
	var p Parser
	v, err := p.ParseBytes(data)
	if err != nil {
		return nil
	}
	sb := v.GetStringBytes(keys...)

	// Make a copy of sb, since sb belongs to p.
	var b []byte
	if sb != nil {
		b = append(b, sb...)
	}

	return b
}

// GetInt returns int value for the field identified by keys path
// in JSON data.
//
// Array indexes may be represented as decimal numbers in keys.
//
// 0 is returned on error. Use Parser for proper error handling.
//
// Parser is faster for obtaining multiple fields from JSON.
func GetInt(data []byte, keys ...string) int {
	var p Parser
	v, err := p.ParseBytes(data)
	if err != nil {
		return 0
	}
	n := v.GetInt(keys...)
	return n
}

// GetFloat64 returns float64 value for the field identified by keys path
// in JSON data.
//
// Array indexes may be represented as decimal numbers in keys.
//
// 0 is returned on error. Use Parser for proper error handling.
//
// Parser is faster for obtaining multiple fields from JSON.
func GetFloat64(data []byte, keys ...string) float64 {
	var p Parser
	v, err := p.ParseBytes(data)
	if err != nil {
		return 0
	}
	f := v.GetFloat64(keys...)
	return f
}

// GetBool returns boolean value for the field identified by keys path
// in JSON data.
//
// Array indexes may be represented as decimal numbers in keys.
//
// False is returned on error. Use Parser for proper error handling.
//
// Parser is faster for obtaining multiple fields from JSON.
func GetBool(data []byte, keys ...string) bool {
	var p Parser
	v, err := p.ParseBytes(data)
	if err != nil {
		return false
	}
	b := v.GetBool(keys...)
	return b
}

// Exists returns true if the field identified by keys path exists in JSON data.
//
// Array indexes may be represented as decimal numbers in keys.
//
// False is returned on error. Use Parser for proper error handling.
//
// Parser is faster when multiple fields must be checked in the JSON.
func Exists(data []byte, keys ...string) bool {
	var p Parser
	v, err := p.ParseBytes(data)
	if err != nil {
		return false
	}
	ok := v.Exists(keys...)
	return ok
}

// Parse parses json string s.
//
// The function is slower than the Parser.Parse for re-used Parser.
func Parse(s string) (*Value, error) {
	var p Parser
	return p.Parse(s)
}

func ParseWithoutCache(s string) (*Value, error) {
	var p Parser
	return p.ParseWithoutCache(s)
}

// MustParse parses json string s.
//
// The function panics if s cannot be parsed.
// The function is slower than the Parser.Parse for re-used Parser.
func MustParse(s string) *Value {
	v, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return v
}

// ParseBytes parses b containing json.
//
// The function is slower than the Parser.ParseBytes for re-used Parser.
func ParseBytes(b []byte) (*Value, error) {
	var p Parser
	return p.ParseBytes(b)
}

func ParseBytesWithoutCache(b []byte) (*Value, error) {
	var p Parser
	return p.ParseBytesWithoutCache(b)
}

// MustParseBytes parses b containing json.
//
// The function panics if b cannot be parsed.
// The function is slower than the Parser.ParseBytes for re-used Parser.
func MustParseBytes(b []byte) *Value {
	v, err := ParseBytes(b)
	if err != nil {
		panic(err)
	}
	return v
}
