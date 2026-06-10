ALTER TABLE IF EXISTS onboardings
    ADD COLUMN IF NOT EXISTS rejection_reason TEXT;

DO $$
BEGIN
    ALTER TABLE onboardings
        ADD CONSTRAINT chk_onboardings_status
        CHECK (status IN ('draft', 'pending_verification', 'approved', 'rejected'));
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE onboarding_documents
        ADD CONSTRAINT chk_onboarding_documents_upload_status
        CHECK (upload_status IN ('pending', 'uploaded', 'rejected'));
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS uq_onboarding_documents_onboarding_doc_type
    ON onboarding_documents(onboarding_id, document_type);

CREATE INDEX IF NOT EXISTS idx_onboarding_documents_s3_key
    ON onboarding_documents(s3_key)
    WHERE s3_key IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_onboardings_user_status
    ON onboardings(user_id, status);

INSERT INTO document_type_definitions (document_type, applicable_role, applicable_country, is_required)
VALUES
    ('identity_proof', 'client', 'IN', TRUE),
    ('fssai_license', 'restaurant_owner', 'IN', TRUE),
    ('gstin', 'restaurant_owner', 'IN', TRUE),
    ('pan_card', 'restaurant_owner', 'IN', TRUE),
    ('employee_id', 'restaurant_manager', 'IN', TRUE),
    ('authorization_letter', 'restaurant_manager', 'IN', TRUE),
    ('driving_license', 'driver', 'IN', TRUE),
    ('vehicle_registration', 'driver', 'IN', TRUE),
    ('vehicle_insurance', 'driver', 'IN', TRUE)
ON CONFLICT (document_type) DO NOTHING;
