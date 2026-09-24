package domain

import "time"

type Role string

const (
	AdminRole Role = "admin"
	UserRole  Role = "user"
)

type User struct {
	ID          string
	Username    string
	DisplayName string
	Role        Role
	CreatedAt   time.Time
}
