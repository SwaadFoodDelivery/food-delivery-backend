package constants

const (
	AuditActionProfileUpdate = "profile_update"
	AuditActionAddressCreate = "address_create"
	AuditActionAddressUpdate = "address_update"
	AuditActionAddressDelete = "address_delete"

	EntityTypeAddresses = "addresses"
	EntityTypeProfiles  = "profiles"

	MaxAddressesPerUser     = 10
	DefaultAddressListLimit = 20
	MaxAddressListLimit     = 100
)
