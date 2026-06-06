package initing

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mraccess"
	"github.com/mondegor/go-sysmess/mrlog"

	"github.com/mondegor/go-components/wire/mrauth"
	authcfg "github.com/mondegor/go-components/wire/mrauth/config"
)

const (
	accessTokenTableName  = "printshop_auth.auth_tokens" //nolint:gosec
	accessTokenPrimaryKey = "token_name"
)

// InitUserProviders - создаёт объект mraccess.UserProvider с указанными настройками.
func InitUserProviders(
	logger mrlog.Logger,
	dbConnManager mrstorage.DBConnManager,
	userGroups mraccess.RightsGetter,
	userRealms []authcfg.UserRealm,
	testUser authcfg.TestUser,
	jwtSecret string,
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

		// стандартный режим: будут приниматься от клиентов токены, хранящиеся в таблице accessTokenTableName
		default:
			mrlog.Debug(logger, fmt.Sprintf("Auth.Session: realm=%s, table=%s", realm.Name, accessTokenTableName))
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
			userGroups,
			testUser,
			realm.AuthToken.AccessType,
			jwtSecret,
			[]string{realm.Name},
		)
	}

	realm2provider["*"] = createUserProviderGroup(
		logger,
		dbConnManager,
		userGroups,
		testUser,
		jwtSecret,
		realms,
	)

	for domain, realms := range domain2realms {
		realm2provider[domain+"/*"] = createUserProviderGroup(
			logger,
			dbConnManager,
			userGroups,
			testUser,
			jwtSecret,
			realms,
		)
	}

	return realm2provider
}

func createUserProviderGroup(
	logger mrlog.Logger,
	dbConnManager mrstorage.DBConnManager,
	userGroups mraccess.RightsGetter,
	testUser authcfg.TestUser,
	jwtSecret string,
	userRealms []authcfg.UserRealm,
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
					userGroups,
					testUser,
					tokenType,
					jwtSecret,
					realms,
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
	userGroups mraccess.RightsGetter,
	testUser authcfg.TestUser,
	tokenType string,
	jwtSecret string,
	allowedRealms []string,
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
					userGroups,
				),
			)
		}
	}

	// JWT режим: принимаются от клиентов JWT токены
	if tokenType == "jwt" {
		return mrauth.NewUserProviderJWT(
			userGroups,
			jwtSecret,
			allowedRealms,
		)
	}

	// Session режим: принимаются от клиентов токены, хранящиеся в таблице accessTokenTableName
	return mrauth.NewUserProviderSession(
		dbConnManager,
		userGroups,
		mrsql.DBTableInfo{
			Name:       accessTokenTableName,
			PrimaryKey: accessTokenPrimaryKey,
		},
		allowedRealms,
	)
}
