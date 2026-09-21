package core

import "fmt"

// ErrorKind is a stable, machine-readable category for domain failures.
type ErrorKind string

const (
	ErrInvalidState       ErrorKind = "INVALID_STATE"
	ErrUnsupportedVersion ErrorKind = "UNSUPPORTED_VERSION"
	ErrUnknownNamespace   ErrorKind = "UNKNOWN_NAMESPACE"
)

// DomainError preserves a stable kind while retaining a human-readable cause.
type DomainError struct {
	Kind    ErrorKind
	Message string
}

func (e *DomainError) Error() string {
	return fmt.Sprintf("%s: %s", e.Kind, e.Message)
}

func newDomainError(kind ErrorKind, format string, args ...any) error {
	return &DomainError{Kind: kind, Message: fmt.Sprintf(format, args...)}
}
