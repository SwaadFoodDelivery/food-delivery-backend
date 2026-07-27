package postgres

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"

	apperrors "food-delivery-backend/internal/errors"

	"github.com/lib/pq"
)

// A lost race against a unique index must reach the business layer as
// ErrConflict so it becomes a 409, not a generic 500. lib/pq reports these as
// *pq.Error with SQLSTATE 23505.
func TestMapUniqueViolation(t *testing.T) {
	uniqueViolation := &pq.Error{
		Code:       "23505",
		Message:    `duplicate key value violates unique constraint "uq_users_role_email_active_ci"`,
		Constraint: "uq_users_role_email_active_ci",
	}

	tests := []struct {
		name string
		in   error
		want error
	}{
		{"nil stays nil", nil, nil},
		{"unique violation becomes conflict", uniqueViolation, apperrors.ErrConflict},
		{"wrapped unique violation becomes conflict", fmt.Errorf("insert user: %w", uniqueViolation), apperrors.ErrConflict},
		{"foreign key violation passes through", &pq.Error{Code: "23503"}, &pq.Error{Code: "23503"}},
		{"no rows passes through", sql.ErrNoRows, sql.ErrNoRows},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapUniqueViolation(tt.in)

			if tt.want == nil {
				if got != nil {
					t.Fatalf("expected nil, got %v", got)
				}
				return
			}
			if errors.Is(tt.want, apperrors.ErrConflict) {
				if !errors.Is(got, apperrors.ErrConflict) {
					t.Fatalf("expected ErrConflict, got %v", got)
				}
				return
			}
			// Anything that is not a unique violation must be returned untouched
			// so existing not-found and internal-error handling still works.
			if errors.Is(got, apperrors.ErrConflict) {
				t.Fatalf("non-unique-violation error was misclassified as conflict: %v", tt.in)
			}
			if got.Error() != tt.want.Error() {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

// mapNotFound and mapUniqueViolation share the same call sites; a 23505 must not
// be swallowed as a not-found and vice versa.
func TestMapNotFoundAndUniqueViolationAreDistinct(t *testing.T) {
	if errors.Is(mapNotFound(sql.ErrNoRows), apperrors.ErrConflict) {
		t.Fatal("sql.ErrNoRows must not map to ErrConflict")
	}
	if errors.Is(mapUniqueViolation(&pq.Error{Code: "23505"}), apperrors.ErrNotFound) {
		t.Fatal("unique violation must not map to ErrNotFound")
	}
}
