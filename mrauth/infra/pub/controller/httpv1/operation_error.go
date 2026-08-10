package httpv1

import (
	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth"
)

// wrapOperationError - привязывает ошибку защищённой операции к полю запроса, в котором её токен
// был передан. Сама ошибка уже приведена к доменной в usecase, здесь к ней добавляется только имя
// поля. Ошибки сессионных токенов (access, refresh) сюда не попадают: привязка к полю форсирует
// ответ 400, а им положен 401, поэтому они уходят клиенту как есть.
// Параметр fieldName - имя поля запроса с токеном либо "", когда токен пришёл не полем
// (path-параметр) и привязывать ошибку не к чему.
func wrapOperationError(err error, fieldName string) error {
	if fieldName == "" {
		return err
	}

	// проверяет, относится ли ошибка к предъявленному токену операции
	// или к состоянию операции, на которую он указывает
	if errors.Is(err, mrauth.ErrOperationInvalid) ||
		errors.Is(err, mrauth.ErrOperationIsNotConfirmed) ||
		errors.Is(err, mrauth.ErrOperationAlreadyExpired) ||
		errors.Is(err, mrauth.ErrOperationAlreadyConfirmed) ||
		errors.Is(err, mrauth.ErrResendCodeIsNotSupported) {
		return errors.WithCustomCode(err, fieldName)
	}

	return err
}
