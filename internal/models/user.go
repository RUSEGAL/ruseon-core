package models

// Role represents a user authorization role for Role-Based Access Control (RBAC).
type Role string

const (
	// RoleAdmin grants full administrative access (managing cameras, users, storage backups, settings).
	RoleAdmin Role = "admin"
	// RoleOperator grants camera streaming, PTZ, metadata tagging, and playback permissions.
	RoleOperator Role = "operator"
	// RoleViewer grants read-only live media playback access.
	RoleViewer Role = "viewer"
	// RoleService grants programmatic access for AI microservices and automated workers.
	RoleService Role = "service"
)

// User represents an authenticated account record in RUSEON Core.
type User struct {
	// Username is the unique account name.
	Username string `json:"username"`
	// PasswordHash contains the bcrypt-hashed password string.
	PasswordHash string `json:"password_hash"`
	// Role defines the authorization privileges assigned to the user.
	Role Role `json:"role"`
}
