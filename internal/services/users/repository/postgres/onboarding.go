package postgres

import (
	"context"
	"database/sql"

	"food-delivery-backend/internal/constants"
	"food-delivery-backend/internal/services/users/models"

	"github.com/jmoiron/sqlx"
)

func (s *Store) ListRequiredDocumentTypes(ctx context.Context, role, country string) ([]models.DocumentTypeDefinitionRow, error) {
	rows := make([]models.DocumentTypeDefinitionRow, 0)
	err := sqlx.SelectContext(ctx, s.accessor.Queryer(), &rows, `
		SELECT document_type, applicable_role::text AS applicable_role, applicable_country, is_required, created_at
		FROM document_type_definitions
		WHERE applicable_role = $1::user_role
		  AND applicable_country = $2
		  AND is_required = TRUE
		ORDER BY document_type ASC
	`, role, country)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Store) CreateOnboarding(ctx context.Context, userID, role, status string) (*models.OnboardingRow, error) {
	var row models.OnboardingRow
	err := sqlx.GetContext(ctx, s.accessor.Queryer(), &row, `
		INSERT INTO onboardings (user_id, role, status, created_at, updated_at)
		VALUES ($1::uuid, $2::user_role, $3, NOW(), NOW())
		RETURNING onboarding_id::text, user_id::text, role::text, status, COALESCE(rejection_reason, '') AS rejection_reason, created_at, updated_at
	`, userID, role, status)
	if err != nil {
		return nil, err
	}
	if row.RejectionReason.String == "" {
		row.RejectionReason = sql.NullString{Valid: false}
	}
	return &row, nil
}

func (s *Store) CreateOnboardingDocument(ctx context.Context, onboardingID, documentType, s3Key, uploadStatus string) (*models.OnboardingDocumentRow, error) {
	var row models.OnboardingDocumentRow
	err := sqlx.GetContext(ctx, s.accessor.Queryer(), &row, `
		INSERT INTO onboarding_documents (onboarding_id, document_type, s3_key, upload_status, created_at, updated_at)
		VALUES ($1::uuid, $2, NULLIF($3, ''), $4, NOW(), NOW())
		RETURNING document_id::text, onboarding_id::text, document_type, s3_key, upload_status, created_at, updated_at
	`, onboardingID, documentType, s3Key, uploadStatus)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *Store) FindOnboardingByIDAndUser(ctx context.Context, onboardingID, userID string) (*models.OnboardingRow, error) {
	var row models.OnboardingRow
	err := sqlx.GetContext(ctx, s.accessor.Queryer(), &row, `
		SELECT onboarding_id::text, user_id::text, role::text, status, rejection_reason, created_at, updated_at
		FROM onboardings
		WHERE onboarding_id = $1::uuid AND user_id = $2::uuid
		LIMIT 1
	`, onboardingID, userID)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &row, nil
}

func (s *Store) CountPendingOnboardingDocuments(ctx context.Context, onboardingID string) (int, error) {
	var count int
	err := sqlx.GetContext(ctx, s.accessor.Queryer(), &count, `
		SELECT COUNT(*)
		FROM onboarding_documents
		WHERE onboarding_id = $1::uuid
		  AND upload_status <> $2
	`, onboardingID, constants.OnboardingUploadStatusUploaded)
	return count, err
}

func (s *Store) UpdateOnboardingStatus(ctx context.Context, onboardingID, status string, rejectionReason *string) error {
	var reason any
	if rejectionReason != nil {
		reason = *rejectionReason
	}
	_, err := s.accessor.Execer().ExecContext(ctx, `
		UPDATE onboardings
		SET status = $2,
		    rejection_reason = NULLIF($3::text, ''),
		    updated_at = NOW()
		WHERE onboarding_id = $1::uuid
	`, onboardingID, status, reason)
	return err
}

func (s *Store) MarkOnboardingDocumentUploadedByS3Key(ctx context.Context, s3Key string) (bool, error) {
	res, err := s.accessor.Execer().ExecContext(ctx, `
		UPDATE onboarding_documents
		SET upload_status = $2, updated_at = NOW()
		WHERE s3_key = $1
		  AND upload_status <> $2
	`, s3Key, constants.OnboardingUploadStatusUploaded)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (s *Store) SetUserOnboardingComplete(ctx context.Context, userID string, isComplete bool) error {
	_, err := s.accessor.Execer().ExecContext(ctx, `
		UPDATE users
		SET onboarding_complete = $2,
		    updated_at = NOW()
		WHERE user_id = $1::uuid
	`, userID, isComplete)
	return err
}
