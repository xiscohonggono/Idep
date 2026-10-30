package domain

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type UserRepository interface {
	GetByID(id string) (*User, error)
}