package pub

import (
	"github.com/mondegor/go-webcore/mrserver"

	"github.com/mondegor/go-components/mrauth/bag/jwt/crypt"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/mrauth/service/check"
	"github.com/mondegor/go-components/mrauth/validate"
)

func initCheckController(
	storageCheckUser *repository.CheckUserPostgres,
	storageUserRealm *repository.UserRealmPostgres,
	requestParser *validate.Parser,
	responseSender mrserver.ResponseSender,
	jwtKeys crypt.KeySet, // OPTIONAL
) (mrserver.HttpController, error) {
	userLoginService := check.NewUserLogin(
		storageCheckUser,
		storageUserRealm,
	)

	// набор ключей статичен на время жизни процесса - сериализуем JWKS один раз при инициализации;
	// для session-only режима (jwtKeys == nil) тело отсутствует и эндпоинт отдаёт 404
	var jwksJSONBody []byte

	if jwtKeys != nil {
		body, err := jwtKeys.JWKS()
		if err != nil {
			return nil, err
		}

		jwksJSONBody = body
	}

	controller := httpv1.NewCheck(
		requestParser,
		responseSender,
		userLoginService,
		check.NewPassword(16), // TODO: в настройки
		jwksJSONBody,
	)

	return controller, nil
}
