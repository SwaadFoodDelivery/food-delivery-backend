DROP INDEX IF EXISTS idx_onboardings_user_status;
DROP INDEX IF EXISTS idx_onboarding_documents_s3_key;
DROP INDEX IF EXISTS uq_onboarding_documents_onboarding_doc_type;

ALTER TABLE IF EXISTS onboarding_documents
    DROP CONSTRAINT IF EXISTS chk_onboarding_documents_upload_status;

ALTER TABLE IF EXISTS onboardings
    DROP CONSTRAINT IF EXISTS chk_onboardings_status;

ALTER TABLE IF EXISTS onboardings
    DROP COLUMN IF EXISTS rejection_reason;

DELETE FROM document_type_definitions
WHERE document_type IN (
    'identity_proof',
    'fssai_license',
    'gstin',
    'pan_card',
    'employee_id',
    'authorization_letter',
    'driving_license',
    'vehicle_registration',
    'vehicle_insurance'
);
