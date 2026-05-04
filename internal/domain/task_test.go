package domain

import (
	"errors"
	"testing"
)

func TestValidateTaskFields(t *testing.T) {
	tests := []struct {
		name string
		id   int64
		typ  int16
		val  int16
		want error
	}{
		{"ok", 1, 0, 0, nil},
		{"ok max", 99, 9, 99, nil},
		{"bad id", 0, 1, 1, ErrInvalidID},
		{"bad type low", 1, -1, 0, ErrInvalidType},
		{"bad type high", 1, 10, 0, ErrInvalidType},
		{"bad value low", 1, 0, -1, ErrInvalidValue},
		{"bad value high", 1, 0, 100, ErrInvalidValue},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateTaskFields(tc.id, tc.typ, tc.val)
			if tc.want == nil {
				if err != nil {
					t.Fatalf("unexpected err: %v", err)
				}
				return
			}
			if err == nil || !errors.Is(err, tc.want) {
				t.Fatalf("err: got %v want wrap %v", err, tc.want)
			}
		})
	}
}
