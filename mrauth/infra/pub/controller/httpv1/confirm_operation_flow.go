package httpv1

import (
	"net/http"

	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-webcore/mrserver"
	"github.com/mondegor/go-webcore/mrserver/mrresp"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/validate"
)

// confirmOperationFlow - общий шаг подтверждения защищённой операции для обработчиков,
// которым требуется подтверждение перед продолжением (открытие сессии, подтверждение операции).
type confirmOperationFlow struct {
	parser            validate.RequestParser
	sender            mrserver.ResponseSender
	useCase           confirmOperationUseCase
	operationResponse confirmOperationResponse
	debugFunc         func(value any) string
}

// confirm - подтверждает защищённую операцию переданным секретом.
// При неверном коде / исчерпании попыток или необходимости 2FA сам отправляет ответ и
// возвращает ok=false. ok=true означает, что операция полностью подтверждена и вызывающий
// обработчик может продолжать. Параметр waitMessage - сообщение для ветки 2FA.
func (f confirmOperationFlow) confirm(
	w http.ResponseWriter,
	r *http.Request,
	token, secret, waitMessage string,
) (op secureoperation.SecureOperation, ok bool, err error) {
	lz := f.parser.Localizer(r)

	op, err = f.useCase.Execute(r.Context(), lz.Language(), token, secret)
	if err != nil {
		if errors.Is(err, secureoperation.ErrConfirmCodeIsIncorrect) ||
			errors.Is(err, secureoperation.ErrNoAttemptsToConfirmOperation) {
			return op, false, f.sender.Send(
				w,
				http.StatusBadRequest,
				f.operationResponse.NewErrorConfirmOperation(
					mrresp.NewError400Response(
						r,
						mrresp.ErrorAttribute{
							Code:      "secret",
							Detail:    lz.TranslateError(err),
							DebugInfo: f.debugFunc(err),
						},
					),
					op,
				),
			)
		}

		if errors.Is(err, errors.ErrRecordNotFound) {
			return op, false, mrauth.ErrTokenNotFoundOrExpired
		}

		return op, false, err
	}

	if op.Is(operationstatus.Confirmed) {
		return op, true, nil
	}

	// иначе необходимо дополнительное подтверждение (2fa)
	return op, false, f.sender.Send(
		w,
		http.StatusOK,
		f.operationResponse.NewConfirmOperation(
			op,
			lz.Translate(waitMessage),
		),
	)
}
