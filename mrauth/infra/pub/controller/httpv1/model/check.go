package model

type (
	// CheckLoginRequest - запрос на проверку свободен ли указанный емаил/телефон.
	CheckLoginRequest = AuthorizeUserRequest

	// CalcPasswordStrengthRequest - запрос на проверку надёжности указанного пароля.
	CalcPasswordStrengthRequest struct {
		Password string `json:"password" validate:"required,min=8,max=32,tag_password"`
	}

	// CalcPasswordStrengthResponse - информация о надёжности пароля.
	CalcPasswordStrengthResponse struct {
		Strength string `json:"strength"`
	}
)
