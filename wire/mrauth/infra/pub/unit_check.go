package pub

import (
	"github.com/mondegor/go-webcore/mrserver"

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
) (mrserver.HttpController, error) {
	userLoginService := check.NewUserLogin(
		storageCheckUser,
		storageUserRealm,
	)

	controller := httpv1.NewCheck(
		requestParser,
		responseSender,
		userLoginService,
		check.NewPassword(16), // TODO: в настройки
	)

	return controller, nil
}
