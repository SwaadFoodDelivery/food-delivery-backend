package business

import (
	"context"
	"database/sql"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"food-delivery-backend/internal/constants"
	apperrors "food-delivery-backend/internal/errors"
	"food-delivery-backend/internal/services/users/models"
	"food-delivery-backend/internal/services/users/repository/repository"
)

type ProfileService interface {
	GetProfile(ctx context.Context, in models.GetProfileInput) (*models.GetProfileOutput, *models.ServiceError)
	UpdateProfile(ctx context.Context, in models.UpdateProfileInput) (*models.UpdateProfileOutput, *models.ServiceError)
	GetAddresses(ctx context.Context, in models.GetAddressesInput) (*models.GetAddressesOutput, *models.ServiceError)
	CreateAddress(ctx context.Context, in models.CreateAddressInput) (*models.CreateAddressOutput, *models.ServiceError)
	UpdateAddress(ctx context.Context, in models.UpdateAddressInput) (*models.UpdateAddressOutput, *models.ServiceError)
	DeleteAddress(ctx context.Context, in models.DeleteAddressInput) *models.ServiceError
}

// ─── GetProfile ───────────────────────────────────────────────────────────────

func (s *Service) GetProfile(ctx context.Context, in models.GetProfileInput) (*models.GetProfileOutput, *models.ServiceError) {
	userID := strings.TrimSpace(in.UserID)
	if userID == "" {
		return nil, badRequest(apperrors.CodeValidation, "user ID is required")
	}

	user, err := s.repo.GetUserProfileByID(ctx, userID)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, &models.ServiceError{StatusCode: http.StatusNotFound, Code: apperrors.CodeProfileNotFound, Message: "profile not found", Details: []string{}}
		}
		return nil, internalErr("failed to load profile")
	}

	out := buildProfileOutput(user)
	out.Profile = s.loadRoleProfile(ctx, userID, user.Role)
	return out, nil
}

// ─── UpdateProfile ────────────────────────────────────────────────────────────

func (s *Service) UpdateProfile(ctx context.Context, in models.UpdateProfileInput) (*models.UpdateProfileOutput, *models.ServiceError) {
	userID := strings.TrimSpace(in.UserID)
	if userID == "" {
		return nil, badRequest(apperrors.CodeValidation, "user ID is required")
	}

	user, err := s.repo.GetUserProfileByID(ctx, userID)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, &models.ServiceError{StatusCode: http.StatusNotFound, Code: apperrors.CodeProfileNotFound, Message: "profile not found", Details: []string{}}
		}
		return nil, internalErr("failed to load profile")
	}

	hasUpdate := in.Name != nil || in.Email != nil ||
		in.DateOfBirth != nil || in.Gender != nil ||
		in.IsAvailable != nil || in.CurrentCity != nil
	if !hasUpdate {
		return nil, badRequest(apperrors.CodeNothingToUpdate, "no updatable fields provided")
	}

	// Resolve new core values (keep existing when nil)
	newName := user.Name
	if in.Name != nil {
		newName = strings.TrimSpace(*in.Name)
	}

	newEmail := user.Email.String
	resetEmailVerified := false
	if in.Email != nil {
		candidate := strings.TrimSpace(*in.Email)
		if _, parseErr := mail.ParseAddress(candidate); parseErr != nil {
			return nil, badRequest(apperrors.CodeValidation, "invalid email address")
		}
		if !strings.EqualFold(candidate, user.Email.String) {
			exists, checkErr := s.repo.UserExistsByEmailRole(ctx, candidate, user.Role)
			if checkErr != nil {
				return nil, internalErr("failed to check email availability")
			}
			if exists {
				return nil, &models.ServiceError{StatusCode: http.StatusConflict, Code: apperrors.CodeEmailConflict, Message: "email is already in use", Details: []string{}}
			}
			newEmail = candidate
			resetEmailVerified = true
		}
	}

	err = s.repo.WithTx(ctx, func(tx repository.Repository) error {
		if txErr := tx.UpdateUserCore(ctx, userID, newName, newEmail, resetEmailVerified); txErr != nil {
			return txErr
		}

		if in.Role == constants.RoleClient && (in.DateOfBirth != nil || in.Gender != nil) {
			dob := ""
			if in.DateOfBirth != nil {
				dob = *in.DateOfBirth
			}
			gender := ""
			if in.Gender != nil {
				gender = *in.Gender
			}
			if txErr := tx.UpsertClientProfile(ctx, userID, dob, gender); txErr != nil {
				return txErr
			}
		}

		if in.Role == constants.RoleDriver && (in.IsAvailable != nil || in.CurrentCity != nil) {
			city := ""
			if in.CurrentCity != nil {
				city = *in.CurrentCity
			}
			if txErr := tx.UpsertDriverProfile(ctx, userID, in.IsAvailable, city); txErr != nil {
				return txErr
			}
		}

		return tx.InsertAuditLog(ctx, repository.AuditLogInput{
			ActorID:    userID,
			ActorRole:  in.Role,
			Action:     constants.AuditActionProfileUpdate,
			EntityType: constants.EntityTypeProfiles,
			EntityID:   userID,
		})
	})
	if err != nil {
		return nil, internalErr("failed to update profile")
	}

	updated, err := s.repo.GetUserProfileByID(ctx, userID)
	if err != nil {
		return nil, internalErr("failed to reload profile")
	}
	out := buildProfileOutput(updated)
	out.Profile = s.loadRoleProfile(ctx, userID, updated.Role)
	return out, nil
}

