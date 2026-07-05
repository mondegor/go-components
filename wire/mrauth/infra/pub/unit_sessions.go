package pub

import (
	"context"
	"strings"

	"github.com/mondegor/go-webcore/mrserver"

	module "github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/bag/jwt/crypt"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/mrauth/usecase/session"
	"github.com/mondegor/go-components/mrauth/validate"
	authcfg "github.com/mondegor/go-components/wire/mrauth/config"
	"github.com/mondegor/go-components/wire/mrauth/mapping"
)

type (
	sessionResolver interface {
		FetchOneByAccessToken(ctx context.Context, accessToken string) (dto.UserScopes, error)
	}
)

func initSessionsController(
	storageSession *repository.SessionPostgres,
	storageAuthToken *repository.AuthTokenPostgres,
	storageUserRealm *repository.UserRealmPostgres,
	requestParser *validate.Parser,
	responseSender mrserver.ResponseSender,
	appResolver module.AppResolver,
	locationResolver module.LocationResolver,
	userRealms []authcfg.UserRealm,
	jwtKeys crypt.KeySet, // OPTIONAL
) (mrserver.HttpController, error) {
	resolver := sessionResolver(storageAuthToken)
	if jwtKeys != nil {
		resolver = newCurrentSessionResolver(
			repository.NewAuthTokenJWT(jwtKeys), // JWT-realm: session_id из claim 'sid'
			storageAuthToken,                    // session-realm: session_id из БД
		)
	}

	useCaseSessionList := session.NewList(
		storageSession,   // sessionLister
		storageAuthToken, // openSessionFetcher
		storageAuthToken, // sessionCloser
		resolver,
		storageUserRealm, // userRealmFetcher
		mapping.OptionUserRealmsToRealmRegistry(userRealms),
		appResolver,
		locationResolver,
		mapping.OptionUserRealmsToSessionLimitRealms(userRealms),
	)

	controller := httpv1.NewSession(
		requestParser,
		responseSender,
		useCaseSessionList,
	)

	return controller, nil
}

// currentSessionResolver - резолвит session_id текущего запроса, диспетчеризуя по виду
// токена (JWT vs opaque session), зеркаля логику NewUserProvider (wire/mrauth/user_provider.go).
type currentSessionResolver struct {
	jwt     module.AuthTokenFetcher
	session module.AuthTokenFetcher
}

func newCurrentSessionResolver(jwtFetcher, sessionFetcher module.AuthTokenFetcher) *currentSessionResolver {
	return &currentSessionResolver{
		jwt:     jwtFetcher,
		session: sessionFetcher,
	}
}

// FetchOneByAccessToken - возвращает область действия (включая session_id) текущего токена.
func (r *currentSessionResolver) FetchOneByAccessToken(ctx context.Context, accessToken string) (dto.UserScopes, error) {
	// JWT состоит из трёх частей через точку: header.payload.signature
	if strings.Count(accessToken, ".") == 2 {
		return r.jwt.FetchOneByAccessToken(ctx, accessToken)
	}

	return r.session.FetchOneByAccessToken(ctx, accessToken)
}
