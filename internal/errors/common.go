package errors

import stderrors "errors"

const (
	CodeValidation       = "VALIDATION_ERROR"
	CodeUnauthorized     = "UNAUTHORIZED"
	CodeForbidden        = "FORBIDDEN"
	CodeInternal         = "INTERNAL_ERROR"
	CodeRateLimited      = "RATE_LIMIT_EXCEEDED"
	CodeUserNotFound     = "USER_NOT_FOUND"
	CodeAccountNotActive = "ACCOUNT_NOT_ACTIVE"
)

var ErrNotFound = stderrors.New("not found")

// ErrConflict is what a unique-index violation is mapped to, so the business
// layer can turn a lost race into a 409 instead of a generic 500.
var ErrConflict = stderrors.New("conflict")

type AppError struct {
	Code    string   `json:"error_code"`
	Message string   `json:"message"`
	Details []string `json:"details"`
}

func (e *AppError) Error() string { return e.Message }

func IsNotFound(err error) bool {
	return stderrors.Is(err, ErrNotFound)
}

func IsConflict(err error) bool {
	return stderrors.Is(err, ErrConflict)
}
