package postgres

import (
	"context"

	apperrors "food-delivery-backend/internal/errors"
	"food-delivery-backend/internal/services/users/models"

	"github.com/jmoiron/sqlx"
)

func (s *Store) GetUserProfileByID(ctx context.Context, userID string) (*models.UserProfileRow, error) {
	var row models.UserProfileRow
	err := sqlx.GetContext(ctx, s.accessor.Queryer(), &row, `
		SELECT user_id::text, name, phone,
		       email, email_verified, phone_verified,
		       role::text, account_status::text, onboarding_complete,
		       profile_image_s3_key, created_at, updated_at
		FROM users
		WHERE user_id = $1::uuid
		  AND is_deleted = FALSE
		LIMIT 1
	`, userID)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &row, nil
}

func (s *Store) GetClientProfileByUserID(ctx context.Context, userID string) (*models.ClientProfileRow, error) {
	var row models.ClientProfileRow
	err := sqlx.GetContext(ctx, s.accessor.Queryer(), &row, `
		SELECT user_id::text, date_of_birth, gender, created_at, updated_at
		FROM client_profiles
		WHERE user_id = $1::uuid
		LIMIT 1
	`, userID)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &row, nil
}

func (s *Store) GetDriverProfileByUserID(ctx context.Context, userID string) (*models.DriverProfileRow, error) {
	var row models.DriverProfileRow
	err := sqlx.GetContext(ctx, s.accessor.Queryer(), &row, `
		SELECT user_id::text, is_available, current_city, created_at, updated_at
		FROM driver_profiles
		WHERE user_id = $1::uuid
		LIMIT 1
	`, userID)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &row, nil
}

// UpdateUserCore deliberately cannot write email. An email is only ever stored
// after its owner has answered an OTP sent to it, which is UpdateUserEmail's job.
func (s *Store) UpdateUserCore(ctx context.Context, userID, name string) error {
	_, err := s.accessor.Execer().ExecContext(ctx, `
		UPDATE users
		SET name       = $2,
		    updated_at = NOW()
		WHERE user_id = $1::uuid
		  AND is_deleted = FALSE
	`, userID, name)
	return err
}

