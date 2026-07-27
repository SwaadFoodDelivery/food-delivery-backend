package repository

import (
	"context"

	"food-delivery-backend/internal/services/users/models"
)

type UpdateAddressInput struct {
	AddressID    string
	UserID       string
	Label        string
	Line1        string
	Line2        string
	Area         string
	City         string
	State        string
	Pincode      string
	Latitude     *float64
	Longitude    *float64
	ContactName  string
	ContactPhone string
	IsDefault    bool
}

type CreateAddressInput struct {
	UserID       string
	Label        string
	Line1        string
	Line2        string
	Area         string
	City         string
	State        string
	Pincode      string
	Latitude     *float64
	Longitude    *float64
	ContactName  string
	ContactPhone string
	IsDefault    bool
}

func (r *repo) GetUserProfileByID(ctx context.Context, userID string) (*models.UserProfileRow, error) {
	return r.pg.GetUserProfileByID(ctx, userID)
}

func (r *repo) GetClientProfileByUserID(ctx context.Context, userID string) (*models.ClientProfileRow, error) {
	return r.pg.GetClientProfileByUserID(ctx, userID)
}

func (r *repo) GetDriverProfileByUserID(ctx context.Context, userID string) (*models.DriverProfileRow, error) {
	return r.pg.GetDriverProfileByUserID(ctx, userID)
}

func (r *repo) UpdateUserCore(ctx context.Context, userID, name string) error {
	return r.pg.UpdateUserCore(ctx, userID, name)
}

func (r *repo) UpdateUserEmail(ctx context.Context, userID, email string) error {
	return r.pg.UpdateUserEmail(ctx, userID, email)
}

func (r *repo) UpsertClientProfile(ctx context.Context, userID, dateOfBirth, gender string) error {
	return r.pg.UpsertClientProfile(ctx, userID, dateOfBirth, gender)
}

func (r *repo) UpsertDriverProfile(ctx context.Context, userID string, isAvailable *bool, currentCity string) error {
	return r.pg.UpsertDriverProfile(ctx, userID, isAvailable, currentCity)
}

func (r *repo) CountActiveAddresses(ctx context.Context, userID string) (int, error) {
	return r.pg.CountActiveAddresses(ctx, userID)
}

func (r *repo) ListAddresses(ctx context.Context, userID string, offset, limit int) ([]models.AddressRow, error) {
	return r.pg.ListAddresses(ctx, userID, offset, limit)
}

func (r *repo) CountAddresses(ctx context.Context, userID string) (int, error) {
	return r.pg.CountAddresses(ctx, userID)
}

func (r *repo) UnsetDefaultAddress(ctx context.Context, userID string) error {
	return r.pg.UnsetDefaultAddress(ctx, userID)
}

func (r *repo) CreateAddress(ctx context.Context, in CreateAddressInput) (*models.AddressRow, error) {
	return r.pg.CreateAddress(ctx,
		in.UserID, in.Label, in.Line1, in.Line2, in.Area,
		in.City, in.State, in.Pincode,
		in.Latitude, in.Longitude,
		in.ContactName, in.ContactPhone, in.IsDefault,
	)
}

func (r *repo) FindAddressByIDAndUser(ctx context.Context, addressID, userID string) (*models.AddressRow, error) {
	return r.pg.FindAddressByIDAndUser(ctx, addressID, userID)
}

func (r *repo) UpdateAddress(ctx context.Context, in UpdateAddressInput) (*models.AddressRow, error) {
	return r.pg.UpdateAddress(ctx,
		in.AddressID, in.UserID, in.Label, in.Line1, in.Line2, in.Area,
		in.City, in.State, in.Pincode,
		in.Latitude, in.Longitude,
		in.ContactName, in.ContactPhone, in.IsDefault,
	)
}

func (r *repo) SoftDeleteAddress(ctx context.Context, addressID, userID string) error {
	return r.pg.SoftDeleteAddress(ctx, addressID, userID)
}
