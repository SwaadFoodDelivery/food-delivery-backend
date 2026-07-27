package business

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"food-delivery-backend/internal/constants"
	apperrors "food-delivery-backend/internal/errors"
	"food-delivery-backend/internal/services/common/storage"
	"food-delivery-backend/internal/services/users/models"
	"food-delivery-backend/internal/services/users/repository/repository"
	"food-delivery-backend/internal/services/users/validations"
)

type OnboardingService interface {
	InitOnboarding(ctx context.Context, in models.InitOnboardingInput) (*models.InitOnboardingOutput, *models.ServiceError)
	SubmitOnboarding(ctx context.Context, in models.SubmitOnboardingInput) (*models.SubmitOnboardingOutput, *models.ServiceError)
	ResubmitOnboarding(ctx context.Context, in models.ResubmitOnboardingInput) (*models.ResubmitOnboardingOutput, *models.ServiceError)
	MarkDocumentUploaded(ctx context.Context, in models.MarkDocumentUploadedInput) *models.ServiceError
}

func (s *Service) InitOnboarding(ctx context.Context, in models.InitOnboardingInput) (*models.InitOnboardingOutput, *models.ServiceError) {
	input, svcErr := s.validateInitOnboarding(in)
	if svcErr != nil {
		return nil, svcErr
	}
	bucket, svcErr := s.validateOnboardingStorage()
	if svcErr != nil {
		return nil, svcErr
	}
	docDefs, svcErr := s.loadRequiredDocs(ctx, input.Role, input.Country)
	if svcErr != nil {
		return nil, svcErr
	}
	onboarding, pendingDocs, svcErr := s.createOnboardingDraft(ctx, input, docDefs)
	if svcErr != nil {
		return nil, svcErr
	}
	outDocs, svcErr := s.buildPresignedUploads(ctx, bucket, pendingDocs)
	if svcErr != nil {
		return nil, svcErr
	}
	return &models.InitOnboardingOutput{OnboardingID: onboarding.OnboardingID, Status: onboarding.Status, Role: onboarding.Role, Documents: outDocs}, nil
}

func (s *Service) SubmitOnboarding(ctx context.Context, in models.SubmitOnboardingInput) (*models.SubmitOnboardingOutput, *models.ServiceError) {
	userID, onboardingID, details := validations.ValidateOnboardingIDs(in.UserID, in.OnboardingID)
	if len(details) > 0 {
		return nil, badRequest(apperrors.CodeValidation, details[0])
	}
	onboarding, err := s.repo.FindOnboardingByIDAndUser(ctx, onboardingID, userID)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, &models.ServiceError{StatusCode: http.StatusNotFound, Code: apperrors.CodeOnboardingNotFound, Message: "onboarding not found", Details: []string{}}
		}
		return nil, internalErr("failed to load onboarding")
	}

	if onboarding.Status == constants.OnboardingStatusApproved {
		return nil, badRequest(apperrors.CodeOnboardingAlreadyApproved, "onboarding is already approved")
	}
	if onboarding.Status == constants.OnboardingStatusPendingVerification {
		return nil, badRequest(apperrors.CodeOnboardingAlreadySubmitted, "onboarding already submitted for verification")
	}

	incompleteCount, err := s.repo.CountPendingOnboardingDocuments(ctx, onboarding.OnboardingID)
	if err != nil {
		return nil, internalErr("failed to validate onboarding documents")
	}
	if incompleteCount > 0 {
		return nil, &models.ServiceError{StatusCode: http.StatusPreconditionFailed, Code: apperrors.CodeOnboardingUploadsIncomplete, Message: "all required documents must be uploaded before submit", Details: []string{}}
	}

	err = s.repo.WithTx(ctx, func(tx repository.Repository) error {
		if txErr := tx.UpdateOnboardingStatus(ctx, repository.UpdateOnboardingStatusInput{
			OnboardingID:    onboarding.OnboardingID,
			Status:          constants.OnboardingStatusPendingVerification,
			RejectionReason: nil,
		}); txErr != nil {
			return txErr
		}
		// Submission is the last user-facing step today (there is no admin
		// approve/reject endpoint yet), so this is where the account leaves
		// "first-time" status. RequireOnboardingAccess reads this same flag;
		// without setting it here a user could re-init and re-submit forever.
		// NOTE: a future reject workflow must set this back to false (see
		// ResubmitOnboarding below), or a rejected user would be locked out of
		// resubmitting by this same gate.
		if txErr := tx.SetUserOnboardingComplete(ctx, userID, true); txErr != nil {
			return txErr
		}
		return tx.InsertAuditLog(ctx, repository.AuditLogInput{
			ActorID:    userID,
			ActorRole:  onboarding.Role,
			Action:     constants.AuditActionOnboardingSubmit,
			EntityType: constants.EntityTypeOnboardings,
			EntityID:   onboarding.OnboardingID,
		})
	})
	if err != nil {
		return nil, internalErr("failed to submit onboarding")
	}

	return &models.SubmitOnboardingOutput{
		OnboardingID: onboarding.OnboardingID,
		Status:       constants.OnboardingStatusPendingVerification,
		Message:      "Onboarding submitted and pending verification",
	}, nil
}

