package dto

import "github.com/mondegor/go-components/mrauth/entity"

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

	// SecureOperationLog - comment struct.
	SecureOperationLog = entity.SecureOperationLog

	// OperationWithUserEmail - компонент для извлечения настроек, которые хранятся в хранилище данных.
	OperationWithUserEmail struct {
		Email string `json:"email"`
	}

	// ChangeTotpOperation - comment struct.
	ChangeTotpOperation OperationWithUserEmail // ???????????????????????????????

	// Disable2faOperation - comment struct.
	Disable2faOperation OperationWithUserEmail

	// ChangeEmailOperation - компонент для извлечения настроек, которые хранятся в хранилище данных.
	ChangeEmailOperation struct {
		NewEmail      string `json:"newEmail"`
		NotifyByEmail string `json:"email"`
	}

	// ChangePasswordOperation - компонент для извлечения настроек, которые хранятся в хранилище данных.
	ChangePasswordOperation struct {
		NewPassword   string `json:"newPassword"`
		NotifyByEmail string `json:"email"`
	}

	// ChangePhoneOperation - компонент для извлечения настроек, которые хранятся в хранилище данных.
	ChangePhoneOperation struct {
		NewPhone      uint64 `json:"newPhone"`
		NotifyByEmail string `json:"email"`
	}
)
