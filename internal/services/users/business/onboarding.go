package business

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"food-delivery-backend/internal/services/common/storage"
	"food-delivery-backend/internal/services/users/models"
	"food-delivery-backend/internal/services/users/repository/repository"
)

const (
	onboardingStatusDraft               = "draft"
	onboardingStatusPendingVerification = "pending_verification"
	onboardingStatusApproved            = "approved"
	onboardingStatusRejected            = "rejected"
)

type OnboardingService interface {
	InitOnboarding(ctx context.Context, in models.InitOnboardingInput) (*models.InitOnboardingOutput, *models.ServiceError)
	SubmitOnboarding(ctx context.Context, in models.SubmitOnboardingInput) (*models.SubmitOnboardingOutput, *models.ServiceError)
	ResubmitOnboarding(ctx context.Context, in models.ResubmitOnboardingInput) (*models.ResubmitOnboardingOutput, *models.ServiceError)
	MarkDocumentUploaded(ctx context.Context, in models.MarkDocumentUploadedInput) *models.ServiceError
}

func (s *Service) InitOnboarding(ctx context.Context, in models.InitOnboardingInput) (*models.InitOnboardingOutput, *models.ServiceError) {
	if strings.TrimSpace(in.UserID) == "" {
		return nil, badRequest("VALIDATION_ERROR", "user_id is required")
	}
	role := strings.TrimSpace(in.Role)
	if !validRole(role) {
		return nil, badRequest("VALIDATION_ERROR", "invalid role")
	}
	country := strings.ToUpper(strings.TrimSpace(in.Country))
	if country == "" {
		country = "IN"
	}
	if len(country) != 2 {
		return nil, badRequest("VALIDATION_ERROR", "country must be ISO-2 code")
	}
	if s.storageProvider == nil {
		return nil, internalErr("storage provider is not configured")
	}
	bucket := strings.TrimSpace(s.cfg.S3.Bucket)
	if bucket == "" {
		return nil, internalErr("S3_BUCKET is not configured")
	}

	docDefs, err := s.repo.ListRequiredDocumentTypes(ctx, role, country)
	if err != nil {
		return nil, internalErr("failed to load required document types")
	}
	if len(docDefs) == 0 {
		return nil, badRequest("ONBOARDING_CONFIG_MISSING", "no required document types configured for this role")
	}

	type pendingDoc struct {
		row models.OnboardingDocumentRow
	}
	pendingDocs := make([]pendingDoc, 0, len(docDefs))

	var onboarding *models.OnboardingRow
	err = s.repo.WithTx(ctx, func(tx repository.Repository) error {
		created, txErr := tx.CreateOnboarding(ctx, repository.CreateOnboardingInput{
			UserID:  in.UserID,
			Role:    role,
			Status:  onboardingStatusDraft,
			Country: country,
		})
		if txErr != nil {
			return txErr
		}
		onboarding = created

		for _, def := range docDefs {
			s3Key := fmt.Sprintf("users/%s/onboarding/%s/%s", in.UserID, created.OnboardingID, def.DocumentType)
			doc, createErr := tx.CreateOnboardingDocument(ctx, repository.CreateOnboardingDocumentInput{
				OnboardingID: created.OnboardingID,
				DocumentType: def.DocumentType,
				S3Key:        s3Key,
				UploadStatus: "pending",
			})
			if createErr != nil {
				return createErr
			}
			pendingDocs = append(pendingDocs, pendingDoc{row: *doc})
		}

		return tx.InsertAuditLog(ctx, repository.AuditLogInput{
			ActorID:    in.UserID,
			ActorRole:  role,
			Action:     "onboarding_init",
			EntityType: "onboardings",
			EntityID:   onboarding.OnboardingID,
		})
	})
	if err != nil {
		return nil, internalErr("failed to initialize onboarding")
	}

	outDocs := make([]models.OnboardingDocumentUpload, 0, len(pendingDocs))
	for _, doc := range pendingDocs {
		presigned, preErr := s.storageProvider.PresignPut(ctx, storage.PresignPutInput{
			Bucket:      bucket,
			Key:         doc.row.S3Key.String,
			ContentType: "application/octet-stream",
			ExpiresIn:   time.Duration(s.cfg.S3.PresignTTLSeconds) * time.Second,
		})
		if preErr != nil {
			return nil, internalErr("failed to generate upload url")
		}
		outDocs = append(outDocs, models.OnboardingDocumentUpload{
			DocumentID:   doc.row.DocumentID,
			DocumentType: doc.row.DocumentType,
			S3Key:        doc.row.S3Key.String,
			UploadURL:    presigned.URL,
			Method:       presigned.Method,
			ExpiresAt:    presigned.ExpiresAt.UTC().Format(time.RFC3339),
			UploadStatus: doc.row.UploadStatus,
		})
	}

	return &models.InitOnboardingOutput{
		OnboardingID: onboarding.OnboardingID,
		Status:       onboarding.Status,
		Role:         onboarding.Role,
		Documents:    outDocs,
	}, nil
}

