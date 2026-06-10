package constants

const (
	OnboardingStatusDraft               = "draft"
	OnboardingStatusPendingVerification = "pending_verification"
	OnboardingStatusApproved            = "approved"
	OnboardingStatusRejected            = "rejected"
)

const (
	OnboardingUploadStatusPending  = "pending"
	OnboardingUploadStatusUploaded = "uploaded"
)

const (
	AuditActionOnboardingInit     = "onboarding_init"
	AuditActionOnboardingSubmit   = "onboarding_submit"
	AuditActionOnboardingResubmit = "onboarding_resubmit"
	EntityTypeOnboardings         = "onboardings"
)
