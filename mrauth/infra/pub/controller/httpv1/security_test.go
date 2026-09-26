package httpv1_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/addresstype"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/mock"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/model"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

//go:generate mockgen -source=security.go -destination=mock/security.go -package=mock
//go:generate mockgen -destination=mock/mrserver_file.go -package=mock github.com/mondegor/go-webcore/mrserver FileResponseSender

// TestSecurityChangePhoneNormalizesNumber - новый телефон доезжает до usecase уже
// нормализованным: разделители отброшены, российский код 8 приведён к 7. Иначе в операцию
// смены телефона ушла бы сырая строка, а её числовое представление осталось бы нулевым.
func TestSecurityChangePhoneNormalizesNumber(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	parser := mock.NewMockRequestParser(ctrl)
	sender := mock.NewMockFileResponseSender(ctrl)
	localizer := mock.NewMockLocalizer(ctrl)
	useCase := mock.NewMockchangePhoneUseCase(ctrl)
	operationResponse := mock.NewMockconfirmOperationResponse(ctrl)

	parser.EXPECT().
		Validate(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ *http.Request, structPointer any) error {
			*structPointer.(*model.ChangePhoneRequest) = model.ChangePhoneRequest{NewPhone: "8 (999) 123-45-67"}

			return nil
		})
	parser.EXPECT().UserID(gomock.Any()).Return(uuid.New())
	parser.EXPECT().DetailedIP(gomock.Any()).Return(mrtype.DetailedIP{})
	parser.EXPECT().Localizer(gomock.Any()).Return(localizer)
	localizer.EXPECT().Translate(gomock.Any()).Return("confirm it")

	useCase.EXPECT().
		Execute(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ dto.ActorMeta, newPhone contactaddress.ContactAddress) (secureoperation.SecureOperation, error) {
			assert.True(t, newPhone.Is(addresstype.Phone))
			assert.Equal(t, "79991234567", newPhone.Value())
			assert.Equal(t, uint64(79991234567), newPhone.DigitValue())

			return secureoperation.SecureOperation{}, nil
		})
	operationResponse.EXPECT().
		NewConfirmOperation(gomock.Any(), "confirm it").
		Return(model.WaitingConfirmOperationResponse{})
	sender.EXPECT().Send(gomock.Any(), http.StatusOK, gomock.Any()).Return(nil)

	controller := httpv1.NewSecurity(
		parser, sender, nil, nil, nil, useCase, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, operationResponse,
	)

	require.NoError(
		t,
		controller.ChangePhone(
			httptest.NewRecorder(),
			httptest.NewRequest(http.MethodPost, "/v1/security/phone", http.NoBody),
		),
	)
}

// TestSecurityChangePasswordTooWeakBoundToField - слабый пароль - ошибка значения поля,
// поэтому код привязан к new_password и ложится под поле ввода, как обещает контракт.
func TestSecurityChangePasswordTooWeakBoundToField(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	parser := mock.NewMockRequestParser(ctrl)
	useCase := mock.NewMockchangePasswordUseCase(ctrl)

	parser.EXPECT().
		Validate(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ *http.Request, structPointer any) error {
			*structPointer.(*model.ChangePasswordRequest) = model.ChangePasswordRequest{NewPassword: "aaaaaaaa"}

			return nil
		})
	parser.EXPECT().UserID(gomock.Any()).Return(uuid.New())
	parser.EXPECT().DetailedIP(gomock.Any()).Return(mrtype.DetailedIP{})

	useCase.EXPECT().
		Execute(gomock.Any(), gomock.Any(), "aaaaaaaa").
		Return(secureoperation.SecureOperation{}, mrauth.ErrPasswordIsTooWeak)

	controller := httpv1.NewSecurity(
		parser, nil, nil, nil, nil, nil, nil, useCase, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)

	err := controller.ChangePassword(
		httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/v1/security/password", http.NoBody),
	)

	var customErr errors.CustomError

	require.ErrorAs(t, err, &customErr)
	assert.Equal(t, "PasswordIsTooWeak/new_password", customErr.CustomCode())
	require.ErrorIs(t, err, mrauth.ErrPasswordIsTooWeak)
}

