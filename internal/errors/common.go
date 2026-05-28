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

type AppError struct {
	Code    string   `json:"error_code"`
	Message string   `json:"message"`
	Details []string `json:"details"`
}

func (e *AppError) Error() string { return e.Message }

func IsNotFound(err error) bool {
	return stderrors.Is(err, ErrNotFound)
}
