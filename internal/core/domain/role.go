package domain

type Role string

const (
	RoleBuyer      Role = "buyer"
	RoleOrganizer  Role = "organizer"
	RoleVenueOwner Role = "ownerVenue"
	RoleController Role = "controller"
	RoleAdmin      Role = "admin"
)
