package business_test

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	postgresinfra "food-delivery-backend/infra/postgres"
	"food-delivery-backend/internal/constants"
	"food-delivery-backend/internal/services/common/storage"
	"food-delivery-backend/internal/services/users/business"
	"food-delivery-backend/internal/services/users/models"
	"food-delivery-backend/internal/services/users/repository/repository"
	"food-delivery-backend/pkg/config"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
)

// This suite commits service transactions so competing connections see real
// state. Use only an explicitly disposable database, never the development DB.
// Fixtures own UUID users; deleting them cascades to documents/notifications.
func TestOnboardingIntegration(t *testing.T) {
	dsn := os.Getenv("ONBOARDING_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ONBOARDING_TEST_DATABASE_URL is not set (requires a disposable PostGIS database)")
	}
	u, err := url.Parse(dsn)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		t.Fatal("ONBOARDING_TEST_DATABASE_URL must be a postgres URL for a disposable database")
	}
	name := strings.TrimPrefix(u.Path, "/")
	if !strings.Contains(name, "_test_") && !strings.HasSuffix(name, "_test") && !(name == "food_delivery" && os.Getenv("GITHUB_ACTIONS") == "true") {
		t.Fatal("refusing database without _test_ or _test in its name; food_delivery is allowed only in GitHub Actions")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	db, err := sqlx.ConnectContext(ctx, "postgres", dsn)
	if err != nil {
		t.Fatalf("connect to disposable onboarding database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(16)
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve migration path")
	}
	if err := postgresinfra.RunMigrations(ctx, db.DB, filepath.Join(filepath.Dir(file), "../../../..", "migrations")); err != nil {
		t.Fatalf("migrate disposable database: %v", err)
	}

	t.Run("submit_requires_uploads_and_only_approval_grants_access", func(t *testing.T) {
		f := newOnboardingFixture(t, ctx, db)
		f.init()
		_, svcErr := f.service.SubmitOnboarding(ctx, f.submitInput())
		wantServiceStatus(t, svcErr, http.StatusPreconditionFailed)
		f.assertState(constants.OnboardingStatusDraft, false)
		f.assertAudit(constants.AuditActionOnboardingSubmit, 0)
		f.uploadAll()
		// Reproduce the legacy flag; submission must actively clear it.
		f.exec(`UPDATE users SET onboarding_complete = TRUE WHERE user_id = $1::uuid`, f.userID)
		f.submit()
		f.assertState(constants.OnboardingStatusPendingVerification, false)
		_, svcErr = f.service.SubmitOnboarding(ctx, f.submitInput())
		wantServiceStatus(t, svcErr, http.StatusBadRequest)
		f.assertAudit(constants.AuditActionOnboardingSubmit, 1)
		f.review(constants.OnboardingStatusApproved)
		f.assertState(constants.OnboardingStatusApproved, true)
		_, svcErr = f.service.SubmitOnboarding(ctx, f.submitInput())
		wantServiceStatus(t, svcErr, http.StatusBadRequest)
		f.assertState(constants.OnboardingStatusApproved, true)
		f.assertAudit(constants.AuditActionOnboardingSubmit, 1)
	})

	t.Run("reject_resubmit_submit_keeps_access_disabled", func(t *testing.T) {
		f := newOnboardingFixture(t, ctx, db)
		f.init()
		f.uploadAll()
		f.submit()
		f.exec(`UPDATE users SET onboarding_complete = TRUE WHERE user_id = $1::uuid`, f.userID)
		f.review(constants.OnboardingStatusRejected)
		f.assertState(constants.OnboardingStatusRejected, false)
		_, svcErr := f.service.SubmitOnboarding(ctx, f.submitInput())
		wantServiceStatus(t, svcErr, http.StatusBadRequest)
		f.assertState(constants.OnboardingStatusRejected, false)
		f.assertAudit(constants.AuditActionOnboardingSubmit, 1)
		out, svcErr := f.service.ResubmitOnboarding(ctx, models.ResubmitOnboardingInput{UserID: f.userID, OnboardingID: f.application.OnboardingID})
		if svcErr != nil || out == nil || out.Status != constants.OnboardingStatusDraft {
			t.Fatalf("resubmit: output=%+v error=%+v", out, svcErr)
		}
		f.assertState(constants.OnboardingStatusDraft, false)
		var reason string
		f.get(&reason, `SELECT COALESCE(rejection_reason, '') FROM onboardings WHERE onboarding_id = $1::uuid`, f.application.OnboardingID)
		if reason != "" {
			t.Fatalf("resubmit retained rejection reason %q", reason)
		}
		f.submit()
		f.assertState(constants.OnboardingStatusPendingVerification, false)
		f.assertAudit(constants.AuditActionOnboardingSubmit, 2)
		f.assertAudit(constants.AuditActionOnboardingResubmit, 1)
	})

	for _, status := range []string{constants.OnboardingStatusDraft, constants.OnboardingStatusPendingVerification, constants.OnboardingStatusRejected} {
		t.Run("init_resumes_latest_"+status, func(t *testing.T) {
			f := newOnboardingFixture(t, ctx, db)
			f.init()
			f.uploadAll()
			if status != constants.OnboardingStatusDraft {
				f.submit()
			}
			if status == constants.OnboardingStatusRejected {
				f.review(status)
			}
			// A historical application must not displace the latest one.
			f.exec(`INSERT INTO onboardings (onboarding_id, user_id, role, status, created_at)
				VALUES ($1::uuid, $2::uuid, 'restaurant_owner', 'rejected', NOW() - INTERVAL '1 day')`, uuid.New().String(), f.userID)
			before := f.documents()
			presigns, _ := f.storage.counts()
			out, svcErr := f.service.InitOnboarding(ctx, models.InitOnboardingInput{UserID: f.userID, Role: constants.RoleRestaurantOwner, Country: "IN"})
			if svcErr != nil || out == nil || out.OnboardingID != f.application.OnboardingID || out.Status != status {
				t.Fatalf("resume %s: output=%+v error=%+v", status, out, svcErr)
			}
			if status != constants.OnboardingStatusDraft {
				afterPresigns, _ := f.storage.counts()
				if len(out.Documents) != 0 || afterPresigns != presigns {
					t.Fatalf("%s init issued uploads: documents=%d presigns=%d -> %d", status, len(out.Documents), presigns, afterPresigns)
				}
			}
			if status == constants.OnboardingStatusRejected && out.RejectionReason != "Replace unreadable document" {
				t.Fatalf("resume lost review feedback: %+v", out)
			}
			var applications int
			f.get(&applications, `SELECT COUNT(*) FROM onboardings WHERE user_id = $1::uuid`, f.userID)
			if applications != 2 {
				t.Fatalf("init created another application: count=%d, want 2 including history", applications)
			}
			after := f.documents()
			if fmt.Sprint(before) != fmt.Sprint(after) {
				t.Fatalf("init changed documents: before=%+v after=%+v", before, after)
			}
		})
	}

	t.Run("confirmation_checks_owner_database_and_object_and_is_idempotent", func(t *testing.T) {
		f := newOnboardingFixture(t, ctx, db)
		f.init()
		doc := f.application.Documents[0]
		otherID := f.createUser(constants.RoleRestaurantOwner)
		f.storage.objects[doc.S3Key] = true
		wantServiceStatus(t, f.service.MarkDocumentUploaded(ctx, models.MarkDocumentUploadedInput{UserID: otherID, S3Key: doc.S3Key}), http.StatusNotFound)
		wantServiceStatus(t, f.service.MarkDocumentUploaded(ctx, models.MarkDocumentUploadedInput{S3Key: doc.S3Key}), http.StatusUnauthorized)
		unknownKey := fmt.Sprintf("users/%s/onboarding/%s/unknown", f.userID, f.application.OnboardingID)
		f.storage.objects[unknownKey] = true
		wantServiceStatus(t, f.service.MarkDocumentUploaded(ctx, models.MarkDocumentUploadedInput{UserID: f.userID, S3Key: unknownKey}), http.StatusNotFound)
		_, checks := f.storage.counts()
		if checks != 0 {
			t.Fatalf("storage probed before DB ownership/document check: %d calls", checks)
		}
		f.assertDocument(doc.DocumentID, constants.OnboardingUploadStatusPending)
		delete(f.storage.objects, doc.S3Key)
		input := models.MarkDocumentUploadedInput{UserID: f.userID, S3Key: doc.S3Key}
		wantServiceStatus(t, f.service.MarkDocumentUploaded(ctx, input), http.StatusPreconditionFailed)
		f.assertDocument(doc.DocumentID, constants.OnboardingUploadStatusPending)
		f.storage.existsErr = errors.New("storage unavailable")
		wantServiceStatus(t, f.service.MarkDocumentUploaded(ctx, input), http.StatusInternalServerError)
		f.assertDocument(doc.DocumentID, constants.OnboardingUploadStatusPending)
		f.storage.existsErr = nil
		f.storage.objects[doc.S3Key] = true
		for i := 0; i < 2; i++ {
			if svcErr := f.service.MarkDocumentUploaded(ctx, input); svcErr != nil {
				t.Fatalf("owner confirmation %d: %+v", i+1, svcErr)
			}
			f.assertDocument(doc.DocumentID, constants.OnboardingUploadStatusUploaded)
		}
		_, checks = f.storage.counts()
		if checks != 4 || f.storage.lastBucket != "onboarding-integration" || f.storage.lastKey != doc.S3Key {
			t.Fatalf("object verification calls=%d bucket=%q key=%q", checks, f.storage.lastBucket, f.storage.lastKey)
		}
	})

	for _, status := range []string{constants.OnboardingStatusPendingVerification, constants.OnboardingStatusRejected, constants.OnboardingStatusApproved} {
		t.Run("confirmation_requires_draft_"+status, func(t *testing.T) {
			f := newOnboardingFixture(t, ctx, db)
			f.init()
			f.uploadAll()
			f.submit()
			if status != constants.OnboardingStatusPendingVerification {
				f.review(status)
			}
			_, before := f.storage.counts()
			wantServiceStatus(t, f.service.MarkDocumentUploaded(ctx, models.MarkDocumentUploadedInput{UserID: f.userID, S3Key: f.application.Documents[0].S3Key}), http.StatusNotFound)
			_, after := f.storage.counts()
			if after != before {
				t.Fatalf("non-draft confirmation checked storage: %d -> %d", before, after)
			}
			f.assertState(status, status == constants.OnboardingStatusApproved)
		})
	}

	t.Run("concurrent_submit_commits_one_transition_and_audit", func(t *testing.T) {
		f := newOnboardingFixture(t, ctx, db)
		f.init()
		f.uploadAll()
		lock, err := db.BeginTxx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer lock.Rollback()
		if _, err := lock.ExecContext(ctx, `SELECT onboarding_id FROM onboardings WHERE onboarding_id = $1::uuid FOR UPDATE`, f.application.OnboardingID); err != nil {
			t.Fatal(err)
		}
		const submitters = 8
		raceCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		results := make(chan *models.ServiceError, submitters)
		for i := 0; i < submitters; i++ {
			go func() {
				_, svcErr := f.service.SubmitOnboarding(raceCtx, f.submitInput())
				results <- svcErr
			}()
		}
		// All readers must see draft and reach the blocked CAS, rather than
		// letting a fast winner turn this into sequential validation checks.
		blocked := 0
		for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
			if err := db.GetContext(raceCtx, &blocked, `SELECT COUNT(*) FROM pg_stat_activity
				WHERE datname = current_database() AND pid <> pg_backend_pid()
				AND wait_event_type = 'Lock' AND query LIKE '%UPDATE onboardings%'`); err != nil {
				t.Errorf("observe competing submits: %v", err)
				break
			}
			if blocked == submitters {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if err := lock.Rollback(); err != nil {
			t.Errorf("release onboarding lock: %v", err)
		}
		successes := 0
		for i := 0; i < submitters; i++ {
			svcErr := <-results
			if svcErr == nil {
				successes++
			} else if svcErr.StatusCode != http.StatusConflict {
				t.Errorf("competing submit error=%+v, want CAS conflict", svcErr)
			}
		}
		if blocked != submitters || successes != 1 {
			t.Fatalf("blocked submitters=%d successful submits=%d, want %d and 1", blocked, successes, submitters)
		}
		f.assertState(constants.OnboardingStatusPendingVerification, false)
		f.assertAudit(constants.AuditActionOnboardingSubmit, 1)
	})

	t.Run("stale_cas_cannot_overwrite_approval", func(t *testing.T) {
		f := newOnboardingFixture(t, ctx, db)
		f.init()
		f.uploadAll()
		f.submit()
		f.review(constants.OnboardingStatusApproved)
		for _, stale := range []repository.UpdateOnboardingStatusInput{
			{ExpectedStatus: constants.OnboardingStatusDraft, Status: constants.OnboardingStatusPendingVerification},
			{ExpectedStatus: constants.OnboardingStatusRejected, Status: constants.OnboardingStatusDraft},
		} {
			stale.OnboardingID = f.application.OnboardingID
			err := f.repo.WithTx(ctx, func(tx repository.Repository) error {
				// A failed CAS must also roll back other transaction writes.
				if err := tx.SetUserOnboardingComplete(ctx, f.userID, false); err != nil {
					return err
				}
				return tx.UpdateOnboardingStatus(ctx, stale)
			})
			if !errors.Is(err, repository.ErrOnboardingStateConflict) {
				t.Fatalf("stale %s CAS: %v, want ErrOnboardingStateConflict", stale.ExpectedStatus, err)
			}
			f.assertState(constants.OnboardingStatusApproved, true)
			f.assertAudit(constants.AuditActionOnboardingSubmit, 1)
			f.assertAudit(constants.AuditActionOnboardingResubmit, 0)
		}
	})
	t.Run("review_queue_maps_rows_and_rejects_incomplete_or_obsolete_approvals", func(t *testing.T) {
		f := newOnboardingFixture(t, ctx, db)
		f.init()
		f.exec(`UPDATE onboardings SET status='pending_verification' WHERE onboarding_id=$1`, f.application.OnboardingID)
		items, svcErr := f.service.ListOnboardingReviews(ctx, constants.OnboardingStatusPendingVerification)
		if svcErr != nil {
			t.Fatal(svcErr)
		}
		found := false
		for _, item := range items {
			if item.OnboardingID == f.application.OnboardingID {
				found = true
				if item.RequiredDocs == 0 || item.UploadedDocs != 0 {
					t.Fatalf("bad document counts: %+v", item)
				}
			}
		}
		if !found {
			t.Fatal("application missing from review queue")
		}
		input := models.ReviewOnboardingInput{ActorID: f.actorID, OnboardingID: f.application.OnboardingID, Status: constants.OnboardingStatusApproved}
		_, svcErr = f.service.ReviewOnboarding(ctx, input)
		wantServiceStatus(t, svcErr, http.StatusConflict)
		f.assertState(constants.OnboardingStatusPendingVerification, false)
		f.exec(`UPDATE onboarding_documents SET upload_status='uploaded' WHERE onboarding_id=$1`, f.application.OnboardingID)
		f.exec(`INSERT INTO onboardings (user_id,role,status,created_at) VALUES ($1, 'restaurant_owner','rejected', NOW()+INTERVAL '1 minute')`, f.userID)
		_, svcErr = f.service.ReviewOnboarding(ctx, input)
		wantServiceStatus(t, svcErr, http.StatusBadRequest)
		f.assertState(constants.OnboardingStatusPendingVerification, false)
	})
}

type onboardingFixture struct {
	t           *testing.T
	ctx         context.Context
	db          *sqlx.DB
	repo        repository.Repository
	service     *business.Service
	storage     *onboardingStorage
	userID      string
	actorID     string
	application *models.InitOnboardingOutput
}

func newOnboardingFixture(t *testing.T, ctx context.Context, db *sqlx.DB) *onboardingFixture {
	t.Helper()
	f := &onboardingFixture{t: t, ctx: ctx, db: db, repo: repository.NewRepository(db, nil), storage: &onboardingStorage{objects: make(map[string]bool)}}
	cfg := &config.Config{}
	cfg.S3.DefaultBucket = "onboarding-integration"
	cfg.S3.PresignTTLSeconds = 60
	f.service = business.NewService(f.repo, cfg, zerolog.Nop(), nil, nil, f.storage)
	f.userID = f.createUser(constants.RoleRestaurantOwner)
	f.actorID = f.createUser(constants.RoleRestaurantManager)
	return f
}

func (f *onboardingFixture) createUser(role string) string {
	f.t.Helper()
	id := uuid.New()
	phone := fmt.Sprintf("+%013d", binary.BigEndian.Uint64(id[:8])%10000000000000)
	f.exec(`INSERT INTO users (user_id, phone, name, email, role) VALUES ($1::uuid, $2, 'Onboarding integration', $3, $4::user_role)`, id.String(), phone, id.String()+"@onboarding.invalid", role)
	f.t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := f.db.ExecContext(ctx, `DELETE FROM audit_logs WHERE actor_id = $1::uuid`, id.String()); err != nil {
			f.t.Errorf("clean fixture audit logs: %v", err)
		}
		if _, err := f.db.ExecContext(ctx, `DELETE FROM users WHERE user_id = $1::uuid`, id.String()); err != nil {
			f.t.Errorf("clean fixture user: %v", err)
		}
	})
	return id.String()
}

