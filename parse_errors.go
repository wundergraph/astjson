package astjson

import "errors"

// Pre-allocated sentinel errors for the parser / validator hot paths.
// Using pre-allocated errors.New values instead of fmt.Errorf keeps each
// call site allocation-free on the error branch and removes the need for
// `fmt` in the parse/validate files, which tightens inlining budgets.
var (
	errParseEmpty               = errors.New("cannot parse empty string")
	errParseMaxDepth            = errors.New("too big depth for the nested JSON; it exceeds 300")
	errParseMissingCloseBracket = errors.New("missing ']'")
	errParseMissingCloseBrace   = errors.New("missing '}'")
	errParseMissingCommaArray   = errors.New("missing ',' after array value")
	errParseMissingCommaObject  = errors.New("missing ',' after object value")
	errParseUnexpectedEndArray  = errors.New("unexpected end of array")
	errParseUnexpectedEndObject = errors.New("unexpected end of object")
	// Note: preserves the original (typo'd) text for API stability.
	errParseMissingOpenQuote  = errors.New(`cannot find opening '"" for object key`)
	errParseMissingColon      = errors.New("missing ':' after object key")
	errParseMissingCloseQuote = errors.New(`missing closing '"'`)

	// Validate-specific.
	errValidateZeroLenNumber     = errors.New("zero-length number")
	errValidateMissingAfterMinus = errors.New("missing number after minus")
	errValidateUnexpectedZero    = errors.New("unexpected number starting from 0")
	errValidateMissingFractional = errors.New("missing fractional part")
	errValidateMissingExponent   = errors.New("missing exponent part")
)
