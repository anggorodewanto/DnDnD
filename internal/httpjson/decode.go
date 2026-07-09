// Package httpjson holds small helpers for JSON HTTP handlers that are shared
// across service packages (combat, homebrew, ...).
package httpjson

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// DecodeError converts a json.Decoder / json.Unmarshal error into a short,
// field-level message suitable for a 400 response body. Where the standard
// library exposes it, the message names the offending field and expected type
// instead of a bare "invalid JSON body", so a caller sees *which* field was
// wrong (APP-7). err must be non-nil.
func DecodeError(err error) string {
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		if typeErr.Field != "" {
			return fmt.Sprintf("field %q has the wrong type (expected %s)", typeErr.Field, typeErr.Type)
		}
		return fmt.Sprintf("wrong type at byte %d (expected %s)", typeErr.Offset, typeErr.Type)
	}

	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return fmt.Sprintf("malformed JSON at byte %d", syntaxErr.Offset)
	}

	// DisallowUnknownFields surfaces as `json: unknown field "x"`; the field
	// name is already the useful part, so lift it out (tolerating a wrapper
	// prefix like `invalid request body: `).
	const unknownField = "unknown field "
	if _, after, found := strings.Cut(err.Error(), unknownField); found {
		return unknownField + after
	}

	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return "request body is empty or truncated"
	}

	return "invalid JSON body"
}