func (f *onboardingFixture) init() {
	f.t.Helper()
	out, svcErr := f.service.InitOnboarding(f.ctx, models.InitOnboardingInput{UserID: f.userID, Role: constants.RoleRestaurantOwner, Country: "IN"})
	if svcErr != nil || out == nil || out.Status != constants.OnboardingStatusDraft || len(out.Documents) == 0 {
		f.t.Fatalf("initialize: output=%+v error=%+v", out, svcErr)
	}
	f.application = out
}

func (f *onboardingFixture) uploadAll() {
	f.t.Helper()
	for _, doc := range f.application.Documents {
		f.storage.objects[doc.S3Key] = true
		if svcErr := f.service.MarkDocumentUploaded(f.ctx, models.MarkDocumentUploadedInput{UserID: f.userID, S3Key: doc.S3Key}); svcErr != nil {
			f.t.Fatalf("confirm %s: %+v", doc.DocumentType, svcErr)
		}
	}
}

func (f *onboardingFixture) submitInput() models.SubmitOnboardingInput {
	return models.SubmitOnboardingInput{UserID: f.userID, OnboardingID: f.application.OnboardingID}
}

func (f *onboardingFixture) submit() {
	f.t.Helper()
	out, svcErr := f.service.SubmitOnboarding(f.ctx, f.submitInput())
	if svcErr != nil || out == nil || out.Status != constants.OnboardingStatusPendingVerification {
		f.t.Fatalf("submit: output=%+v error=%+v", out, svcErr)
	}
}

