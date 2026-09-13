# ORDER-GRPC-001/002: actual order-service reads

Implemented boundary: authenticated client `GET /api/v1/orders/:orderId` calls
typed `order.v1.OrderService/GetOrder`. The route exists only when
`ORDER_GRPC_REQUIRED=true`. Disabled mode does not silently serve a local order
snapshot; the new route is absent. When enabled, `GET /api/v1/orders` also calls
typed `GetUserOrders` for owned paginated summaries. Existing checkout, timeline,
cancellation, payment and delivery endpoints keep their backend implementations.

The old `any` echo-success gRPC methods were placeholders, not integration. They
are removed. This feature does not transfer write ownership to order-service.
Order-service reads the backend-owned schema through read-only transactions;
it must not run its copied legacy migrations. PostgreSQL remains the single
order record, not a second independently populated orders database. A future
writer extraction requires an explicit cart/menu/address/notification transaction
strategy and migration ownership handover first.

## Local-only configuration

Run the companion order-service branch against a disposable backend-migrated
schema 29 database with SELECT-only credentials. Start the backend with:

```text
APP_ENV=development
ORDER_GRPC_REQUIRED=true
ORDER_GRPC_ADDR=127.0.0.1:50051
ORDER_GRPC_SERVICE_KEY=<same private key configured on order-service>
ORDER_GRPC_TIMEOUT_MS=2000
```

Supply a private key of at least 32 characters via the process environment; never
put a real key in source or browser configuration. The caller sends it only as
`x-order-service-key` gRPC metadata. Plaintext is restricted to development/test
and numeric loopback IPs; production, DNS names and non-loopback are rejected.
This is not a public deployment or TLS/mTLS implementation. Timeout range is
100–10000 ms, also bounded by the upstream request deadline. An enabled dependency
that cannot connect fails backend startup. At runtime it fails the request; no
local fallback hides a broken integration.

The HTTP authentication middleware supplies user UUID and role. URL query/body
requester identities are ignored. Only clients may read their own order; the
service must enforce ownership in its SQL. Foreign and missing orders return
the same 404. Response JSON exposes persisted item snapshots, integer INR paise,
current stored status and timestamps; deprecated protobuf float fields are not
returned to browser clients.

| gRPC result | HTTP |
| --- | --- |
| InvalidArgument | 400 |
| NotFound, including foreign order | 404 |
| PermissionDenied | 403 |
| DeadlineExceeded | 504 |
| Unavailable / Canceled | 503 |
| Dependency Unauthenticated, Internal, Unimplemented | 502 |

Dependency auth errors are not customer-session expiry (401); the API never
returns internal RPC diagnostics or credentials. Invalid local user identity is
401; non-client local roles are denied before calling gRPC.

Contract: proto PR2, pinned pseudo-version
`v0.0.0-20260913090154-dc2e8c6d6c1d`; no local module replacement is committed.
Run `go test -race ./...` and `go vet ./...`. HTTP tests use an actual loopback
gRPC server to verify transport, delegated identity, exact amounts, deadlines,
error mapping and opt-in route registration. Real order-service/PostGIS paired
acceptance is a separate required gate, not implied by those contract tests.

That paired gate is now committed as TestPairedOrderServicePostgres and the
paired-order-service CI job. It builds the pinned real order-service
implementation, uses a SELECT-only login against fresh backend-migrated PostGIS,
and checks HTTP owned reads, stored snapshots, ownership/role denial and service
credential failure. The list subtest checks tied timestamps, microsecond precision,
insertion between pages, terminal/empty pages and foreign/tampered cursors.
It explicitly seeds middleware identity, not browser login.
Local rerun with a running service and the dedicated schema:

```sh
ORDER_RPC_E2E_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/swaad_grpc_test_20260912?sslmode=disable' \
ORDER_RPC_E2E_ADDR=127.0.0.1:15052 \
ORDER_RPC_E2E_KEY='<same-local-service-key>' \
go test -race ./internal/services/order/api -run TestPairedOrderServicePostgres -count=1 -v
```

New fictional fixtures remain in the guarded disposable database. Partial
configuration fails; absent all configuration explicitly skips the paired test.
CI supplies it, so CI cannot satisfy this gate with a skipped local-only test.

Rollback: disable `ORDER_GRPC_REQUIRED` and revert this feature through review.
No data migration or order rewrite is required. The established demo checkout
is unaffected; consumers of the new read endpoint must tolerate its absence.

## Paginated order summaries

`GET /api/v1/orders?limit=20&cursor=<opaque-token>` returns the existing
`data.orders` summary array plus `data.next_cursor`. Limits must be 1–50; omitted
limit defaults to 20. Empty results are `[]`; an empty next_cursor means there
is no further page. No global total is available; deprecated RPC total is not
exposed as a count. The existing frontend can continue consuming its first page.

The service orders by `(created_at, order_id)` descending, fetches at most limit+1
rows and signs a versioned owner-bound cursor from the last returned row only
when another row exists. SQL independently filters the authenticated owner.
Invalid, foreign or tampered cursors return HTTP 400. Cursor size is at most 1024
bytes. Pagination preserves ordering, not a frozen cross-request snapshot; newer
inserts appear on a fresh first page. Key rotation invalidates existing cursors.
Summary reads do not fetch items or payment details. Missing delivery remains
omitted, and inactive restaurants do not hide a customer's historical orders.

Deploy the list-capable service and SELECT-only grants for `restaurants` and
`deliveries` (in addition to `orders`, `order_items`, `payments`) BEFORE this
backend with delegation enabled. An old GetOrder-only service returns 502 for
the list; there is deliberately no local SQL fallback. On rollback, revert the
backend before reverting service/grants. Disabled mode retains local first-page
listing but rejects any nonempty cursor with 400, so clients should clear it and
restart the list. Never grant INSERT/UPDATE/DELETE/TRUNCATE/REFERENCES/TRIGGER.
