package mapping

import (
	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/bag/crypt"
	"github.com/mondegor/go-components/mrauth/bag/jwt"
	bagsession "github.com/mondegor/go-components/mrauth/bag/session"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit/action"
	"github.com/mondegor/go-components/mrauth/service/authtoken"
	"github.com/mondegor/go-components/mrauth/service/realm"
	usecaseauth "github.com/mondegor/go-components/mrauth/usecase/auth"
	usecasesession "github.com/mondegor/go-components/mrauth/usecase/session"
	authcfg "github.com/mondegor/go-components/wire/mrauth/config"
)

// OptionUserRealmsToRealmRegistry - строит реестр соответствия id <-> name realm'ов.
func OptionUserRealmsToRealmRegistry(realms []authcfg.UserRealm) mrauth.RealmRegistry {
	mappedRealms := make([]realm.Realm, 0, len(realms))

	for _, item := range realms {
		mappedRealms = append(
			mappedRealms,
			realm.Realm{
				ID:   item.ID,
				Name: item.Name,
			},
		)
	}

	return realm.New(mappedRealms)
}

// OptionUserRealmsToStringRealms - извлекает имена realm'ов в виде списка строк.
func OptionUserRealmsToStringRealms(realms []authcfg.UserRealm) []string {
	mappedRealms := make([]string, 0, len(realms))

	for _, item := range realms {
		mappedRealms = append(mappedRealms, item.Name)
	}

	return mappedRealms
}

// OptionUserRealmsToSessionLimitRealms - строит лимиты одновременных сессий по видам
// пользователя (kind) внутри каждого realm на основе конфигурации UserKind.SessionMax.
func OptionUserRealmsToSessionLimitRealms(realms []authcfg.UserRealm) []usecasesession.LimitRealm {
	mappedRealms := make([]usecasesession.LimitRealm, 0, len(realms))

	for _, item := range realms {
		kindLimits := make([]usecasesession.UserKindLimit, 0, len(item.UserKinds))
		for _, kind := range item.UserKinds {
			kindLimits = append(
				kindLimits,
				usecasesession.UserKindLimit{
					Kind:       kind.Name,
					SessionMax: int(kind.SessionMax),
				},
			)
		}

		mappedRealms = append(
			mappedRealms,
			usecasesession.LimitRealm{
				ID:         item.ID,
				KindLimits: kindLimits,
			},
		)
	}

	return mappedRealms
}

// OptionUserRealmsToConfirmCreateUserRealms - строит realm'ы регистрации пользователей,
// пропуская realm'ы без поддержки регистрации.
func OptionUserRealmsToConfirmCreateUserRealms(realms []authcfg.UserRealm) []usecaseauth.CreateUserRealm {
	mappedRealms := make([]usecaseauth.CreateUserRealm, 0, len(realms))

	for _, item := range realms {
		// добавляются только области пользователей, с поддержкой регистрации
		if item.RegisterUserKind == "none" {
			continue
		}

		mappedRealms = append(
			mappedRealms,
			usecaseauth.CreateUserRealm{
				Name: item.Name,
				Operation: unit.NewCreateUser(
					item.Name,
					item.RegisterUserKind,
					crypt.NewSecretGenerator(int(item.AuthToken.Length)),
					crypt.NewSecretGenerator(int(item.OperationConfirm.CodeLength)),
					action.WithMaxAttempts(int16(item.OperationConfirm.SendByEmail.MaxAttempts)),
					action.WithMaxResends(int16(item.OperationConfirm.SendByEmail.MaxResends)),
					action.WithMinResendTime(item.OperationConfirm.SendByEmail.MinResendTime),
					action.WithExpiry(item.OperationConfirm.SessionExpiry),
				),
			},
		)
	}

	return mappedRealms
}

// OptionUserRealmsToConfirmCreateSessionRealms - строит realm'ы авторизации (создания сессии)
// с подтверждением по email и телефону.
func OptionUserRealmsToConfirmCreateSessionRealms(realms []authcfg.UserRealm) []usecaseauth.CreateSessionRealm {
	mappedRealms := make([]usecaseauth.CreateSessionRealm, 0, len(realms))

	for _, item := range realms {
		mappedRealms = append(
			mappedRealms,
			usecaseauth.CreateSessionRealm{
				Name: item.Name,
				Operation: unit.NewAuthorizeUser(
					crypt.NewSecretGenerator(int(item.AuthToken.Length)),
					crypt.NewSecretGenerator(int(item.OperationConfirm.CodeLength)),
					unit.WithAuthorizeUserConfirmByEmailOpts(
						action.WithMaxAttempts(int16(item.OperationConfirm.SendByEmail.MaxAttempts)),
						action.WithMaxResends(int16(item.OperationConfirm.SendByEmail.MaxResends)),
						action.WithMinResendTime(item.OperationConfirm.SendByEmail.MinResendTime),
						action.WithExpiry(item.OperationConfirm.SessionExpiry),
					),
					unit.WithAuthorizeUserConfirmByPhoneOpts(
						action.WithMaxAttempts(int16(item.OperationConfirm.SendByPhone.MaxAttempts)),
						action.WithMaxResends(int16(item.OperationConfirm.SendByPhone.MaxResends)),
						action.WithMinResendTime(item.OperationConfirm.SendByPhone.MinResendTime),
						action.WithExpiry(item.OperationConfirm.SessionExpiry),
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
func OptionUserRealmsToCreateSessionRealms(realms []authcfg.UserRealm, jwtConfig authcfg.JWT) []authtoken.Realm {
	mappedRealms := make([]authtoken.Realm, 0, len(realms))

	for _, item := range realms {
		var tokenIssuer mrauth.TokenIssuer

		switch item.AuthToken.AccessType {
		case "jwt":
			tokenIssuer = jwt.NewTokenIssuer(
				crypt.NewSecretGenerator(int(item.AuthToken.Length)),
				item.AuthToken.AccessExpiry,
				item.AuthToken.RefreshExpiry,
				jwtConfig.Issuer,
				jwtConfig.SigningKey,
			)
		default:
			tokenIssuer = bagsession.NewTokenIssuer(
				crypt.NewSecretGenerator(int(item.AuthToken.Length)),
				item.AuthToken.AccessExpiry,
				item.AuthToken.RefreshExpiry,
			)
		}

		mappedRealms = append(
			mappedRealms,
			authtoken.Realm{
				ID:          item.ID,
				TokenIssuer: tokenIssuer,
			},
		)
	}

	return mappedRealms
}
