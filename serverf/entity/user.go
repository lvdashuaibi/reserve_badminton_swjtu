package entity

type RegisterData struct {
	Username string `json:"name"`
	Password string `json:"password"`
	Token    string `json:"token"`
}

type LoginData struct {
	Username string `json:"name"`
	Password string `json:"password"`
}

type ChangePasswordData struct {
	NewPassword string `json:"new_password"`
}
