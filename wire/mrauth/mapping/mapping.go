package mapping

import (
	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/bag/crypt"
	"github.com/mondegor/go-components/mrauth/bag/jwt"
	bagsession "github.com/mondegor/go-components/mrauth/bag/session"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit/action"
	"github.com/mondegor/go-components/mrauth/service/session"
	usecaseauth "github.com/mondegor/go-components/mrauth/usecase/auth"
	usecasesession "github.com/mondegor/go-components/mrauth/usecase/session"
	auth "github.com/mondegor/go-components/wire/mrauth/config"
)

// OptionUserRealmsToStringRealms - извлекает имена realm'ов в виде списка строк.
func OptionUserRealmsToStringRealms(realms []auth.UserRealm) []string {
	mappedRealms := make([]string, 0, len(realms))

	for _, realm := range realms {
		mappedRealms = append(mappedRealms, realm.Name)
	}

	return mappedRealms
}

// OptionUserRealmsToSessionLimitRealms - строит лимиты одновременных сессий по видам
// пользователя (kind) внутри каждого realm на основе конфигурации UserKind.SessionMax.
func OptionUserRealmsToSessionLimitRealms(realms []auth.UserRealm) []usecasesession.LimitRealm {
	mappedRealms := make([]usecasesession.LimitRealm, 0, len(realms))

	for _, realm := range realms {
		kindLimits := make([]usecasesession.UserKindLimit, 0, len(realm.UserKinds))
		for _, kind := range realm.UserKinds {
			kindLimits = append(
				kindLimits,
				usecasesession.UserKindLimit{
					Kind:       kind.Name,
					SessionMax: kind.SessionMax,
				},
			)
		}

		mappedRealms = append(
			mappedRealms,
			usecasesession.LimitRealm{
				Name:       realm.Name,
				KindLimits: kindLimits,
			},
		)
	}

	return mappedRealms
}

// OptionUserRealmsToConfirmCreateUserRealms - строит realm'ы регистрации пользователей,
// пропуская realm'ы без поддержки регистрации.
func OptionUserRealmsToConfirmCreateUserRealms(realms []auth.UserRealm) []usecaseauth.CreateUserRealm {
	mappedRealms := make([]usecaseauth.CreateUserRealm, 0, len(realms))

	for _, realm := range realms {
		// добавляются только области пользователей, с поддержкой регистрации
		if realm.RegisterUserKind == "none" {
			continue
		}

		mappedRealms = append(
			mappedRealms,
			usecaseauth.CreateUserRealm{
				Name: realm.Name,
				Operation: unit.NewCreateUser(
					realm.Name,
					realm.RegisterUserKind,
					crypt.NewTokenGenerator(int(realm.AuthToken.Length)),
					crypt.NewCodeGenerator(int(realm.OperationConfirm.CodeLength)),
					action.WithMaxAttempts(int16(realm.OperationConfirm.SendByEmail.MaxAttempts)),
					action.WithMaxResends(int16(realm.OperationConfirm.SendByEmail.MaxResends)),
					action.WithMinResendTime(realm.OperationConfirm.SendByEmail.MinResendTime),
					action.WithExpiry(realm.OperationConfirm.SessionExpiry),
				),
			},
		)
	}

	return mappedRealms
}

// OptionUserRealmsToConfirmCreateSessionRealms - строит realm'ы авторизации (создания сессии)
// с подтверждением по email и телефону.
func OptionUserRealmsToConfirmCreateSessionRealms(realms []auth.UserRealm) []usecaseauth.CreateSessionRealm {
	mappedRealms := make([]usecaseauth.CreateSessionRealm, 0, len(realms))

	for _, realm := range realms {
		mappedRealms = append(
			mappedRealms,
			usecaseauth.CreateSessionRealm{
				Name: realm.Name,
				Operation: unit.NewAuthorizeUser(
					crypt.NewTokenGenerator(int(realm.AuthToken.Length)),
					crypt.NewCodeGenerator(int(realm.OperationConfirm.CodeLength)),
					unit.WithAuthorizeUserConfirmByEmailOpts(
						action.WithMaxAttempts(int16(realm.OperationConfirm.SendByEmail.MaxAttempts)),
						action.WithMaxResends(int16(realm.OperationConfirm.SendByEmail.MaxResends)),
						action.WithMinResendTime(realm.OperationConfirm.SendByEmail.MinResendTime),
						action.WithExpiry(realm.OperationConfirm.SessionExpiry),
					),
					unit.WithAuthorizeUserConfirmByPhoneOpts(
						action.WithMaxAttempts(int16(realm.OperationConfirm.SendByPhone.MaxAttempts)),
						action.WithMaxResends(int16(realm.OperationConfirm.SendByPhone.MaxResends)),
						action.WithMinResendTime(realm.OperationConfirm.SendByPhone.MinResendTime),
						action.WithExpiry(realm.OperationConfirm.SessionExpiry),
					),
					unit.WithAuthorizeUserConfirmPhoneByEmail(true),
				),
			},
		)
	}

	return mappedRealms
}

// OptionUserRealmsToCreateSessionRealms - строит realm'ы выпуска токенов сессии, выбирая
// issuer по типу токена realm'а (jwt либо обычный session-токен).
func OptionUserRealmsToCreateSessionRealms(realms []auth.UserRealm, jwtConfig auth.JWT) []session.AuthTokenRealm {
	mappedRealms := make([]session.AuthTokenRealm, 0, len(realms))

	for _, realm := range realms {
		var tokenIssuer mrauth.TokenIssuer

		switch realm.AuthToken.AccessType {
		case "jwt":
			tokenIssuer = jwt.NewTokenIssuer(
				crypt.NewTokenGenerator(int(realm.AuthToken.Length)),
				realm.AuthToken.AccessExpiry,
				realm.AuthToken.RefreshExpiry,
				jwtConfig.Issuer,
				jwtConfig.SigningKey,
			)
		default:
			tokenIssuer = bagsession.NewTokenIssuer(
				crypt.NewTokenGenerator(int(realm.AuthToken.Length)),
				realm.AuthToken.AccessExpiry,
				realm.AuthToken.RefreshExpiry,
			)
		}

		mappedRealms = append(
			mappedRealms,
			session.AuthTokenRealm{
				Name:        realm.Name,
				TokenIssuer: tokenIssuer,
			},
		)
	}

	return mappedRealms
}
