package initing

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/mraccess"
	"github.com/mondegor/go-sysmess/mrlog"
	"github.com/mondegor/go-sysmess/mrstorage"

	"github.com/mondegor/go-components/wire/mrauth"
	authcfg "github.com/mondegor/go-components/wire/mrauth/config"
)

// InitUserProviders - создаёт объект mraccess.UserProvider с указанными настройками.
func InitUserProviders(
	logger mrlog.Logger,
	dbConnManager mrstorage.DBConnManager,
	userGroupRights mraccess.RightsGetter,
	userRealms []authcfg.UserRealm,
	testUser authcfg.TestUser,
	jwtSecret string,
	authTokensTableName string,
) (realm2provider map[string]mraccess.UserProvider) {
	if len(userRealms) == 0 {
		mrlog.Error(logger, "Auth: AccessControl.Realms is empty")

		return nil
	}

	realm2provider = make(map[string]mraccess.UserProvider, len(userRealms))
	realms := make([]authcfg.UserRealm, 0, len(userRealms))
	domain2realms := make(map[string][]authcfg.UserRealm, len(userRealms))

	for _, realm := range userRealms {
		switch realm.AuthToken.AccessType {
		// если метод аутентификации указан JWT, то будут приниматься от клиентов JWT токены
		case "jwt":
			mrlog.Debug(logger, fmt.Sprintf("Auth.JWT: realm=%s, secret=%s", realm.Name, jwtSecret))

		// стандартный режим: будут приниматься от клиентов токены, хранящиеся в таблице authTokensTableName
		default:
			mrlog.Debug(logger, fmt.Sprintf("Auth.Session: realm=%s, table=%s", realm.Name, authTokensTableName))
		}

		domain := realm.Name
		if val, _, ok := strings.Cut(realm.Name, "/"); ok {
			domain = val
		}

		realms = append(realms, realm)
		domain2realms[domain] = append(domain2realms[domain], realm)

		realm2provider[realm.Name] = createUserProviderByTokenType(
			logger,
			dbConnManager,
			userGroupRights,
			testUser,
			realm.AuthToken.AccessType,
			jwtSecret,
			[]string{realm.Name},
			authTokensTableName,
		)
	}

	realm2provider["*"] = createUserProviderGroup(
		logger,
		dbConnManager,
		userGroupRights,
		testUser,
		jwtSecret,
		realms,
		authTokensTableName,
	)

	for domain, domainRealms := range domain2realms {
		realm2provider[domain+"/*"] = createUserProviderGroup(
			logger,
			dbConnManager,
			userGroupRights,
			testUser,
			jwtSecret,
			domainRealms,
			authTokensTableName,
		)
	}

	return realm2provider
}

func createUserProviderGroup(
	logger mrlog.Logger,
	dbConnManager mrstorage.DBConnManager,
	userGroupRights mraccess.RightsGetter,
	testUser authcfg.TestUser,
	jwtSecret string,
	userRealms []authcfg.UserRealm,
	authTokensTableName string,
) mraccess.UserProvider {
	type2realms := make(map[string][]string, len(userRealms))

	for _, realm := range userRealms {
		type2realms[realm.AuthToken.AccessType] = append(type2realms[realm.AuthToken.AccessType], realm.Name)
	}

	providers := make([]mraccess.TypedUserProvider, 0, len(type2realms))

	for tokenType, realms := range type2realms {
		providers = append(
			providers,
			mraccess.TypedUserProvider{
				Type: tokenType,
				Value: createUserProviderByTokenType(
					logger,
					dbConnManager,
					userGroupRights,
					testUser,
					tokenType,
					jwtSecret,
					realms,
					authTokensTableName,
				),
			},
		)
	}

	if len(providers) == 1 {
		return providers[0].Value
	}

	return mrauth.NewUserProvider(providers...)
}

func createUserProviderByTokenType(
	logger mrlog.Logger,
	dbConnManager mrstorage.DBConnManager,
	userGroupRights mraccess.RightsGetter,
	testUser authcfg.TestUser,
	tokenType string,
	jwtSecret string,
	allowedRealms []string,
	authTokensTableName string,
) mraccess.UserProvider {
	// если указан тестовый пользователь, то при успешной проверки realm будет возвращаться тестовый провайдер
	if testUser.ID != "" {
		for _, realm := range allowedRealms {
			if testUser.Realm != realm {
				continue
			}

			mrlog.Debug(
				logger,
				"Auth.Debug",
				"userId", testUser.ID,
				"realm", testUser.Realm,
				"kind", testUser.Kind,
				"lang", testUser.LangCode,
			)

			return mraccess.NewOneUserProvider(
				mraccess.NewUser(
					uuid.MustParse(testUser.ID),
					testUser.Realm+"/"+testUser.Kind,
					testUser.LangCode,
					userGroupRights,
				),
			)
		}
	}

	// JWT режим: принимаются от клиентов JWT токены
	if tokenType == "jwt" {
		return mrauth.NewUserProviderJWT(
			userGroupRights,
			jwtSecret,
			allowedRealms,
		)
	}

	// Session режим: принимаются от клиентов токены, хранящиеся в таблице accessTokenTableName
	return mrauth.NewUserProviderSession(
		dbConnManager,
		userGroupRights,
		authTokensTableName,
		allowedRealms,
	)
}
