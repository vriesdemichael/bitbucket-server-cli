package errors

import (
	"errors"
	"fmt"
)

type Kind string

const (
	KindAuthentication Kind = "authentication"
	KindAuthorization  Kind = "authorization"
	KindValidation     Kind = "validation"
	KindNotFound       Kind = "not_found"
	KindConflict       Kind = "conflict"
	KindTransient      Kind = "transient"
	KindPermanent      Kind = "permanent"
	KindNotImplemented Kind = "not_implemented"
	KindInternal       Kind = "internal"
)

type AppError struct {
	Kind    Kind
	Message string
	Cause   error
}

func New(kind Kind, message string, cause error) *AppError {
	return &AppError{
		Kind:    kind,
		Message: message,
		Cause:   cause,
	}
}

func (appError *AppError) Error() string {
	if appError.Cause == nil {
		return fmt.Sprintf("%s: %s", appError.Kind, appError.Message)
	}

	return fmt.Sprintf("%s: %s (%v)", appError.Kind, appError.Message, appError.Cause)
}

func (appError *AppError) Unwrap() error {
	return appError.Cause
}

func ExitCode(err error) int {
	if err == nil {
		return 0
	}

	var appError *AppError
	if errors.As(err, &appError) {
		switch appError.Kind {
		case KindValidation:
			return 2
		case KindAuthentication, KindAuthorization:
			return 3
		case KindNotFound:
			return 4
		case KindConflict:
			return 5
		case KindTransient:
			return 10
		case KindNotImplemented:
			return 11
		default:
			return 1
		}
	}

	return 1
}