// TestSecurityApplyEmail - применение первого шага смены емаила отвечает 200 с операцией
// второго шага (WaitingConfirmOperation). Ошибки токена привязываются к полю token, а занятый
// за время подтверждения адрес - нет: поля с адресом в запросе нет, ошибка показывается
// общим уведомлением, как обещает контракт.
func TestSecurityApplyEmail(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		useCaseErr     error
		wantCustomCode string // "" - ошибки нет либо она не привязана к полю
	}

	tests := []testCase{
		{name: "success"},
		{name: "operation invalid", useCaseErr: mrauth.ErrOperationInvalid, wantCustomCode: "OperationInvalid/token"},
		{name: "not confirmed", useCaseErr: mrauth.ErrOperationIsNotConfirmed, wantCustomCode: "OperationIsNotConfirmed/token"},
		{name: "email already exists", useCaseErr: mrauth.ErrEmailAlreadyExists},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			parser := mock.NewMockRequestParser(ctrl)
			sender := mock.NewMockFileResponseSender(ctrl)
			localizer := mock.NewMockLocalizer(ctrl)
			useCase := mock.NewMockapplyEmailUseCase(ctrl)
			operationResponse := mock.NewMockconfirmOperationResponse(ctrl)

			parser.EXPECT().
				Validate(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ *http.Request, structPointer any) error {
					*structPointer.(*model.ApplyEmailRequest) = model.ApplyEmailRequest{Token: "op-token"}

					return nil
				})
			parser.EXPECT().UserID(gomock.Any()).Return(uuid.New())
			parser.EXPECT().DetailedIP(gomock.Any()).Return(mrtype.DetailedIP{})

			// пояс пользователя из запроса доходит до usecase: в нём выводится срок в уведомлении
			userLocation := time.FixedZone("MSK", 3*60*60)
			parser.EXPECT().Location(gomock.Any()).Return(userLocation)

			confirmOp := secureoperation.SecureOperation{Token: "new-op-token"}
			useCase.EXPECT().
				Execute(gomock.Any(), gomock.Any(), userLocation, "op-token").
				Return(confirmOp, tt.useCaseErr)

			if tt.useCaseErr == nil {
				parser.EXPECT().Localizer(gomock.Any()).Return(localizer)
				localizer.EXPECT().Translate(gomock.Any()).Return("confirm it")
				operationResponse.EXPECT().
					NewConfirmOperation(confirmOp, "confirm it").
					Return(model.WaitingConfirmOperationResponse{})
				sender.EXPECT().Send(gomock.Any(), http.StatusOK, gomock.Any()).Return(nil)
			}

			controller := httpv1.NewSecurity(
				parser, sender, nil, nil, useCase, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, operationResponse,
			)

			err := controller.ApplyEmail(
				httptest.NewRecorder(),
				httptest.NewRequest(http.MethodPost, "/v1/security/apply-email", http.NoBody),
			)

			if tt.useCaseErr == nil {
				require.NoError(t, err)

				return
			}

			require.ErrorIs(t, err, tt.useCaseErr)

			var customErr errors.CustomError

			if tt.wantCustomCode == "" {
				assert.NotErrorAs(t, err, &customErr)

				return
			}

			require.ErrorAs(t, err, &customErr)
			assert.Equal(t, tt.wantCustomCode, customErr.CustomCode())
		})
	}
}

// TestSecurityTOTPRoutes - заготовка TOTP-генератора отдаётся двумя представлениями, и какое
// лежит по какому адресу, видно только здесь: базовый адрес - JSON с секретом, под-ресурс
// /qrcode - картинка. Тест держит это соответствие, иначе перестановка обработчиков или
// возврат старого адреса разошлись бы со спекой молча.
func TestSecurityTOTPRoutes(t *testing.T) {
	t.Parallel()

	// зависимости не нужны: Handlers() лишь собирает список, обработчики не вызываются
	controller := httpv1.NewSecurity(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)

	handlerByURL := make(map[string]string)

	for _, h := range controller.Handlers() {
		if h.Method == http.MethodGet {
			handlerByURL[h.URL] = handlerName(h.Func)
		}
	}

	secret, ok := handlerByURL["/v1/security/totp/{token}"]
	require.True(t, ok, "секрет TOTP генератора должен отдаваться по GET /v1/security/totp/{token}")
	assert.Equal(t, "GetTOTPGeneratorSecret", secret)

	qrcode, ok := handlerByURL["/v1/security/totp/{token}/qrcode"]
	require.True(t, ok, "QR-код должен отдаваться по GET /v1/security/totp/{token}/qrcode")
	assert.Equal(t, "RenderTOTPGeneratorQR", qrcode)
}

// handlerName - имя метода, на который указывает обработчик: сами функции в Go не сравниваются,
// поэтому проверяется имя. Для method value runtime отдаёт его с суффиксом "-fm".
func handlerName(fn any) string {
	full := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
	name := full[strings.LastIndexByte(full, '.')+1:]

	return strings.TrimSuffix(name, "-fm")
}
