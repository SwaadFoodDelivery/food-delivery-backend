package repository

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	postgresinfra "food-delivery-backend/infra/postgres"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

const (
	integrationUserID      = "00000000-0000-4000-8000-000000000004"
	integrationOtherUserID = "00000000-0000-4000-8000-000000000002"
	integrationRestaurant  = "10000000-0000-4000-8000-000000000001"

	serviceableAddressID = "70000000-0000-4000-8000-000000000001"
	outsideAddressID     = "70000000-0000-4000-8000-000000000002"
	missingAddressID     = "70000000-0000-4000-8000-000000000003"
	boundaryAddressID    = "70000000-0000-4000-8000-000000000004"
)

func TestServiceabilityPostGIS(t *testing.T) {
	dsn := os.Getenv("SERVICEABILITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("SERVICEABILITY_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := sqlx.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	root := repositoryRoot(t)
	if err := postgresinfra.RunMigrations(ctx, db.DB, filepath.Join(root, "migrations")); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	seed, err := os.ReadFile(filepath.Join(root, "scripts", "seed_demo.sql"))
	if err != nil {
		t.Fatalf("read demo seed: %v", err)
	}
	if _, err := db.ExecContext(ctx, string(seed)); err != nil {
		t.Fatalf("seed demo fixtures: %v", err)
	}

	userID := uuid.MustParse(integrationUserID)
	otherUserID := uuid.MustParse(integrationOtherUserID)
	restaurantID := uuid.MustParse(integrationRestaurant)
	addressIDs := []uuid.UUID{
		uuid.MustParse(serviceableAddressID),
		uuid.MustParse(outsideAddressID),
		uuid.MustParse(missingAddressID),
		uuid.MustParse(boundaryAddressID),
	}
	cleanupAddresses(t, ctx, db, addressIDs)
	defer cleanupAddresses(t, context.Background(), db, addressIDs)

	_, err = db.ExecContext(ctx, `
		INSERT INTO addresses (address_id, user_id, line1, city, state, pincode, latitude, longitude, is_default)
		VALUES
			($1, $4, 'Serviceable fixture', 'Shamgarh', 'Madhya Pradesh', '458883', 24.187400, 75.639600, TRUE),
			($2, $4, 'Outside fixture', 'Shamgarh', 'Madhya Pradesh', '458883', 24.000000, 75.000000, FALSE),
			($3, $4, 'Missing coordinates fixture', 'Shamgarh', 'Madhya Pradesh', '458883', NULL, NULL, FALSE)`,
		addressIDs[0], addressIDs[1], addressIDs[2], userID)
	if err != nil {
		t.Fatalf("insert serviceability fixtures: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO addresses (address_id, user_id, line1, city, state, pincode, latitude, longitude, is_default)
		SELECT $1, $2, 'Boundary fixture', 'Shamgarh', 'Madhya Pradesh', '458883',
		       ST_Y(ST_Project(r.location::geography, (r.service_radius_km * 1000) - 1, radians(90))::geometry),
		       ST_X(ST_Project(r.location::geography, (r.service_radius_km * 1000) - 1, radians(90))::geometry), FALSE
		FROM restaurants r WHERE r.restaurant_id = $3`, addressIDs[3], userID, restaurantID); err != nil {
		t.Fatalf("insert boundary fixture: %v", err)
	}

	repo := NewPostgresRepository(db)
	serviceable, err := repo.CheckServiceability(ctx, userID, addressIDs[0], restaurantID)
	if err != nil {
		t.Fatalf("serviceable decision: %v", err)
	}
	if !serviceable.Serviceable || serviceable.ReasonCode != "serviceable" || serviceable.DeliveryFee != 3000 || serviceable.EstimatedDeliveryMin != 35 {
		t.Fatalf("serviceable decision = %#v", serviceable)
	}

	outside, err := repo.CheckServiceability(ctx, userID, addressIDs[1], restaurantID)
	if err != nil {
		t.Fatalf("outside decision: %v", err)
	}
	if outside.Serviceable || outside.ReasonCode != "outside_delivery_radius" {
		t.Fatalf("outside decision = %#v", outside)
	}

	missing, err := repo.CheckServiceability(ctx, userID, addressIDs[2], restaurantID)
	if err != nil {
		t.Fatalf("missing-coordinate decision: %v", err)
	}
	if missing.Serviceable || missing.ReasonCode != "missing_coordinates" {
		t.Fatalf("missing-coordinate decision = %#v", missing)
	}

	boundary, err := repo.CheckServiceability(ctx, userID, addressIDs[3], restaurantID)
	if err != nil {
		t.Fatalf("boundary decision: %v", err)
	}
	if !boundary.Serviceable || boundary.ReasonCode != "serviceable" {
		t.Fatalf("boundary decision = %#v", boundary)
	}

	if _, err := repo.CheckServiceability(ctx, otherUserID, addressIDs[0], restaurantID); err != ErrAddressNotFound {
		t.Fatalf("ownership decision = %v, want ErrAddressNotFound", err)
	}

	if _, err := db.ExecContext(ctx, `UPDATE restaurants SET is_open = FALSE WHERE restaurant_id = $1`, restaurantID); err != nil {
		t.Fatalf("close restaurant fixture: %v", err)
	}
	closed, err := repo.CheckServiceability(ctx, userID, addressIDs[0], restaurantID)
	if err != nil {
		t.Fatalf("closed decision: %v", err)
	}
	if closed.Serviceable || closed.ReasonCode != "restaurant_closed" {
		t.Fatalf("closed decision = %#v", closed)
	}
	if _, err := db.ExecContext(ctx, `UPDATE restaurants SET is_open = TRUE WHERE restaurant_id = $1`, restaurantID); err != nil {
		t.Fatalf("restore restaurant fixture: %v", err)
	}

	if _, err := db.ExecContext(ctx, `UPDATE restaurants SET status = 'inactive' WHERE restaurant_id = $1`, restaurantID); err != nil {
		t.Fatalf("deactivate restaurant fixture: %v", err)
	}
	unavailable, err := repo.CheckServiceability(ctx, userID, addressIDs[0], restaurantID)
	if err != nil {
		t.Fatalf("inactive decision: %v", err)
	}
	if unavailable.Serviceable || unavailable.ReasonCode != "restaurant_unavailable" {
		t.Fatalf("inactive decision = %#v", unavailable)
	}
	if _, err := db.ExecContext(ctx, `UPDATE restaurants SET status = 'active' WHERE restaurant_id = $1`, restaurantID); err != nil {
		t.Fatalf("restore restaurant status fixture: %v", err)
	}
}

func cleanupAddresses(t *testing.T, ctx context.Context, db *sqlx.DB, ids []uuid.UUID) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `DELETE FROM addresses WHERE address_id = ANY($1)`, pq.Array(ids)); err != nil {
		t.Fatalf("cleanup serviceability fixtures: %v", err)
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve repository test path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../../.."))
}