// ─── GetAddresses ─────────────────────────────────────────────────────────────

func (s *Service) GetAddresses(ctx context.Context, in models.GetAddressesInput) (*models.GetAddressesOutput, *models.ServiceError) {
	userID := strings.TrimSpace(in.UserID)
	if userID == "" {
		return nil, badRequest(apperrors.CodeValidation, "user ID is required")
	}

	page := in.Page
	if page < 1 {
		page = 1
	}
	limit := in.Limit
	if limit < 1 || limit > constants.MaxAddressListLimit {
		limit = constants.DefaultAddressListLimit
	}
	offset := (page - 1) * limit

	total, err := s.repo.CountAddresses(ctx, userID)
	if err != nil {
		return nil, internalErr("failed to count addresses")
	}

	rows, err := s.repo.ListAddresses(ctx, userID, offset, limit)
	if err != nil {
		return nil, internalErr("failed to list addresses")
	}

	addresses := make([]models.AddressItem, 0, len(rows))
	for _, row := range rows {
		addresses = append(addresses, addressRowToItem(row))
	}

	return &models.GetAddressesOutput{
		Addresses: addresses,
		Total:     total,
		Page:      page,
		Limit:     limit,
	}, nil
}

// ─── CreateAddress ────────────────────────────────────────────────────────────

func (s *Service) CreateAddress(ctx context.Context, in models.CreateAddressInput) (*models.CreateAddressOutput, *models.ServiceError) {
	userID := strings.TrimSpace(in.UserID)
	if userID == "" {
		return nil, badRequest(apperrors.CodeValidation, "user ID is required")
	}

	count, err := s.repo.CountActiveAddresses(ctx, userID)
	if err != nil {
		return nil, internalErr("failed to check address limit")
	}
	if count >= constants.MaxAddressesPerUser {
		return nil, &models.ServiceError{
			StatusCode: http.StatusUnprocessableEntity,
			Code:       apperrors.CodeAddressLimitExceeded,
			Message:    "maximum address limit reached",
			Details:    []string{},
		}
	}

	var row *models.AddressRow
	err = s.repo.WithTx(ctx, func(tx repository.Repository) error {
		if in.IsDefault {
			if txErr := tx.UnsetDefaultAddress(ctx, userID); txErr != nil {
				return txErr
			}
		}
		created, txErr := tx.CreateAddress(ctx, repository.CreateAddressInput{
			UserID: userID, Label: in.Label, Line1: in.Line1, Line2: in.Line2,
			Area: in.Area, City: in.City, State: in.State, Pincode: in.Pincode,
			Latitude: in.Latitude, Longitude: in.Longitude,
			ContactName: in.ContactName, ContactPhone: in.ContactPhone,
			IsDefault: in.IsDefault,
		})
		if txErr != nil {
			return txErr
		}
		row = created
		return tx.InsertAuditLog(ctx, repository.AuditLogInput{
			ActorID:    userID,
			ActorRole:  in.ActorRole,
			Action:     constants.AuditActionAddressCreate,
			EntityType: constants.EntityTypeAddresses,
			EntityID:   created.AddressID,
		})
	})
	if err != nil {
		return nil, internalErr("failed to create address")
	}

	return &models.CreateAddressOutput{Address: addressRowToItem(*row)}, nil
}

