package business

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
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
	ListOnboardingReviews(ctx context.Context, status string) ([]models.OnboardingReviewItem, *models.ServiceError)
	ReviewOnboarding(ctx context.Context, in models.ReviewOnboardingInput) (*models.ReviewOnboardingOutput, *models.ServiceError)
}

func (s *Service) ListOnboardingReviews(ctx context.Context, status string) ([]models.OnboardingReviewItem, *models.ServiceError) {
	status = strings.ToLower(strings.TrimSpace(status))
	if status != "" && status != constants.OnboardingStatusPendingVerification && status != constants.OnboardingStatusApproved && status != constants.OnboardingStatusRejected {
		return nil, badRequest(apperrors.CodeValidation, "status must be pending_verification, approved, or rejected")
	}
	items, err := s.repo.ListOnboardingReviews(ctx, status)
	if err != nil {
		return nil, internalErr("failed to load onboarding reviews")
	}
	return items, nil
}

func (s *Service) ReviewOnboarding(ctx context.Context, in models.ReviewOnboardingInput) (*models.ReviewOnboardingOutput, *models.ServiceError) {
	actorID := strings.TrimSpace(in.ActorID)
	onboardingID := strings.TrimSpace(in.OnboardingID)
	status := strings.ToLower(strings.TrimSpace(in.Status))
	if actorID == "" || onboardingID == "" {
		return nil, badRequest(apperrors.CodeValidation, "actor and onboarding IDs are required")
	}
	if status != constants.OnboardingStatusApproved && status != constants.OnboardingStatusRejected {
		return nil, badRequest(apperrors.CodeValidation, "status must be approved or rejected")
	}
	reason := strings.TrimSpace(in.RejectionReason)
	if status == constants.OnboardingStatusRejected && len(reason) < 3 {
		return nil, badRequest(apperrors.CodeValidation, "rejection_reason is required when rejecting onboarding")
	}
	item, err := s.repo.ReviewOnboarding(ctx, repository.ReviewOnboardingInput{ActorID: actorID, OnboardingID: onboardingID, Status: status, RejectionReason: reason})
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, &models.ServiceError{StatusCode: http.StatusNotFound, Code: apperrors.CodeOnboardingNotFound, Message: "onboarding not found", Details: []string{}}
		}
		if errors.Is(err, repository.ErrOnboardingAlreadyReviewed) {
			return nil, badRequest(apperrors.CodeValidation, "onboarding has already been reviewed")
		}
		if errors.Is(err, repository.ErrOnboardingStateConflict) {
			return nil, &models.ServiceError{StatusCode: http.StatusConflict, Code: apperrors.CodeOnboardingUploadsIncomplete, Message: "onboarding documents are incomplete", Details: []string{}}
		}
		return nil, internalErr("failed to review onboarding")
	}
	message := "Onboarding approved"
	if status == constants.OnboardingStatusRejected {
		message = "Onboarding rejected with feedback"
	}
	return &models.ReviewOnboardingOutput{OnboardingID: item.OnboardingID, Status: item.Status, Message: message}, nil
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
	if onboarding.Status != constants.OnboardingStatusDraft {
		return &models.InitOnboardingOutput{OnboardingID: onboarding.OnboardingID, Status: onboarding.Status, Role: onboarding.Role, RejectionReason: onboarding.RejectionReason.String, Documents: []models.OnboardingDocumentUpload{}}, nil
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
	if onboarding.Status != constants.OnboardingStatusDraft {
		return nil, badRequest(apperrors.CodeValidation, "move rejected onboarding to draft before submitting")
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
			ExpectedStatus:  constants.OnboardingStatusDraft,
			OnboardingID:    onboarding.OnboardingID,
			Status:          constants.OnboardingStatusPendingVerification,
			RejectionReason: nil,
		}); txErr != nil {
			return txErr
		}
		// Only a successful review grants approval; submission stays pending.
		if txErr := tx.SetUserOnboardingComplete(ctx, userID, false); txErr != nil {
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
		if errors.Is(err, repository.ErrOnboardingStateConflict) {
			return nil, &models.ServiceError{StatusCode: http.StatusConflict, Code: apperrors.CodeValidation, Message: "onboarding state changed or documents are incomplete; reload and retry", Details: []string{}}
		}
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
			ExpectedStatus:  constants.OnboardingStatusRejected,
			OnboardingID:    onboarding.OnboardingID,
			Status:          constants.OnboardingStatusDraft,
			RejectionReason: nil,
		}); txErr != nil {
			return txErr
		}
		// Reopening a rejected application does not confer approval.
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
		if errors.Is(err, repository.ErrOnboardingStateConflict) {
			return nil, &models.ServiceError{StatusCode: http.StatusConflict, Code: apperrors.CodeValidation, Message: "onboarding state changed; reload and retry", Details: []string{}}
		}
		return nil, internalErr("failed to resubmit onboarding")
	}

	return &models.ResubmitOnboardingOutput{
		OnboardingID: onboarding.OnboardingID,
		Status:       constants.OnboardingStatusDraft,
		Message:      "Onboarding moved to draft. Upload missing documents and submit again",
	}, nil
}

func (s *Service) MarkDocumentUploaded(ctx context.Context, in models.MarkDocumentUploadedInput) *models.ServiceError {
	userID := strings.TrimSpace(in.UserID)
	if userID == "" {
		return &models.ServiceError{StatusCode: http.StatusUnauthorized, Code: apperrors.CodeUnauthorized, Message: "authentication required", Details: []string{}}
	}
	s3Key, details := validations.ValidateS3Key(in.S3Key)
	if len(details) > 0 {
		return badRequest(apperrors.CodeValidation, details[0])
	}
	// Check database ownership before accessing storage: knowing another
	// applicant's object key must reveal neither its existence nor its state.
	if _, err := s.repo.FindUploadableOnboardingDocument(ctx, userID, s3Key); err != nil {
		if repository.IsNotFound(err) {
			return &models.ServiceError{StatusCode: http.StatusNotFound, Code: apperrors.CodeOnboardingDocumentNotFound, Message: "editable document not found", Details: []string{}}
		}
		return internalErr("failed to load document")
	}
	bucket, svcErr := s.validateOnboardingStorage()
	if svcErr != nil {
		return svcErr
	}
	exists, err := s.storageProvider.ObjectExists(ctx, bucket, s3Key)
	if err != nil {
		return internalErr("failed to verify uploaded object")
	}
	if !exists {
		return &models.ServiceError{StatusCode: http.StatusPreconditionFailed, Code: apperrors.CodeOnboardingUploadsIncomplete, Message: "upload the document before confirming it", Details: []string{}}
	}
	updated, err := s.repo.MarkOnboardingDocumentUploadedByS3Key(ctx, userID, s3Key)
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
		if created.Status != constants.OnboardingStatusDraft {
			return nil
		}
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
