package httpjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// typeMismatchErr returns a real *json.UnmarshalTypeError from unmarshalling a
// number into a string field, so the tests exercise the stdlib error shapes
// rather than hand-rolled fakes.
func typeMismatchErr(t *testing.T) error {
	t.Helper()
	var dst struct {
		TemplateID string `json:"template_id"`
	}
	err := json.Unmarshal([]byte(`{"template_id": 123}`), &dst)
	require.Error(t, err)
	return err
}

func TestDecodeError_NamesFieldOnTypeMismatch(t *testing.T) {
	msg := DecodeError(typeMismatchErr(t))
	assert.Contains(t, msg, `"template_id"`, "the offending field is named")
	assert.Contains(t, msg, "string", "the expected type is named")
}

func TestDecodeError_TypeMismatchWithoutField(t *testing.T) {
	// A number decoded straight into a string (no surrounding struct) yields a
	// type error with an empty Field but a set Offset.
	var dst string
	err := json.Unmarshal([]byte(`123`), &dst)
	require.Error(t, err)

	msg := DecodeError(err)
	assert.Contains(t, msg, "wrong type")
	assert.Contains(t, msg, "string")
}

func TestDecodeError_SyntaxError(t *testing.T) {
	var dst map[string]any
	err := json.Unmarshal([]byte(`{"a":`), &dst)
	require.Error(t, err)

	msg := DecodeError(err)
	assert.Contains(t, msg, "malformed JSON")
}

func TestDecodeError_UnknownField(t *testing.T) {
	var dst struct {
		Name string `json:"name"`
	}
	dec := json.NewDecoder(strings.NewReader(`{"name":"x","extra_field":1}`))
	dec.DisallowUnknownFields()
	err := dec.Decode(&dst)
	require.Error(t, err)

	msg := DecodeError(err)
	assert.Contains(t, msg, "unknown field")
	assert.Contains(t, msg, "extra_field")
}

func TestDecodeError_UnknownFieldThroughWrapper(t *testing.T) {
	var dst struct {
		Name string `json:"name"`
	}
	dec := json.NewDecoder(strings.NewReader(`{"extra_field":1}`))
	dec.DisallowUnknownFields()
	raw := dec.Decode(&dst)
	require.Error(t, raw)

	// Homebrew historically wrapped decode errors; the field name must still
	// be lifted out through the wrapper prefix.
	wrapped := errors.New("invalid request body: " + raw.Error())
	msg := DecodeError(wrapped)
	assert.Contains(t, msg, "unknown field")
	assert.Contains(t, msg, "extra_field")
}

func TestDecodeError_EmptyBody(t *testing.T) {
	var dst struct{ A int }
	err := json.NewDecoder(bytes.NewReader(nil)).Decode(&dst)
	require.ErrorIs(t, err, io.EOF)

	assert.Contains(t, DecodeError(err), "empty")
}

func TestDecodeError_TruncatedBody(t *testing.T) {
	assert.Contains(t, DecodeError(io.ErrUnexpectedEOF), "truncated")
}

func TestDecodeError_Fallback(t *testing.T) {
	assert.Equal(t, "invalid JSON body", DecodeError(errors.New("something odd")))
}
