package proxy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
)

// maxJSONDepth is how deeply nested a body can be. It is the limit of
// encoding/json, which no response of the Management API comes near.
const maxJSONDepth = 10000

// errTooDeep is the error for a body that is nested more deeply than
// maxJSONDepth.
var errTooDeep = fmt.Errorf("not valid JSON: nested more than %d levels deep", maxJSONDepth)

// jsonFrame is an object or an array that stripSecrets is inside of.
type jsonFrame struct {
	// An object frame is an object, not an array.
	object bool

	// An object frame that expects a key is waiting for its next key.
	expectKey bool

	// The items are how many were written to the frame so far.
	items int
}

// stripSecrets returns body, which must be one JSON value, without the object
// members whose name is a secret field. The name is compared without regard to
// case, at any depth, and the member is removed with its value, not set to null.
// Numbers are written exactly as they were read.
//
// It reads and writes tokens and keeps the objects it is inside of on a stack of
// its own, so a deeply nested body from a remote server cannot overflow the call
// stack, and it gives up on a body nested more than maxJSONDepth levels deep. It
// returns an error for anything that is not exactly one valid JSON value, because
// a body that cannot be read cannot be checked.
func stripSecrets(body []byte, isSecret func(name string) bool) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()

	var (
		out    bytes.Buffer
		frames []jsonFrame
		done   bool
	)

	// The output is never much longer than the body, so one allocation is enough.
	out.Grow(len(body))

	// The closed function marks that a value ended, so the enclosing object waits
	// for a key.
	closed := func() {
		if n := len(frames); n > 0 && frames[n-1].object {
			frames[n-1].expectKey = true
		}
	}

	// The begin function writes what comes before a value: the comma between
	// array items.
	begin := func() {
		if n := len(frames); n > 0 && !frames[n-1].object {
			if frames[n-1].items > 0 {
				out.WriteByte(',')
			}

			frames[n-1].items++
		}
	}

	for !done {
		token, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("not valid JSON: %w", err)
		}

		switch token := token.(type) {
		case json.Delim:
			if token == '}' || token == ']' {
				out.WriteByte(byte(token))
				frames = frames[:len(frames)-1]
				closed()
			} else {
				if len(frames) >= maxJSONDepth {
					return nil, errTooDeep
				}

				begin()
				out.WriteByte(byte(token))
				frames = append(frames, jsonFrame{object: token == '{', expectKey: token == '{'})
			}
		case string:
			if n := len(frames); n > 0 && frames[n-1].object && frames[n-1].expectKey {
				frame := &frames[n-1]

				// The frame keeps waiting for a key while the secret is skipped.
				if isSecret(token) {
					if err := skipValue(decoder, len(frames)); err != nil {
						return nil, err
					}

					continue
				}

				frame.expectKey = false

				if frame.items > 0 {
					out.WriteByte(',')
				}

				frame.items++
				writeString(&out, token)
				out.WriteByte(':')

				continue
			}

			begin()
			writeString(&out, token)
			closed()
		default:
			begin()
			writeScalar(&out, token)
			closed()
		}

		done = len(frames) == 0
	}

	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("not valid JSON: data after the first value")
	}

	return out.Bytes(), nil
}

// skipValue reads the next value from the decoder and drops it. The value can be
// an object or an array, which are skipped by counting how deep the decoder is.
// The outer depth is how deep the value is already, for the limit.
func skipValue(decoder *json.Decoder, outer int) error {
	depth := 0

	for {
		token, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("not valid JSON: %w", err)
		}

		if delim, ok := token.(json.Delim); ok {
			if delim == '{' || delim == '[' {
				depth++

				if outer+depth > maxJSONDepth {
					return errTooDeep
				}
			} else {
				depth--
			}
		}

		if depth == 0 {
			return nil
		}
	}
}

// writeString writes s as a JSON string.
func writeString(out *bytes.Buffer, s string) {
	encoded, _ := json.Marshal(s)
	out.Write(encoded)
}

// writeScalar writes a number, a boolean or null as it was read.
func writeScalar(out *bytes.Buffer, token json.Token) {
	switch token := token.(type) {
	case json.Number:
		out.WriteString(token.String())
	case bool:
		out.WriteString(strconv.FormatBool(token))
	default:
		out.WriteString("null")
	}
}