// UpdateUserEmail is called only once the OTP sent to the new address has been
// answered, so the address lands already verified.
func (s *Store) UpdateUserEmail(ctx context.Context, userID, email string) error {
	res, err := s.accessor.Execer().ExecContext(ctx, `
		UPDATE users
		SET email          = $2,
		    email_verified = TRUE,
		    updated_at     = NOW()
		WHERE user_id = $1::uuid
		  AND is_deleted = FALSE
	`, userID, email)
	if err != nil {
		return mapUniqueViolation(err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (s *Store) UpsertClientProfile(ctx context.Context, userID, dateOfBirth, gender string) error {
	_, err := s.accessor.Execer().ExecContext(ctx, `
		INSERT INTO client_profiles (user_id, date_of_birth, gender, created_at, updated_at)
		VALUES ($1::uuid, NULLIF($2, '')::date, NULLIF($3, ''), NOW(), NOW())
		ON CONFLICT (user_id) DO UPDATE
		SET date_of_birth = COALESCE(NULLIF($2, '')::date, client_profiles.date_of_birth),
		    gender        = COALESCE(NULLIF($3, ''), client_profiles.gender),
		    updated_at    = NOW()
	`, userID, dateOfBirth, gender)
	return err
}

func (s *Store) UpsertDriverProfile(ctx context.Context, userID string, isAvailable *bool, currentCity string) error {
	var availVal any
	if isAvailable != nil {
		availVal = *isAvailable
	}
	_, err := s.accessor.Execer().ExecContext(ctx, `
		UPDATE driver_profiles
		SET is_available = COALESCE($2, is_available),
		    current_city = COALESCE(NULLIF($3, ''), current_city),
		    updated_at   = NOW()
		WHERE user_id = $1::uuid
	`, userID, availVal, currentCity)
	return err
}

func (s *Store) CountActiveAddresses(ctx context.Context, userID string) (int, error) {
	var count int
	err := sqlx.GetContext(ctx, s.accessor.Queryer(), &count, `
		SELECT COUNT(*) FROM addresses
		WHERE user_id = $1::uuid AND is_deleted = FALSE
	`, userID)
	return count, err
}

func (s *Store) ListAddresses(ctx context.Context, userID string, offset, limit int) ([]models.AddressRow, error) {
	rows := make([]models.AddressRow, 0)
	err := sqlx.SelectContext(ctx, s.accessor.Queryer(), &rows, `
		SELECT address_id::text, user_id::text, label, line1, line2, area,
		       city, state, pincode, latitude, longitude,
		       contact_name, contact_phone, is_default, is_deleted,
		       created_at, updated_at
		FROM addresses
		WHERE user_id = $1::uuid AND is_deleted = FALSE
		ORDER BY is_default DESC, created_at DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	return rows, err
}

func (s *Store) CountAddresses(ctx context.Context, userID string) (int, error) {
	var count int
	err := sqlx.GetContext(ctx, s.accessor.Queryer(), &count, `
		SELECT COUNT(*) FROM addresses WHERE user_id = $1::uuid AND is_deleted = FALSE
	`, userID)
	return count, err
}

func (s *Store) UnsetDefaultAddress(ctx context.Context, userID string) error {
	_, err := s.accessor.Execer().ExecContext(ctx, `
		UPDATE addresses SET is_default = FALSE, updated_at = NOW()
		WHERE user_id = $1::uuid AND is_deleted = FALSE AND is_default = TRUE
	`, userID)
	return err
}

func (s *Store) CreateAddress(ctx context.Context,
	userID, label, line1, line2, area, city, state, pincode string,
	lat, lon *float64,
	contactName, contactPhone string,
	isDefault bool,
) (*models.AddressRow, error) {
	var row models.AddressRow
	err := sqlx.GetContext(ctx, s.accessor.Queryer(), &row, `
		INSERT INTO addresses
		    (user_id, label, line1, line2, area, city, state, pincode,
		     latitude, longitude, contact_name, contact_phone, is_default,
		     is_deleted, created_at, updated_at)
		VALUES
		    ($1::uuid, NULLIF($2,''), $3, NULLIF($4,''), NULLIF($5,''),
		     $6, $7, $8, $9, $10, NULLIF($11,''), NULLIF($12,''), $13,
		     FALSE, NOW(), NOW())
		RETURNING
		    address_id::text, user_id::text, label, line1, line2, area,
		    city, state, pincode, latitude, longitude,
		    contact_name, contact_phone, is_default, is_deleted,
		    created_at, updated_at
	`, userID, label, line1, line2, area, city, state, pincode,
		lat, lon, contactName, contactPhone, isDefault)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *Store) FindAddressByIDAndUser(ctx context.Context, addressID, userID string) (*models.AddressRow, error) {
	var row models.AddressRow
	err := sqlx.GetContext(ctx, s.accessor.Queryer(), &row, `
		SELECT address_id::text, user_id::text, label, line1, line2, area,
		       city, state, pincode, latitude, longitude,
		       contact_name, contact_phone, is_default, is_deleted,
		       created_at, updated_at
		FROM addresses
		WHERE address_id = $1::uuid AND user_id = $2::uuid AND is_deleted = FALSE
		LIMIT 1
	`, addressID, userID)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &row, nil
}

func (s *Store) UpdateAddress(ctx context.Context,
	addressID, userID, label, line1, line2, area, city, state, pincode string,
	lat, lon *float64,
	contactName, contactPhone string,
	isDefault bool,
) (*models.AddressRow, error) {
	var row models.AddressRow
	err := sqlx.GetContext(ctx, s.accessor.Queryer(), &row, `
		UPDATE addresses
		SET label         = NULLIF($3, ''),
		    line1         = $4,
		    line2         = NULLIF($5, ''),
		    area          = NULLIF($6, ''),
		    city          = $7,
		    state         = $8,
		    pincode       = $9,
		    latitude      = $10,
		    longitude     = $11,
		    contact_name  = NULLIF($12, ''),
		    contact_phone = NULLIF($13, ''),
		    is_default    = $14,
		    updated_at    = NOW()
		WHERE address_id = $1::uuid AND user_id = $2::uuid AND is_deleted = FALSE
		RETURNING
		    address_id::text, user_id::text, label, line1, line2, area,
		    city, state, pincode, latitude, longitude,
		    contact_name, contact_phone, is_default, is_deleted,
		    created_at, updated_at
	`, addressID, userID, label, line1, line2, area, city, state, pincode,
		lat, lon, contactName, contactPhone, isDefault)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &row, nil
}

func (s *Store) SoftDeleteAddress(ctx context.Context, addressID, userID string) error {
	res, err := s.accessor.Execer().ExecContext(ctx, `
		UPDATE addresses
		SET is_deleted = TRUE, is_default = FALSE, updated_at = NOW()
		WHERE address_id = $1::uuid AND user_id = $2::uuid AND is_deleted = FALSE
	`, addressID, userID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}