func (s *Service) SubmitOnboarding(ctx context.Context, in models.SubmitOnboardingInput) (*models.SubmitOnboardingOutput, *models.ServiceError) {
	if strings.TrimSpace(in.UserID) == "" || strings.TrimSpace(in.OnboardingID) == "" {
		return nil, badRequest("VALIDATION_ERROR", "user_id and onboarding_id are required")
	}

	onboarding, err := s.repo.FindOnboardingByIDAndUser(ctx, in.OnboardingID, in.UserID)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, &models.ServiceError{StatusCode: http.StatusNotFound, Code: "ONBOARDING_NOT_FOUND", Message: "onboarding not found", Details: []string{}}
		}
		return nil, internalErr("failed to load onboarding")
	}

	if onboarding.Status == onboardingStatusApproved {
		return nil, badRequest("ONBOARDING_ALREADY_APPROVED", "onboarding is already approved")
	}
	if onboarding.Status == onboardingStatusPendingVerification {
		return nil, badRequest("ONBOARDING_ALREADY_SUBMITTED", "onboarding already submitted for verification")
	}

	incompleteCount, err := s.repo.CountPendingOnboardingDocuments(ctx, onboarding.OnboardingID)
	if err != nil {
		return nil, internalErr("failed to validate onboarding documents")
	}
	if incompleteCount > 0 {
		return nil, &models.ServiceError{StatusCode: http.StatusPreconditionFailed, Code: "UPLOADS_INCOMPLETE", Message: "all required documents must be uploaded before submit", Details: []string{}}
	}

	err = s.repo.WithTx(ctx, func(tx repository.Repository) error {
		if txErr := tx.UpdateOnboardingStatus(ctx, repository.UpdateOnboardingStatusInput{
			OnboardingID:    onboarding.OnboardingID,
			Status:          onboardingStatusPendingVerification,
			RejectionReason: nil,
		}); txErr != nil {
			return txErr
		}
		return tx.InsertAuditLog(ctx, repository.AuditLogInput{
			ActorID:    in.UserID,
			ActorRole:  onboarding.Role,
			Action:     "onboarding_submit",
			EntityType: "onboardings",
			EntityID:   onboarding.OnboardingID,
		})
	})
	if err != nil {
		return nil, internalErr("failed to submit onboarding")
	}

	return &models.SubmitOnboardingOutput{
		OnboardingID: onboarding.OnboardingID,
		Status:       onboardingStatusPendingVerification,
		Message:      "Onboarding submitted and pending verification",
	}, nil
}

func (s *Service) ResubmitOnboarding(ctx context.Context, in models.ResubmitOnboardingInput) (*models.ResubmitOnboardingOutput, *models.ServiceError) {
	if strings.TrimSpace(in.UserID) == "" || strings.TrimSpace(in.OnboardingID) == "" {
		return nil, badRequest("VALIDATION_ERROR", "user_id and onboarding_id are required")
	}

	onboarding, err := s.repo.FindOnboardingByIDAndUser(ctx, in.OnboardingID, in.UserID)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, &models.ServiceError{StatusCode: http.StatusNotFound, Code: "ONBOARDING_NOT_FOUND", Message: "onboarding not found", Details: []string{}}
		}
		return nil, internalErr("failed to load onboarding")
	}
	if onboarding.Status != onboardingStatusRejected {
		return nil, badRequest("ONBOARDING_NOT_REJECTED", "only rejected onboarding can be resubmitted")
	}

	err = s.repo.WithTx(ctx, func(tx repository.Repository) error {
		if txErr := tx.UpdateOnboardingStatus(ctx, repository.UpdateOnboardingStatusInput{
			OnboardingID:    onboarding.OnboardingID,
			Status:          onboardingStatusDraft,
			RejectionReason: nil,
		}); txErr != nil {
			return txErr
		}
		return tx.InsertAuditLog(ctx, repository.AuditLogInput{
			ActorID:    in.UserID,
			ActorRole:  onboarding.Role,
			Action:     "onboarding_resubmit",
			EntityType: "onboardings",
			EntityID:   onboarding.OnboardingID,
		})
	})
	if err != nil {
		return nil, internalErr("failed to resubmit onboarding")
	}

	return &models.ResubmitOnboardingOutput{
		OnboardingID: onboarding.OnboardingID,
		Status:       onboardingStatusDraft,
		Message:      "Onboarding moved to draft. Upload missing documents and submit again",
	}, nil
}

func (s *Service) MarkDocumentUploaded(ctx context.Context, in models.MarkDocumentUploadedInput) *models.ServiceError {
	if strings.TrimSpace(in.S3Key) == "" {
		return badRequest("VALIDATION_ERROR", "s3_key is required")
	}
	updated, err := s.repo.MarkOnboardingDocumentUploadedByS3Key(ctx, strings.TrimSpace(in.S3Key))
	if err != nil {
		return internalErr("failed to update document upload status")
	}
	if !updated {
		return &models.ServiceError{StatusCode: http.StatusNotFound, Code: "DOCUMENT_NOT_FOUND", Message: "document not found", Details: []string{}}
	}
	return nil
}

func validRole(role string) bool {
	switch strings.TrimSpace(role) {
	case "client", "restaurant_owner", "restaurant_manager", "driver":
		return true
	default:
		return false
	}
}
