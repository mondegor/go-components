package security_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/bag/totp"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/usecase/security"
	"github.com/mondegor/go-components/mrauth/usecase/security/mock"
)

type GetTOTPSecretSuite struct {
	baseSuite

	fetcher *mock.MockoperationFetcher
}

func TestGetTOTPSecretSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(GetTOTPSecretSuite))
}

func (s *GetTOTPSecretSuite) SetupTest() {
	s.baseSuite.SetupTest()

	s.fetcher = mock.NewMockoperationFetcher(s.ctrl)
}

// newUseCase - собирает usecase на настоящем аутентификаторе: otpauth-ссылка строится им же,
// что и QR-код, поэтому подмена сборщика моком проверяла бы только передачу аргументов.
func (s *GetTOTPSecretSuite) newUseCase() *security.GetTOTPGeneratorSecret {
	return security.NewGetTOTPGeneratorSecret(s.fetcher, totp.NewAuthenticator("TestIssuer", 64))
}

// TestReturnsSecretAndURI - секрет отдаётся ровно тем же, что лежит в payload операции,
// а otpauth-ссылка несёт его вместе с issuer и именем аккаунта: по ней приложение
// добавляет генератор без ручного ввода.
func (s *GetTOTPSecretSuite) TestReturnsSecretAndURI() {
	userID := uuid.New()

	s.fetcher.EXPECT().
		FetchOne(gomock.Any(), "op-token").
		Return(confirmedOp(userID, `{"email":"u@e","secret":"`+testTotpSecret+`"}`), nil)

	item, err := s.newUseCase().Execute(s.ctx, userID, "op-token")
	s.Require().NoError(err)
	s.Equal(testTotpSecret, item.Secret)
	s.Equal("otpauth://totp/TestIssuer:u@e?issuer=TestIssuer&secret="+testTotpSecret, item.OTPAuthURI)
}

// TestForeignOperation - операция чужого пользователя не выдаёт секрет: гейт тот же,
// что у QR-кода, поэтому обход одного адреса через другой невозможен.
func (s *GetTOTPSecretSuite) TestForeignOperation() {
	s.fetcher.EXPECT().
		FetchOne(gomock.Any(), "op-token").
		Return(confirmedOp(uuid.New(), `{"email":"u@e","secret":"`+testTotpSecret+`"}`), nil)

	_, err := s.newUseCase().Execute(s.ctx, uuid.New(), "op-token")
	s.Require().ErrorIs(err, errors.ErrAccessForbidden)
}

// TestWrongOperationName - токен операции другого типа секрет не выдаёт.
func (s *GetTOTPSecretSuite) TestWrongOperationName() {
	userID := uuid.New()
	op := confirmedOp(userID, `{"email":"u@e","secret":"`+testTotpSecret+`"}`)
	op.Name = "confirm.change.password"

	s.fetcher.EXPECT().FetchOne(gomock.Any(), "op-token").Return(op, nil)

	_, err := s.newUseCase().Execute(s.ctx, userID, "op-token")
	s.Require().ErrorIs(err, errors.ErrAccessForbidden)
}

// TestNotConfirmedOperation - до подтверждения емаил-кода секрет не выдаётся: иначе
// подтверждение владения емаилом можно было бы обойти.
func (s *GetTOTPSecretSuite) TestNotConfirmedOperation() {
	userID := uuid.New()
	op := confirmedOp(userID, `{"email":"u@e","secret":"`+testTotpSecret+`"}`)
	op.Status = operationstatus.Opened

	s.fetcher.EXPECT().FetchOne(gomock.Any(), "op-token").Return(op, nil)

	_, err := s.newUseCase().Execute(s.ctx, userID, "op-token")
	s.Require().ErrorIs(err, secureoperation.ErrOperationIsNotConfirmed)
}

// TestOperationNotFound - неизвестный токен операции приводится usecase к доменной
// ошибке о недействительном токене, а не к отсутствию записи (404).
func (s *GetTOTPSecretSuite) TestOperationNotFound() {
	s.fetcher.EXPECT().
		FetchOne(gomock.Any(), "op-token").
		Return(secureoperation.SecureOperation{}, errors.ErrEventStorageNoRecordFound)

	_, err := s.newUseCase().Execute(s.ctx, uuid.New(), "op-token")
	s.Require().ErrorIs(err, secureoperation.ErrOperationInvalid)
	s.Require().NotErrorIs(err, errors.ErrRecordNotFound)
}
