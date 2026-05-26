package repository

import (
	"context"

	"food-delivery-backend/internal/services/users/models"
)

type CreateOnboardingInput struct {
	UserID  string
	Role    string
	Status  string
	Country string
}

type CreateOnboardingDocumentInput struct {
	OnboardingID string
	DocumentType string
	S3Key        string
	UploadStatus string
}

type UpdateOnboardingStatusInput struct {
	OnboardingID    string
	Status          string
	RejectionReason *string
}

func (r *repo) ListRequiredDocumentTypes(ctx context.Context, role, country string) ([]models.DocumentTypeDefinitionRow, error) {
	return r.pg.ListRequiredDocumentTypes(ctx, role, country)
}

func (r *repo) CreateOnboarding(ctx context.Context, in CreateOnboardingInput) (*models.OnboardingRow, error) {
	return r.pg.CreateOnboarding(ctx, in.UserID, in.Role, in.Status)
}

func (r *repo) CreateOnboardingDocument(ctx context.Context, in CreateOnboardingDocumentInput) (*models.OnboardingDocumentRow, error) {
	return r.pg.CreateOnboardingDocument(ctx, in.OnboardingID, in.DocumentType, in.S3Key, in.UploadStatus)
}

func (r *repo) FindOnboardingByIDAndUser(ctx context.Context, onboardingID, userID string) (*models.OnboardingRow, error) {
	return r.pg.FindOnboardingByIDAndUser(ctx, onboardingID, userID)
}

func (r *repo) CountPendingOnboardingDocuments(ctx context.Context, onboardingID string) (int, error) {
	return r.pg.CountPendingOnboardingDocuments(ctx, onboardingID)
}

func (r *repo) UpdateOnboardingStatus(ctx context.Context, in UpdateOnboardingStatusInput) error {
	return r.pg.UpdateOnboardingStatus(ctx, in.OnboardingID, in.Status, in.RejectionReason)
}

func (r *repo) MarkOnboardingDocumentUploadedByS3Key(ctx context.Context, s3Key string) (bool, error) {
	return r.pg.MarkOnboardingDocumentUploadedByS3Key(ctx, s3Key)
}

func (r *repo) SetUserOnboardingComplete(ctx context.Context, userID string, isComplete bool) error {
	return r.pg.SetUserOnboardingComplete(ctx, userID, isComplete)
}
