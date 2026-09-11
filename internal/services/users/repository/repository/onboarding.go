package repository

import (
	"context"
	stderrors "errors"

	"food-delivery-backend/internal/services/users/models"
	postgresstore "food-delivery-backend/internal/services/users/repository/postgres"
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
	ExpectedStatus  string
	OnboardingID    string
	Status          string
	RejectionReason *string
}

type ReviewOnboardingInput struct {
	ActorID         string
	OnboardingID    string
	Status          string
	RejectionReason string
}

func (r *repo) ListOnboardingReviews(ctx context.Context, status string) ([]models.OnboardingReviewItem, error) {
	return r.pg.ListOnboardingReviews(ctx, status)
}

func (r *repo) ReviewOnboarding(ctx context.Context, in ReviewOnboardingInput) (*models.OnboardingReviewItem, error) {
	if r.tx != nil {
		return r.mapReviewError(r.pg.ReviewOnboarding(ctx, in.ActorID, in.OnboardingID, in.Status, in.RejectionReason))
	}
	var item *models.OnboardingReviewItem
	err := r.WithTx(ctx, func(tx Repository) error {
		var txErr error
		item, txErr = tx.ReviewOnboarding(ctx, in)
		return txErr
	})
	return item, err
}

func (r *repo) mapReviewError(item *models.OnboardingReviewItem, err error) (*models.OnboardingReviewItem, error) {
	if stderrors.Is(err, postgresstore.ErrOnboardingAlreadyReviewed) {
		return nil, ErrOnboardingAlreadyReviewed
	}
	return item, err
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
	return r.pg.UpdateOnboardingStatus(ctx, in.OnboardingID, in.ExpectedStatus, in.Status, in.RejectionReason)
}

func (r *repo) FindUploadableOnboardingDocument(ctx context.Context, userID, s3Key string) (*models.OnboardingDocumentRow, error) {
	return r.pg.FindUploadableOnboardingDocument(ctx, userID, s3Key)
}

func (r *repo) MarkOnboardingDocumentUploadedByS3Key(ctx context.Context, userID, s3Key string) (bool, error) {
	return r.pg.MarkOnboardingDocumentUploadedByS3Key(ctx, userID, s3Key)
}

func (r *repo) SetUserOnboardingComplete(ctx context.Context, userID string, isComplete bool) error {
	return r.pg.SetUserOnboardingComplete(ctx, userID, isComplete)
}
