package pub

import (
	"context"
	"strings"

	"github.com/mondegor/go-webcore/mrserver"

	module "github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/mrauth/usecase/session"
	"github.com/mondegor/go-components/mrauth/validate"
)

func initSessionsController(
	storageSession *repository.SessionPostgres,
	storageAuthToken *repository.AuthTokenPostgres,
	requestParser *validate.Parser,
	responseSender mrserver.ResponseSender,
	jwtSecret []byte,
	appResolver module.AppResolver,
	locationResolver module.LocationResolver,
) (mrserver.HttpController, error) {
	resolver := newCurrentSessionResolver(
		repository.NewAuthTokenJWT(string(jwtSecret)), // JWT-realm: session_id из claim 'sid'
		storageAuthToken, // session-realm: session_id из БД
	)

	useCaseSessionList := session.NewList(
		storageSession,   // sessionLister
		storageAuthToken, // openSessionFetcher
		storageAuthToken, // sessionCloser
		resolver,         // currentSessionResolver
		appResolver,
		locationResolver,
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
