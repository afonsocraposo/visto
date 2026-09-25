package domain

import "time"

type Role string

const (
	AdminRole Role = "admin"
	UserRole  Role = "user"
)

type User struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"name"`
	Role        Role      `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
}
