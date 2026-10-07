package domain

type Role string

const (
	RoleBuyer      Role = "buyer"
	RoleOrganizer  Role = "organizer"
	RoleController Role = "controller"
	RoleAdmin      Role = "admin"
)
