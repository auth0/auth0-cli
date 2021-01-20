package auth0

import (
	"fmt"
	"time"
)

// Bool returns a pointer to the bool value passed in.
func Bool(b bool) *bool { return &b }

// BoolValue returns the value of the bool pointer passed in or false if the
// pointer is nil.
func BoolValue(b *bool) bool {
	if b != nil {
		return *b
	}
	return false
}

// Int returns a pointer to the int value passed in.
func Int(i int) *int {
	return &i
}

// IntValue returns the value of the int pointer passed in or 0 if the pointer
// is nil.
func IntValue(i *int) int {
	if i != nil {
		return *i
	}
	return 0
}

// Float64 returns a pointer to the float64 value passed in.
func Float64(f float64) *float64 {
	return &f
}

// Float64Value returns the value of the float64 pointer passed in or 0 if the pointer
// is nil.
func Float64Value(f *float64) float64 {
	if f != nil {
		return *f
	}
	return 0.00
}

// String returns a pointer to the string value passed in.
func String(s string) *string {
	return &s
}

// Stringf returns a pointer to the string value passed in formatted using
// fmt.Sprintf.
func Stringf(s string, v ...interface{}) *string {
	return String(fmt.Sprintf(s, v...))
}

// StringValue returns the value of the string pointer passed in or "" if the
// pointer is nil.
func StringValue(v *string) string {
	if v != nil {
		return *v
	}
	return ""
}

// Time returns a pointer to the time value passed in.
func Time(t time.Time) *time.Time {
	return &t
}

// TimeValue returns the value of the time pointer passed in or the zero value
// of time if the pointer is nil.
func TimeValue(t *time.Time) time.Time {
	if t != nil {
		return *t
	}
	return time.Time{}
}
