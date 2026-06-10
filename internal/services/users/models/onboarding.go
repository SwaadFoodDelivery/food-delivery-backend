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
	S3Key string
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
	OnboardingID string                     `json:"onboarding_id"`
	Status       string                     `json:"status"`
	Role         string                     `json:"role"`
	Documents    []OnboardingDocumentUpload `json:"documents"`
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
