package access

import "errors"

// Resource access errors retain distinct adapter response mappings.
var (
	ErrUnauthorized = errors.New("kb_access: unauthorized")
	ErrNotFound     = errors.New("kb_access: not found")
	ErrForbidden    = errors.New("kb_access: forbidden")
)
