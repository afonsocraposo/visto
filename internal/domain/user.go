package domain

import "time"

type Role string

const (
	AdminRole Role = "admin"
	UserRole  Role = "user"
)

type User struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Role        Role      `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
}
