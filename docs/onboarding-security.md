# Onboarding approval and upload confirmation

Submission is not approval. Draft → pending_verification leaves
`users.onboarding_complete=false`; only the operations approve transaction
sets it true. Rejection keeps it false. Rejected → draft uses the resubmit
endpoint, followed by init to resume the same application and refresh uploads.
Init on a pending/rejected application returns its state and feedback without
issuing new upload URLs. Compare-and-set transitions reject concurrent or stale
submissions. Legacy pending applications cannot override a newer application.

`POST /api/v1/onboarding/documents/uploaded` now requires a Bearer session.
The caller must own an editable draft document. The server verifies a nonempty
object with storage HEAD before marking it uploaded. Unknown/foreign/non-draft
documents return 404, absent/empty uploads 412, and storage failures fail closed.
The storage request has a three-second timeout and cannot follow redirects.
An authenticated retry for an uploaded draft document is idempotent.

Owner and driver operational APIs reload the current account and require active,
role-matched, approved access. Migration 28 repairs legacy premature grants for
the latest unapproved application; seed accounts without applications and
approved applications are unchanged. Rollback intentionally does not restore
unreviewed privileges. Deploy the matching frontend pending-review changes with
this backend; the old unauthenticated callback client is incompatible.

## Verification

- `go test -race ./...` and `go vet ./...`.
- `ONBOARDING_TEST_DATABASE_URL=postgres://.../swaad_onboarding_test?sslmode=disable go test -race ./internal/services/users/business -run TestOnboardingIntegration -count=1 -v`.
- `MIGRATION_TEST_DATABASE_URL=postgres://.../swaad_onboarding_test?sslmode=disable go test -race ./infra/postgres -count=1 -v`.
- Actual signed MinIO PUT/HEAD test: see `internal/services/common/storage/README.md`.

Use disposable databases. The migration smoke seeds fictional restaurants;
onboarding fixtures own generated users and clean them up. CI runs the database
suites serially against a fresh PostGIS service. Browser acceptance is tracked
separately in the frontend repository. These are demo controls, not malware
scanning, document authenticity verification, or immutable object-version review.

Agent review is engineering evidence, not a fabricated human GitHub approval.
