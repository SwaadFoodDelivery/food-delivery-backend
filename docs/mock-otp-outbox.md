# Private mock OTP transport for browser tests

Implemented for local learning/test environments only. The browser still calls
the real guest, send-OTP and verify-OTP endpoints. Random generation, Redis
hashing/expiry, attempt limits, session creation and HttpOnly refresh cookies are
unchanged. Only SMS delivery is replaced by a private filesystem outbox. No HTTP
endpoint exposes codes and there is no fixed test OTP.

From this backend checkout, with the disposable dependencies described in the
frontend `tests/e2e/README.md` running:

```sh
otp_test_outbox=$(mktemp -d /tmp/swaad-otp-outbox.XXXXXX)
MOCK_OTP_OUTBOX_DIR="$otp_test_outbox" REDIS_DB=1 bash scripts/run-e2e-backend.sh
```

Supply that exact directory as `E2E_OTP_OUTBOX` to the frontend auth test in a
second terminal. The launcher selects `APP_ENV=development`, `OTP_PROVIDER=mock`
and harmless local demo credentials. It migrates only `swaad_e2e_20260911`; do not
point these helpers at a shared or production database. Database 1 refers to the
dedicated E2E Redis on port 16379, not the ordinary development Redis.

Configuration rejects an outbox unless the provider is `mock` and environment is
`development` or `test`. The provider requires an existing absolute directory
with permissions 0700 and rejects a symlink as the final directory component.
Private 0600 files are atomically published as `.delivery-*.json`, containing
`phone_hash` (SHA-256 of the normalized phone), `code` and UTC `sent_at`. The hash
is a correlation key, not anonymization. Treat the entire directory as sensitive.
When the outbox is enabled, codes are not also written to application logs.
With it unset, the existing mock transport's development log behavior remains.

The browser reads only its fresh matching delivery and deletes that file after a
successful verification. Failed/interrupted runs can leave files; there is no
automatic filesystem expiry. OTP validation still expires in Redis. After
stopping the owned backend, inspect and remove only the exact generated outbox
and private test fixture directories when no longer needed. Never publish them,
include them in reports, or enable traces/screenshots of OTP inputs. Do not use
real phone numbers or identity documents. Codes are for fictional local accounts.

Auth tests obey OTP rate limits. Each full run sends two OTPs from loopback;
rapid repetition can correctly receive 429. Wait for the limiter allowance to
refill rather than deleting buckets or disabling limits. The auth setup uses
Redis DB 1; legacy session-seeded customer/persona helpers currently use DB 0,
so restart the owned backend with the matching DB before those suites.

Verification: `go test -race ./...`, `go vet ./...`, CI PostGIS migration and
repository suites, and frontend `npm run test:e2e:auth`. This does not validate
real SMS/email transport, registration, OTP expiry UX or a public deployment.
