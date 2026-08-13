package models

// Role represents a CE role for RBAC
type Role string

const (
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
	RoleViewer   Role = "viewer"
	RoleService  Role = "service"
)

// User represents an authenticated user in RUSEON
type User struct {
	Username     string `json:"username"`
	PasswordHash string `json:"password_hash"`
	Role         Role   `json:"role"`
}
