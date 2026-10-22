package plugins

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// keyFrame is an object or an array that RejectDuplicateKeys is inside of.
type keyFrame struct {
	// Whether the frame is an object or an array.
	object bool

	// The keys of an object that were read so far.
	seen map[string]bool

	// The key of the object member that is being read, only set while the value
	// of the member is being read.
	key string

	// The index of the array item that is being read.
	index int

	// Whether an object is waiting for its next key.
	expectKey bool
}

// RejectDuplicateKeys reports an object in data that has the same key twice,
// at any depth, and names the JSON path of that object, for example
// `$.plugins[0].versions[1]: duplicate key "permissions"`.
//
// Decoders do not agree on what a duplicate key means. A JSON Schema validator
// may check only the last value while encoding/json merges the values into the
// Go value, so a document can pass validation and still decode into something
// that was never validated. A document with a duplicate key is therefore
// rejected before it is validated or decoded. Keys are compared exactly, so keys
// that only differ in case are not duplicates here.
//
// It only adds the check for duplicates and takes whatever encoding/json takes
// otherwise. A document that is not valid JSON is reported as an error too, but
// a caller that needs a good message for that should check the syntax first.
func RejectDuplicateKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))

	// The frames are kept in a slice rather than on the call stack, so that a
	// deeply nested document cannot overflow the stack.
	var frames []keyFrame

	for {
		token, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("not valid JSON: %w", err)
		}

		var finished bool

		switch token := token.(type) {
		case json.Delim:
			switch token {
			case '{':
				frames = append(frames, keyFrame{object: true, seen: make(map[string]bool), expectKey: true})
			case '[':
				frames = append(frames, keyFrame{})
			default:
				frames = frames[:len(frames)-1]
				finished = true
			}
		case string:
			if len(frames) > 0 && frames[len(frames)-1].object && frames[len(frames)-1].expectKey {
				top := &frames[len(frames)-1]

				if top.seen[token] {
					return fmt.Errorf("%s: duplicate key %q", framesPath(frames[:len(frames)-1]), token)
				}

				top.seen[token] = true
				top.key = token
				top.expectKey = false

				continue
			}

			finished = true
		default:
			finished = true
		}

		// A value has been read completely when it is a scalar or the end of an
		// object or an array. Tell the object or array that it is in.
		if finished && len(frames) > 0 {
			parent := &frames[len(frames)-1]
			if parent.object {
				parent.expectKey = true
			} else {
				parent.index++
			}
		}

		if len(frames) == 0 {
			return nil
		}
	}
}

// UnmarshalStrict decodes data into v like json.Unmarshal does, but rejects an
// object that has the same key twice first, as RejectDuplicateKeys does. A
// document that is not valid JSON is reported as such.
func UnmarshalStrict(data []byte, v any) error {
	if err := RejectDuplicateKeys(data); err != nil {
		return err
	}

	return json.Unmarshal(data, v)
}

// framesPath renders the JSON path of the value that the frames lead to.
func framesPath(frames []keyFrame) string {
	var path strings.Builder

	path.WriteByte('$')

	for _, frame := range frames {
		switch {
		case !frame.object:
			fmt.Fprintf(&path, "[%d]", frame.index)
		case isPlainKey(frame.key):
			path.WriteString("." + frame.key)
		default:
			fmt.Fprintf(&path, "[%q]", frame.key)
		}
	}

	return path.String()
}

// isPlainKey reports whether key can be written after a dot in a JSON path.
func isPlainKey(key string) bool {
	if key == "" {
		return false
	}

	for _, r := range key {
		plain := r == '_' || r == '-' || r == '$' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
		if !plain {
			return false
		}
	}

	return true
}
