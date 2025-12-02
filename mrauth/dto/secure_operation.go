package dto

type (
	// CreateUserOperation - comment struct.
	CreateUserOperation struct {
		Realm    string `json:"realm"`
		UserKind string `json:"userKind"`
		LangCode string `json:"langCode"`
		Email    string `json:"email"`
	}

	// AuthorizeUserOperation - comment struct.
	AuthorizeUserOperation struct {
		Realm    string `json:"realm"`
		LangCode string `json:"langCode"`
	}

	// OperationWithUserEmail - comment struct.
	OperationWithUserEmail struct {
		Email string `json:"email"`
	}

	// ChangeTotpOperation - comment struct.
	ChangeTotpOperation OperationWithUserEmail // ???????????????????????????????

	// Disable2faOperation - comment struct.
	Disable2faOperation OperationWithUserEmail

	// ChangeEmailOperation - comment struct.
	ChangeEmailOperation struct {
		NewEmail      string `json:"newEmail"`
		NotifyByEmail string `json:"email"`
	}

	// ChangePasswordOperation - comment struct.
	ChangePasswordOperation struct {
		NewPassword   string `json:"newPassword"`
		NotifyByEmail string `json:"email"`
	}

	// ChangePhoneOperation - comment struct.
	ChangePhoneOperation struct {
		NewPhone      uint64 `json:"newPhone"`
		NotifyByEmail string `json:"email"`
	}
)
