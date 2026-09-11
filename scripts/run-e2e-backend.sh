#!/usr/bin/env bash
set -euo pipefail

# Development only: isolated fixtures, fixed harmless local secrets, no paid services.
# Start the dedicated dependencies and apply seed_demo.sql per the frontend E2E
# README. This command runs migrations automatically but never resets a database.
cd "$(dirname "$0")/.."
export APP_ENV=development APP_PORT=18080
export POSTGRES_HOST=127.0.0.1 POSTGRES_PORT=5432 POSTGRES_USER=postgres
export POSTGRES_PASSWORD=postgres POSTGRES_DB=swaad_e2e_20260911 POSTGRES_SSLMODE=disable
export REDIS_ADDR=127.0.0.1:16379 NATS_URL=nats://127.0.0.1:14222 NATS_CLIENT_ID=swaad-e2e
export JWT_SECRET=dev-secret-change-me-in-prod
export GUEST_TOKEN_SECRET=dev-guest-secret-change-me-in-prod-1234567890abcdef
export CART_HMAC_SECRET=dev-cart-hmac-secret-change-me-in-prod-1234567890abcdef
export CLIENT_API_KEY=dev-client-api-key
export S3_PROVIDER=dev S3_ENDPOINT=http://127.0.0.1:9000 S3_PRESIGN_BASE_URL=http://127.0.0.1:9000
export S3_ACCESS_KEY_ID=minioadmin S3_SECRET_ACCESS_KEY=minioadmin S3_REGION=ap-south-1
export S3_DEFAULT_BUCKET=food-delivery-local S3_ONBOARDING_BUCKET=food-delivery-onboarding
export PAYMENT_PROVIDER=mock DELIVERY_PROVIDER=mock OTP_PROVIDER=mock EMAIL_PROVIDER=mock
export MOCK_DELIVERY_DURATION_SECONDS=30 ORDER_GRPC_REQUIRED=false
exec go run ./cmd/server
