package models

import (
	"database/sql"
	"time"
)

// ─── DB row types ────────────────────────────────────────────────────────────

type UserProfileRow struct {
	UserID             string         `db:"user_id"`
	Name               string         `db:"name"`
	Phone              string         `db:"phone"`
	Email              sql.NullString `db:"email"`
	EmailVerified      bool           `db:"email_verified"`
	PhoneVerified      bool           `db:"phone_verified"`
	Role               string         `db:"role"`
	AccountStatus      string         `db:"account_status"`
	OnboardingComplete bool           `db:"onboarding_complete"`
	ProfileImageS3Key  sql.NullString `db:"profile_image_s3_key"`
	CreatedAt          time.Time      `db:"created_at"`
	UpdatedAt          time.Time      `db:"updated_at"`
}

type ClientProfileRow struct {
	UserID      string         `db:"user_id"`
	DateOfBirth sql.NullTime   `db:"date_of_birth"`
	Gender      sql.NullString `db:"gender"`
	CreatedAt   time.Time      `db:"created_at"`
	UpdatedAt   time.Time      `db:"updated_at"`
}

type DriverProfileRow struct {
	UserID      string         `db:"user_id"`
	IsAvailable bool           `db:"is_available"`
	CurrentCity sql.NullString `db:"current_city"`
	CreatedAt   time.Time      `db:"created_at"`
	UpdatedAt   time.Time      `db:"updated_at"`
}

type AddressRow struct {
	AddressID    string          `db:"address_id"`
	UserID       string          `db:"user_id"`
	Label        sql.NullString  `db:"label"`
	Line1        string          `db:"line1"`
	Line2        sql.NullString  `db:"line2"`
	Area         sql.NullString  `db:"area"`
	City         string          `db:"city"`
	State        string          `db:"state"`
	Pincode      string          `db:"pincode"`
	Latitude     sql.NullFloat64 `db:"latitude"`
	Longitude    sql.NullFloat64 `db:"longitude"`
	ContactName  sql.NullString  `db:"contact_name"`
	ContactPhone sql.NullString  `db:"contact_phone"`
	IsDefault    bool            `db:"is_default"`
	IsDeleted    bool            `db:"is_deleted"`
	CreatedAt    time.Time       `db:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at"`
}

// ─── Business input / output ─────────────────────────────────────────────────

type GetProfileInput struct {
	UserID string
}

type ClientProfileData struct {
	DateOfBirth string `json:"date_of_birth,omitempty"`
	Gender      string `json:"gender,omitempty"`
}

type DriverProfileData struct {
	IsAvailable bool   `json:"is_available"`
	CurrentCity string `json:"current_city,omitempty"`
}

type GetProfileOutput struct {
	UserID             string `json:"user_id"`
	Name               string `json:"name"`
	Phone              string `json:"phone"`
	Email              string `json:"email,omitempty"`
	EmailVerified      bool   `json:"email_verified"`
	PhoneVerified      bool   `json:"phone_verified"`
	Role               string `json:"role"`
	AccountStatus      string `json:"account_status"`
	OnboardingComplete bool   `json:"onboarding_complete"`
	ProfileImageS3Key  string `json:"profile_image_s3_key,omitempty"`
	// Profile is one of ClientProfileData, DriverProfileData, or nil for restaurant roles
	Profile   any    `json:"profile,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type UpdateProfileInput struct {
	UserID      string
	Role        string
	Name        *string
	Email       *string
	DateOfBirth *string // YYYY-MM-DD, client only
	Gender      *string // client only
	IsAvailable *bool   // driver only
	CurrentCity *string // driver only
}

type UpdateProfileOutput = GetProfileOutput

type AddressItem struct {
	AddressID    string   `json:"address_id"`
	Label        string   `json:"label,omitempty"`
	Line1        string   `json:"line1"`
	Line2        string   `json:"line2,omitempty"`
	Area         string   `json:"area,omitempty"`
	City         string   `json:"city"`
	State        string   `json:"state"`
	Pincode      string   `json:"pincode"`
	Latitude     *float64 `json:"latitude,omitempty"`
	Longitude    *float64 `json:"longitude,omitempty"`
	ContactName  string   `json:"contact_name,omitempty"`
	ContactPhone string   `json:"contact_phone,omitempty"`
	IsDefault    bool     `json:"is_default"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
}

type GetAddressesInput struct {
	UserID string
	Page   int
	Limit  int
}

type GetAddressesOutput struct {
	Addresses []AddressItem `json:"addresses"`
	Total     int           `json:"total"`
	Page      int           `json:"page"`
	Limit     int           `json:"limit"`
}

type CreateAddressInput struct {
	UserID       string
	ActorRole    string
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

type CreateAddressOutput struct {
	Address AddressItem `json:"address"`
}

type UpdateAddressInput struct {
	UserID       string
	ActorRole    string
	AddressID    string
	Label        *string
	Line1        *string
	Line2        *string
	Area         *string
	City         *string
	State        *string
	Pincode      *string
	Latitude     *float64
	Longitude    *float64
	ContactName  *string
	ContactPhone *string
	IsDefault    *bool
}

type UpdateAddressOutput struct {
	Address AddressItem `json:"address"`
}

type DeleteAddressInput struct {
	UserID    string
	ActorRole string
	AddressID string
}

// ─── HTTP request structs ─────────────────────────────────────────────────────

type UpdateProfileRequest struct {
	Name        *string `json:"name"`
	Email       *string `json:"email"`
	DateOfBirth *string `json:"date_of_birth"`
	Gender      *string `json:"gender"`
	IsAvailable *bool   `json:"is_available"`
	CurrentCity *string `json:"current_city"`
}

type CreateAddressRequest struct {
	Label        string   `json:"label"`
	Line1        string   `json:"line1"`
	Line2        string   `json:"line2"`
	Area         string   `json:"area"`
	City         string   `json:"city"`
	State        string   `json:"state"`
	Pincode      string   `json:"pincode"`
	Latitude     *float64 `json:"latitude"`
	Longitude    *float64 `json:"longitude"`
	ContactName  string   `json:"contact_name"`
	ContactPhone string   `json:"contact_phone"`
	IsDefault    bool     `json:"is_default"`
}

type UpdateAddressRequest struct {
	Label        *string  `json:"label"`
	Line1        *string  `json:"line1"`
	Line2        *string  `json:"line2"`
	Area         *string  `json:"area"`
	City         *string  `json:"city"`
	State        *string  `json:"state"`
	Pincode      *string  `json:"pincode"`
	Latitude     *float64 `json:"latitude"`
	Longitude    *float64 `json:"longitude"`
	ContactName  *string  `json:"contact_name"`
	ContactPhone *string  `json:"contact_phone"`
	IsDefault    *bool    `json:"is_default"`
}
