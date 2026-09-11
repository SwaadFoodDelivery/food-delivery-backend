# MinIO integration test

`TestMinIOPutHeadIntegration` skips unless `STORAGE_MINIO_ENDPOINT` is set.
It requires an existing bucket and credentials allowed to PUT, HEAD, and DELETE
objects in that bucket (and list permission for missing-object HEAD to return
404 rather than 403). It does not create buckets or change policies.

Run against the local development MinIO from the backend root:

```sh
STORAGE_MINIO_ENDPOINT=http://localhost:9000 \
STORAGE_MINIO_BUCKET=food-delivery-onboarding \
STORAGE_MINIO_ACCESS_KEY=minioadmin \
STORAGE_MINIO_SECRET_KEY=minioadmin \
go test -count=1 -v ./internal/services/common/storage -run '^TestMinIOPutHeadIntegration$'
```

These are local development credentials. For other instances, supply credentials
through your environment. `STORAGE_MINIO_REGION` defaults to `us-east-1`.

The test verifies absence, uploads a nonempty object using `PresignPut` with its
signed Content-Type header, and calls the actual `ObjectExists` implementation.
This validates MinIO's acceptance of the current SigV4 `UNSIGNED-PAYLOAD` HEAD
signature. It then overwrites the same generated object with an empty upload
and verifies rejection. There are no mocked HTTP responses or signature checks.

Each run uses a cryptographically random 128-bit key beneath
`storage-integration/`, including a space to exercise URI escaping. Cleanup uses
the private signer for a DELETE of only that exact generated key and checks
absence with HEAD; it never lists or recursively deletes objects. No `mc` or AWS
SDK is needed. Failures omit signed URLs, response bodies, and credentials.
If cleanup fails, the failure reports only the bucket and generated key so the
fixture can be removed manually. A forcibly terminated process may leave its
fixture. On a versioned bucket, DELETE can leave historical versions and a
delete marker; use an unversioned development bucket for complete removal.