// ─── UpdateAddress ────────────────────────────────────────────────────────────

func (s *Service) UpdateAddress(ctx context.Context, in models.UpdateAddressInput) (*models.UpdateAddressOutput, *models.ServiceError) {
	userID := strings.TrimSpace(in.UserID)
	addressID := strings.TrimSpace(in.AddressID)
	if userID == "" || addressID == "" {
		return nil, badRequest(apperrors.CodeValidation, "user ID and address ID are required")
	}

	current, err := s.repo.FindAddressByIDAndUser(ctx, addressID, userID)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, &models.ServiceError{StatusCode: http.StatusNotFound, Code: apperrors.CodeAddressNotFound, Message: "address not found", Details: []string{}}
		}
		return nil, internalErr("failed to load address")
	}

	// Merge: apply non-nil updates over current values
	merged := repository.UpdateAddressInput{
		AddressID:    addressID,
		UserID:       userID,
		Label:        current.Label.String,
		Line1:        current.Line1,
		Line2:        current.Line2.String,
		Area:         current.Area.String,
		City:         current.City,
		State:        current.State,
		Pincode:      current.Pincode,
		Latitude:     nullFloat64ToPtr(current.Latitude),
		Longitude:    nullFloat64ToPtr(current.Longitude),
		ContactName:  current.ContactName.String,
		ContactPhone: current.ContactPhone.String,
		IsDefault:    current.IsDefault,
	}
	if in.Label != nil {
		merged.Label = *in.Label
	}
	if in.Line1 != nil {
		merged.Line1 = *in.Line1
	}
	if in.Line2 != nil {
		merged.Line2 = *in.Line2
	}
	if in.Area != nil {
		merged.Area = *in.Area
	}
	if in.City != nil {
		merged.City = *in.City
	}
	if in.State != nil {
		merged.State = *in.State
	}
	if in.Pincode != nil {
		merged.Pincode = *in.Pincode
	}
	if in.Latitude != nil {
		merged.Latitude = in.Latitude
	}
	if in.Longitude != nil {
		merged.Longitude = in.Longitude
	}
	if in.ContactName != nil {
		merged.ContactName = *in.ContactName
	}
	if in.ContactPhone != nil {
		merged.ContactPhone = *in.ContactPhone
	}
	if in.IsDefault != nil {
		merged.IsDefault = *in.IsDefault
	}

	var row *models.AddressRow
	err = s.repo.WithTx(ctx, func(tx repository.Repository) error {
		if merged.IsDefault && !current.IsDefault {
			if txErr := tx.UnsetDefaultAddress(ctx, userID); txErr != nil {
				return txErr
			}
		}
		updated, txErr := tx.UpdateAddress(ctx, merged)
		if txErr != nil {
			return txErr
		}
		row = updated
		return tx.InsertAuditLog(ctx, repository.AuditLogInput{
			ActorID:    userID,
			ActorRole:  in.ActorRole,
			Action:     constants.AuditActionAddressUpdate,
			EntityType: constants.EntityTypeAddresses,
			EntityID:   addressID,
		})
	})
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, &models.ServiceError{StatusCode: http.StatusNotFound, Code: apperrors.CodeAddressNotFound, Message: "address not found", Details: []string{}}
		}
		return nil, internalErr("failed to update address")
	}

	return &models.UpdateAddressOutput{Address: addressRowToItem(*row)}, nil
}

