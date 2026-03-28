package dto

type (
	// WaitingConfirmOperation struct {
	// 	Token             string
	// 	ConfirmMethod     confirmmethod.Enum
	// 	RemainingAttempts int16
	// 	RemainingResends  int16
	// 	ResendsAt         time.Time
	// 	ExpiresAt         time.Time
	// 	DebugInfo         string
	// }.

	// CreateUserOperation - comment struct.
	CreateUserOperation struct {
		Realm    string `json:"realm"`
		UserKind string `json:"user_kind"`
		LangCode string `json:"lang_code"`
		Email    string `json:"email"`
	}

	// AuthorizeUserOperation - comment struct.
	AuthorizeUserOperation struct {
		Realm    string `json:"realm"`
		LangCode string `json:"lang_code"`
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
		NewEmail      string `json:"new_email"`
		NotifyByEmail string `json:"email"`
	}

	// ChangePasswordOperation - comment struct.
	ChangePasswordOperation struct {
		NewPassword   string `json:"new_password"`
		NotifyByEmail string `json:"email"`
	}

	// ChangePhoneOperation - comment struct.
	ChangePhoneOperation struct {
		NewPhone      uint64 `json:"new_phone"`
		NotifyByEmail string `json:"email"`
	}
)
