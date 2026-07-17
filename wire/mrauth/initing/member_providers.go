package initing

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mraccess"
	"github.com/mondegor/go-core/mrlog"
	"github.com/mondegor/go-core/mrstorage"

	"github.com/mondegor/go-components/mrauth/bag/jwt/crypt"
	"github.com/mondegor/go-components/mrauth/model/usergroup"
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
	jwtKeys crypt.KeySet, // OPTIONAL if jwt not exists,
	authTokensTableName string,
) (realm2provider map[string]mraccess.UserProvider, err error) {
	if len(userRealms) == 0 {
		return nil, errors.New("InitUserProviders: no user realm provided")
	}

	if authTokensTableName == "" {
		return nil, errors.New("InitUserProviders: auth tokens table name are not set")
	}

	realm2provider = make(map[string]mraccess.UserProvider, len(userRealms))
	realms := make([]authcfg.UserRealm, 0, len(userRealms))
	domain2realms := make(map[string][]authcfg.UserRealm, len(userRealms))

	for _, realm := range userRealms {
		switch realm.AuthToken.AccessType {
		// если метод аутентификации указан JWT, то будут приниматься от клиентов JWT токены
		case "jwt":
			mrlog.Debug(logger, "Auth.JWT: realm="+realm.Name)

			if jwtKeys == nil {
				return nil, errors.New("InitUserProviders: jwt keys are not set")
			}

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
			jwtKeys,
			[]string{realm.Name},
			authTokensTableName,
		)
	}

	realm2provider["*"] = createUserProviderGroup(
		logger,
		dbConnManager,
		userGroupRights,
		testUser,
		jwtKeys,
		realms,
		authTokensTableName,
	)

	for domain, domainRealms := range domain2realms {
		realm2provider[domain+"/*"] = createUserProviderGroup(
			logger,
			dbConnManager,
			userGroupRights,
			testUser,
			jwtKeys,
			domainRealms,
			authTokensTableName,
		)
	}

	return realm2provider, nil
}

func createUserProviderGroup(
	logger mrlog.Logger,
	dbConnManager mrstorage.DBConnManager,
	userGroupRights mraccess.RightsGetter,
	testUser authcfg.TestUser,
	jwtKeys crypt.KeySet,
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
					jwtKeys,
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
	jwtKeys crypt.KeySet,
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
					usergroup.Build(testUser.Realm, testUser.Kind),
					"00000000", // тестовый пользователь без реальной сессии
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
			jwtKeys,
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