// ─── DeleteAddress ────────────────────────────────────────────────────────────

func (s *Service) DeleteAddress(ctx context.Context, in models.DeleteAddressInput) *models.ServiceError {
	userID := strings.TrimSpace(in.UserID)
	addressID := strings.TrimSpace(in.AddressID)
	if userID == "" || addressID == "" {
		return badRequest(apperrors.CodeValidation, "user ID and address ID are required")
	}

	err := s.repo.WithTx(ctx, func(tx repository.Repository) error {
		if txErr := tx.SoftDeleteAddress(ctx, addressID, userID); txErr != nil {
			return txErr
		}
		return tx.InsertAuditLog(ctx, repository.AuditLogInput{
			ActorID:    userID,
			ActorRole:  in.ActorRole,
			Action:     constants.AuditActionAddressDelete,
			EntityType: constants.EntityTypeAddresses,
			EntityID:   addressID,
		})
	})
	if err != nil {
		if repository.IsNotFound(err) {
			return &models.ServiceError{StatusCode: http.StatusNotFound, Code: apperrors.CodeAddressNotFound, Message: "address not found", Details: []string{}}
		}
		return internalErr("failed to delete address")
	}
	return nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func (s *Service) loadRoleProfile(ctx context.Context, userID, role string) any {
	switch role {
	case constants.RoleClient:
		row, err := s.repo.GetClientProfileByUserID(ctx, userID)
		if err != nil || row == nil {
			return &models.ClientProfileData{}
		}
		p := &models.ClientProfileData{}
		if row.DateOfBirth.Valid {
			p.DateOfBirth = row.DateOfBirth.Time.Format("2006-01-02")
		}
		if row.Gender.Valid {
			p.Gender = row.Gender.String
		}
		return p

	case constants.RoleDriver:
		row, err := s.repo.GetDriverProfileByUserID(ctx, userID)
		if err != nil || row == nil {
			return &models.DriverProfileData{}
		}
		p := &models.DriverProfileData{IsAvailable: row.IsAvailable}
		if row.CurrentCity.Valid {
			p.CurrentCity = row.CurrentCity.String
		}
		return p

	default:
		return nil
	}
}

func buildProfileOutput(u *models.UserProfileRow) *models.GetProfileOutput {
	out := &models.GetProfileOutput{
		UserID:             u.UserID,
		Name:               u.Name,
		Phone:              u.Phone,
		EmailVerified:      u.EmailVerified,
		PhoneVerified:      u.PhoneVerified,
		Role:               u.Role,
		AccountStatus:      u.AccountStatus,
		OnboardingComplete: u.OnboardingComplete,
		CreatedAt:          u.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:          u.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if u.Email.Valid {
		out.Email = u.Email.String
	}
	if u.ProfileImageS3Key.Valid {
		out.ProfileImageS3Key = u.ProfileImageS3Key.String
	}
	return out
}

func addressRowToItem(r models.AddressRow) models.AddressItem {
	item := models.AddressItem{
		AddressID: r.AddressID,
		Line1:     r.Line1,
		City:      r.City,
		State:     r.State,
		Pincode:   r.Pincode,
		IsDefault: r.IsDefault,
		CreatedAt: r.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: r.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if r.Label.Valid {
		item.Label = r.Label.String
	}
	if r.Line2.Valid {
		item.Line2 = r.Line2.String
	}
	if r.Area.Valid {
		item.Area = r.Area.String
	}
	if r.Latitude.Valid {
		v := r.Latitude.Float64
		item.Latitude = &v
	}
	if r.Longitude.Valid {
		v := r.Longitude.Float64
		item.Longitude = &v
	}
	if r.ContactName.Valid {
		item.ContactName = r.ContactName.String
	}
	if r.ContactPhone.Valid {
		item.ContactPhone = r.ContactPhone.String
	}
	return item
}

func nullFloat64ToPtr(n sql.NullFloat64) *float64 {
	if !n.Valid {
		return nil
	}
	v := n.Float64
	return &v
}