func (s *Service) ResubmitOnboarding(ctx context.Context, in models.ResubmitOnboardingInput) (*models.ResubmitOnboardingOutput, *models.ServiceError) {
	userID, onboardingID, details := validations.ValidateOnboardingIDs(in.UserID, in.OnboardingID)
	if len(details) > 0 {
		return nil, badRequest(apperrors.CodeValidation, details[0])
	}
	onboarding, err := s.repo.FindOnboardingByIDAndUser(ctx, onboardingID, userID)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, &models.ServiceError{StatusCode: http.StatusNotFound, Code: apperrors.CodeOnboardingNotFound, Message: "onboarding not found", Details: []string{}}
		}
		return nil, internalErr("failed to load onboarding")
	}
	if onboarding.Status != constants.OnboardingStatusRejected {
		return nil, badRequest(apperrors.CodeOnboardingNotRejected, "only rejected onboarding can be resubmitted")
	}

	err = s.repo.WithTx(ctx, func(tx repository.Repository) error {
		if txErr := tx.UpdateOnboardingStatus(ctx, repository.UpdateOnboardingStatusInput{
			OnboardingID:    onboarding.OnboardingID,
			Status:          constants.OnboardingStatusDraft,
			RejectionReason: nil,
		}); txErr != nil {
			return txErr
		}
		// Mirrors the flag flip in SubmitOnboarding: a rejected onboarding needs
		// the user back in the flow, so RequireOnboardingAccess must let them
		// through again.
		if txErr := tx.SetUserOnboardingComplete(ctx, userID, false); txErr != nil {
			return txErr
		}
		return tx.InsertAuditLog(ctx, repository.AuditLogInput{
			ActorID:    userID,
			ActorRole:  onboarding.Role,
			Action:     constants.AuditActionOnboardingResubmit,
			EntityType: constants.EntityTypeOnboardings,
			EntityID:   onboarding.OnboardingID,
		})
	})
	if err != nil {
		return nil, internalErr("failed to resubmit onboarding")
	}

	return &models.ResubmitOnboardingOutput{
		OnboardingID: onboarding.OnboardingID,
		Status:       constants.OnboardingStatusDraft,
		Message:      "Onboarding moved to draft. Upload missing documents and submit again",
	}, nil
}

func (s *Service) MarkDocumentUploaded(ctx context.Context, in models.MarkDocumentUploadedInput) *models.ServiceError {
	s3Key, details := validations.ValidateS3Key(in.S3Key)
	if len(details) > 0 {
		return badRequest(apperrors.CodeValidation, details[0])
	}
	updated, err := s.repo.MarkOnboardingDocumentUploadedByS3Key(ctx, s3Key)
	if err != nil {
		return internalErr("failed to update document upload status")
	}
	if !updated {
		return &models.ServiceError{StatusCode: http.StatusNotFound, Code: apperrors.CodeOnboardingDocumentNotFound, Message: "document not found", Details: []string{}}
	}
	return nil
}