func (f *onboardingFixture) review(status string) {
	f.t.Helper()
	out, svcErr := f.service.ReviewOnboarding(f.ctx, models.ReviewOnboardingInput{ActorID: f.actorID, OnboardingID: f.application.OnboardingID, Status: status, RejectionReason: "Replace unreadable document"})
	if svcErr != nil || out == nil || out.Status != status {
		f.t.Fatalf("review: output=%+v error=%+v", out, svcErr)
	}
}

func (f *onboardingFixture) assertState(status string, complete bool) {
	f.t.Helper()
	var got struct {
		Status   string `db:"status"`
		Complete bool   `db:"onboarding_complete"`
	}
	f.get(&got, `SELECT o.status, u.onboarding_complete FROM onboardings o JOIN users u ON u.user_id = o.user_id WHERE o.onboarding_id = $1::uuid`, f.application.OnboardingID)
	if got.Status != status || got.Complete != complete {
		f.t.Fatalf("state=%s complete=%v, want %s complete=%v", got.Status, got.Complete, status, complete)
	}
}

func (f *onboardingFixture) assertAudit(action string, want int) {
	f.t.Helper()
	var count int
	f.get(&count, `SELECT COUNT(*) FROM audit_logs WHERE entity_type = 'onboardings' AND entity_id = $1 AND action = $2`, f.application.OnboardingID, action)
	if count != want {
		f.t.Fatalf("%s audit count=%d, want %d", action, count, want)
	}
}

