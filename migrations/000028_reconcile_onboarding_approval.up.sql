-- Submission previously granted approval before an operations review. Revoke
-- that accidental grant for accounts whose latest application is not approved.
-- Seeded accounts without applications and reviewed approvals are unchanged.
UPDATE users u
SET onboarding_complete = FALSE, updated_at = NOW()
WHERE u.onboarding_complete = TRUE
  AND (SELECT o.status FROM onboardings o WHERE o.user_id = u.user_id
       ORDER BY o.created_at DESC, o.onboarding_id DESC LIMIT 1)
      IN ('draft', 'pending_verification', 'rejected');