func (s *Service) validateInitOnboarding(in models.InitOnboardingInput) (models.InitOnboardingInput, *models.ServiceError) {
	out, details := validations.ValidateInitOnboardingInput(in)
	if len(details) > 0 {
		return models.InitOnboardingInput{}, badRequest(apperrors.CodeValidation, details[0])
	}
	return out, nil
}

func (s *Service) validateOnboardingStorage() (string, *models.ServiceError) {
	if s.storageProvider == nil {
		return "", internalErr("storage provider is not configured")
	}
	if s.cfg == nil {
		return "", internalErr("config is not configured")
	}
	bucket := s.cfg.S3BucketFor(constants.S3BucketPurposeOnboarding)
	if bucket == "" {
		return "", internalErr("S3_ONBOARDING_BUCKET or S3_DEFAULT_BUCKET is not configured")
	}
	return bucket, nil
}

func (s *Service) loadRequiredDocs(ctx context.Context, role, country string) ([]models.DocumentTypeDefinitionRow, *models.ServiceError) {
	docDefs, err := s.repo.ListRequiredDocumentTypes(ctx, role, country)
	if err != nil {
		return nil, internalErr("failed to load required document types")
	}
	if len(docDefs) == 0 {
		return nil, badRequest(apperrors.CodeOnboardingConfigMissing, "no required document types configured for this role")
	}
	return docDefs, nil
}

func (s *Service) createOnboardingDraft(ctx context.Context, in models.InitOnboardingInput, docDefs []models.DocumentTypeDefinitionRow) (*models.OnboardingRow, []models.OnboardingDocumentRow, *models.ServiceError) {
	pendingDocs := make([]models.OnboardingDocumentRow, 0, len(docDefs))
	var onboarding *models.OnboardingRow
	err := s.repo.WithTx(ctx, func(tx repository.Repository) error {
		created, txErr := tx.CreateOnboarding(ctx, repository.CreateOnboardingInput{UserID: in.UserID, Role: in.Role, Status: constants.OnboardingStatusDraft, Country: in.Country})
		if txErr != nil {
			return txErr
		}
		onboarding = created
		for _, def := range docDefs {
			s3Key := fmt.Sprintf("users/%s/onboarding/%s/%s", in.UserID, created.OnboardingID, def.DocumentType)
			doc, createErr := tx.CreateOnboardingDocument(ctx, repository.CreateOnboardingDocumentInput{OnboardingID: created.OnboardingID, DocumentType: def.DocumentType, S3Key: s3Key, UploadStatus: constants.OnboardingUploadStatusPending})
			if createErr != nil {
				return createErr
			}
			pendingDocs = append(pendingDocs, *doc)
		}
		return tx.InsertAuditLog(ctx, repository.AuditLogInput{ActorID: in.UserID, ActorRole: in.Role, Action: constants.AuditActionOnboardingInit, EntityType: constants.EntityTypeOnboardings, EntityID: onboarding.OnboardingID})
	})
	if err != nil {
		return nil, nil, internalErr("failed to initialize onboarding")
	}
	return onboarding, pendingDocs, nil
}

func (s *Service) buildPresignedUploads(ctx context.Context, bucket string, docs []models.OnboardingDocumentRow) ([]models.OnboardingDocumentUpload, *models.ServiceError) {
	out := make([]models.OnboardingDocumentUpload, 0, len(docs))
	for _, doc := range docs {
		presigned, err := s.storageProvider.PresignPut(ctx, storage.PresignPutInput{Bucket: bucket, Key: doc.S3Key.String, ContentType: "application/octet-stream", ExpiresIn: time.Duration(s.cfg.S3.PresignTTLSeconds) * time.Second})
		if err != nil {
			return nil, internalErr("failed to generate upload url")
		}
		out = append(out, models.OnboardingDocumentUpload{DocumentID: doc.DocumentID, DocumentType: doc.DocumentType, S3Key: doc.S3Key.String, UploadURL: presigned.URL, Method: presigned.Method, ExpiresAt: presigned.ExpiresAt.UTC().Format(time.RFC3339), UploadStatus: doc.UploadStatus})
	}
	return out, nil
}
