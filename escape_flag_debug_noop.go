//go:build !astjson_debug

package astjson

// debugVerifyEscapeHint is a no-op when the astjson_debug build tag is
// not set. See escape_flag_debug.go for the active implementation.
func (v *Value) debugVerifyEscapeHint() {}