func (f *onboardingFixture) assertDocument(id, status string) {
	f.t.Helper()
	var got string
	f.get(&got, `SELECT upload_status FROM onboarding_documents WHERE document_id = $1::uuid`, id)
	if got != status {
		f.t.Fatalf("document status=%s, want %s", got, status)
	}
}

func (f *onboardingFixture) documents() []string {
	f.t.Helper()
	var docs []string
	if err := f.db.SelectContext(f.ctx, &docs, `SELECT document_id::text || ':' || document_type || ':' || s3_key || ':' || upload_status FROM onboarding_documents WHERE onboarding_id = $1::uuid ORDER BY document_id`, f.application.OnboardingID); err != nil {
		f.t.Fatal(err)
	}
	return docs
}

func (f *onboardingFixture) exec(query string, args ...any) {
	f.t.Helper()
	if _, err := f.db.ExecContext(f.ctx, query, args...); err != nil {
		f.t.Fatal(err)
	}
}

func (f *onboardingFixture) get(dest any, query string, args ...any) {
	f.t.Helper()
	if err := f.db.GetContext(f.ctx, dest, query, args...); err != nil {
		f.t.Fatal(err)
	}
}

func wantServiceStatus(t *testing.T, err *models.ServiceError, status int) {
	t.Helper()
	if err == nil || err.StatusCode != status {
		t.Fatalf("service error=%+v, want HTTP %d", err, status)
	}
}

type onboardingStorage struct {
	mu         sync.Mutex
	objects    map[string]bool
	existsErr  error
	presigns   int
	checks     int
	lastBucket string
	lastKey    string
}

var _ storage.Provider = (*onboardingStorage)(nil)

func (s *onboardingStorage) PresignPut(_ context.Context, in storage.PresignPutInput) (*storage.PresignPutOutput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.presigns++
	return &storage.PresignPutOutput{URL: "https://storage.invalid/" + in.Bucket + "/" + in.Key, Method: http.MethodPut, ExpiresAt: time.Now().Add(in.ExpiresIn)}, nil
}

func (s *onboardingStorage) ObjectExists(_ context.Context, bucket, key string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checks++
	s.lastBucket, s.lastKey = bucket, key
	return s.objects[key], s.existsErr
}

func (s *onboardingStorage) counts() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.presigns, s.checks
}
