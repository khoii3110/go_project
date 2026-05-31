package user

import "time"

const (
	RoleManager = "manager"
	RoleMember  = "member"
)

type User struct {
	UserID       string    `json:"userId"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	Role         string    `json:"role"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}
