# Order identity guard for history and cancellation

Orders use the composite database identity `(order_id, created_at)`. The existing
history and cancellation HTTP routes accept only an order UUID. If more than one
row belongs to that UUID **and the authenticated user**, both routes now return
HTTP 409 with `ORDER_ID_AMBIGUOUS`. The API does not silently choose the latest row.
Foreign rows sharing the UUID neither cause ambiguity nor receive mutations.
Zero owned matches still return 404 `ORDER_NOT_FOUND`; terminal cancellation still
returns 409 `ORDER_NOT_CANCELABLE`.

History inspects at most two owned matches and reads events using the sole selected
created_at. Cancellation locks at most two owned matches and rejects ambiguity
before any order, delivery, audit, or notification change. A successful cancellation
updates exactly the selected user/order_id/created_at row and requires one affected
row. Delivery updates retain the full composite order identity.

This is a selection-time guard, not a uniqueness guarantee or a lock against future
partition inserts. A new same-ID row committed after selection is not cancelled;
subsequent UUID-only history/cancellation requests then report ambiguity. Supporting
explicit composite identities is a separate API change. This patch changes no
schema, gRPC writes, order placement, payment rules, or read-service behavior.

The customer list UI must treat `ORDER_ID_AMBIGUOUS` as an actionable conflict and
must not retry cancellation or assume a UUID uniquely identifies a historical row.
This guard is a prerequisite for that UI; it stacks on backend list-read PR19:
https://github.com/SwaadFoodDelivery/food-delivery-backend/pull/19.

## Verification

After coordinating dedicated database use with root:

```sh
ORDER_IDENTITY_TEST_DATABASE_URL='<private URL for swaad_grpc_test_20260912>' \
  go test -race ./internal/services/order/repository -run '^TestOrderIdentityPostgres$' -count=1 -v
go test -race ./...
go vet ./...
```

The opt-in integration test verifies the requested and actual database names,
requires inherited clean backend schema29 and PostGIS, and never creates databases
or runs migrations. It adds private fictional users/orders to the disposable test
database and leaves those fixtures there. It compares complete before/after rows
and side effects for ambiguity/terminal denial, tests foreign ownership and shared
UUIDs, and synchronizes a second connection's insert after cancellation selection
with channels through a private per-call test seam. No sleeps or global hooks are
used. CI runs it in the existing paired job's dedicated database after schema setup.

Verified locally on 2026-09-14: full `go test -race -count=1 ./...`, `go vet ./...`,
and the opt-in PostgreSQL command above all passed. PostgreSQL ran against
`swaad_grpc_test_20260912` with clean schema29 and PostGIS. All six scenarios passed,
including the channel-synchronized insert-after-selection regression. Other opt-in
integration suites were not enabled during the full race run. No migrations or
runtime restarts were performed.

## Rollback

No data migration needs undoing. If rollback is necessary, disable the affected
cancellation UI/route and retain the composite update predicate and affected-row
check until an equivalent guard is restored. Do not revert to the broad UUID-only
cancellation UPDATE: that restores the safety defect. History should continue to
reject ambiguous IDs, or be disabled while its replacement is prepared.
