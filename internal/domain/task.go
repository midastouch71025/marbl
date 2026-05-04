package domain

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidID    = errors.New("id must be positive")
	ErrInvalidType  = errors.New("type must be 0..9")
	ErrInvalidValue = errors.New("value must be 0..99")
)

func ValidateTaskFields(id int64, typ int16, val int16) error {
	if id < 1 {
		return ErrInvalidID
	}
	if typ < 0 || typ > 9 {
		return fmt.Errorf("%w: got %d", ErrInvalidType, typ)
	}
	if val < 0 || val > 99 {
		return fmt.Errorf("%w: got %d", ErrInvalidValue, val)
	}
	return nil
}
