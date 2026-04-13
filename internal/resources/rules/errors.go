package rules

import "errors"

var (
	ErrInvalidID         = errors.New("invalid rule id")
	ErrInvalidCollect    = errors.New("invalid collect request")
	ErrUnsupportedTarget = errors.New("unsupported target")
)
