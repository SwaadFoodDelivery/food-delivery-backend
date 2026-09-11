package models

import (
	"database/sql"
	"time"
)

type InitOnboardingInput struct {
	UserID  string
	Role    string
	Country string
}

type SubmitOnboardingInput struct {
	UserID       string
	OnboardingID string
}

type ResubmitOnboardingInput struct {
	UserID       string
	OnboardingID string
}

type MarkDocumentUploadedInput struct {
	UserID string
	S3Key  string
}

type OnboardingDocumentUpload struct {
	DocumentID   string `json:"document_id"`
	DocumentType string `json:"document_type"`
	S3Key        string `json:"s3_key"`
	UploadURL    string `json:"upload_url"`
	Method       string `json:"method"`
	ExpiresAt    string `json:"expires_at"`
	UploadStatus string `json:"upload_status"`
}

type InitOnboardingOutput struct {
	RejectionReason string                     `json:"rejection_reason,omitempty"`
	OnboardingID    string                     `json:"onboarding_id"`
	Status          string                     `json:"status"`
	Role            string                     `json:"role"`
	Documents       []OnboardingDocumentUpload `json:"documents"`
}

type SubmitOnboardingOutput struct {
	OnboardingID string `json:"onboarding_id"`
	Status       string `json:"status"`
	Message      string `json:"message"`
}

type ResubmitOnboardingOutput struct {
	OnboardingID string `json:"onboarding_id"`
	Status       string `json:"status"`
	Message      string `json:"message"`
}

type OnboardingReviewItem struct {
	OnboardingID    string    `json:"onboarding_id" db:"onboarding_id"`
	UserID          string    `json:"user_id" db:"user_id"`
	UserName        string    `json:"user_name" db:"user_name"`
	Phone           string    `json:"phone" db:"phone"`
	Email           string    `json:"email,omitempty" db:"email"`
	Role            string    `json:"role" db:"role"`
	Status          string    `json:"status" db:"status"`
	RejectionReason string    `json:"rejection_reason,omitempty" db:"rejection_reason"`
	RequiredDocs    int       `json:"required_documents" db:"required_documents"`
	UploadedDocs    int       `json:"uploaded_documents" db:"uploaded_documents"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}

type ReviewOnboardingInput struct {
	ActorID         string
	OnboardingID    string
	Status          string
	RejectionReason string
}

type ReviewOnboardingOutput struct {
	OnboardingID string `json:"onboarding_id"`
	Status       string `json:"status"`
	Message      string `json:"message"`
}

type InitOnboardingRequest struct {
	Role    string `json:"role"`
	Country string `json:"country"`
}

type OnboardingRow struct {
	OnboardingID    string         `db:"onboarding_id"`
	UserID          string         `db:"user_id"`
	Role            string         `db:"role"`
	Status          string         `db:"status"`
	RejectionReason sql.NullString `db:"rejection_reason"`
	CreatedAt       time.Time      `db:"created_at"`
	UpdatedAt       time.Time      `db:"updated_at"`
}

type OnboardingDocumentRow struct {
	DocumentID   string         `db:"document_id"`
	OnboardingID string         `db:"onboarding_id"`
	DocumentType string         `db:"document_type"`
	S3Key        sql.NullString `db:"s3_key"`
	UploadStatus string         `db:"upload_status"`
	CreatedAt    time.Time      `db:"created_at"`
	UpdatedAt    time.Time      `db:"updated_at"`
}

type DocumentTypeDefinitionRow struct {
	DocumentType      string    `db:"document_type"`
	ApplicableRole    string    `db:"applicable_role"`
	ApplicableCountry string    `db:"applicable_country"`
	IsRequired        bool      `db:"is_required"`
	CreatedAt         time.Time `db:"created_at"`
}
