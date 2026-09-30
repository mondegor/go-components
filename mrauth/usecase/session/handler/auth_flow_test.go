package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"testing"

	"github.com/google/uuid"
	sysmesserrors "github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrtype"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/operationtype"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/usecase/session/handler"
	"github.com/mondegor/go-components/mrauth/usecase/session/handler/mock"
)

//go:generate mockgen -source=auth_flow.go -destination=mock/auth_flow.go -package=mock

type AuthFlowSuite struct {
	suite.Suite

	ctrl    *gomock.Controller
	ctx     context.Context
	service *mock.MockauthUserService
	uc      *handler.AuthFlow
}

func TestAuthFlowSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(AuthFlowSuite))
}

func (s *AuthFlowSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.service = mock.NewMockauthUserService(s.ctrl)
	s.uc = handler.NewAuthFlow(s.service)
}

func okScopes() dto.UserScopes {
	return dto.UserScopes{
		UserID:    uuid.New(),
		SessionID: 0x1f3bc817,
		Realm:     "site/admin",
		Kind:      "admin",
		LangCode:  "en",
		TimeZone:  "Europe/Moscow",
	}
}

// okCreateIn - payload операции создания пользователя со всеми заполненными полями.
// Заполнены именно все: этот payload доезжает до сервиса как есть, поэтому поле,
// оставленное нулевым, никем бы здесь не удерживалось.
func okCreateIn() dto.CreateUserOperation {
	return dto.CreateUserOperation{
		Realm:        "site/admin",
		UserKind:     "admin",
		LangCode:     "en",
		TimeZone:     "Europe/Moscow",
		Email:        "user@example.com",
		RegisteredIP: mrtype.NewIP(netip.MustParseAddr("203.0.113.7")),
	}
}

// okAuthorizeIn - payload операции авторизации со всеми заполненными полями.
func okAuthorizeIn() dto.AuthorizeUserOperation {
	return dto.AuthorizeUserOperation{Realm: "site/admin", LangCode: "en"}
}

// confirmedOp - операция с корректным payload'ом для указанного типа операции:
// хелперы разбора проверяют инварианты, поэтому пустой payload здесь уже не подходит.
//
// Payload собирается из самого DTO, а не из строкового литерала: иначе имена json-тегов
// дублировались бы в тесте, и переименование тега его бы не уронило - payload просто
// разобрался бы в нули.
func (s *AuthFlowSuite) confirmedOp(opType operationtype.Enum, userID uuid.UUID) secureoperation.SecureOperation {
	s.T().Helper()

	if opType == operationtype.CreateUser {
		return s.confirmedOpWith(opType, userID, s.mustMarshal(okCreateIn()))
	}

	return s.confirmedOpWith(opType, userID, s.mustMarshal(okAuthorizeIn()))
}

func (s *AuthFlowSuite) confirmedOpWith(opType operationtype.Enum, userID uuid.UUID, payload []byte) secureoperation.SecureOperation {
	s.T().Helper()

	return secureoperation.SecureOperation{
		Type:    opType,
		UserID:  userID,
		Payload: payload,
	}
}

func (s *AuthFlowSuite) mustMarshal(v any) []byte {
	s.T().Helper()

	data, err := json.Marshal(v)
	s.Require().NoError(err)

	return data
}

// вариант 1: создание пользователя (op.UserID == Nil) -> подготовка к авторизации с новым userID.
func (s *AuthFlowSuite) TestCreateUserThenAuthorize() {
	newUserID := uuid.New()
	scopes := okScopes()

	gomock.InOrder(
		s.service.EXPECT().ResolveUser(gomock.Any(), uuid.Nil, gomock.Any()).Return(newUserID, nil),
		s.service.EXPECT().PrepareAuthorization(gomock.Any(), dto.ActorMeta{UserID: newUserID}, gomock.Any()).Return(scopes, nil, nil),
	)

	got, _, err := s.uc.Execute(s.ctx, dto.ActorMeta{}, s.confirmedOp(operationtype.CreateUser, uuid.Nil))
	s.Require().NoError(err)
	s.Equal(scopes, got)
}

// вариант 2: существующий пользователь (op.UserID задан) разрешается через ResolveUser
// (привязка к новому realm либо идемпотентный повтор внутри сервиса), затем идёт подготовка
// к авторизации с тем же userID.
func (s *AuthFlowSuite) TestExistingUserResolvedThenAuthorize() {
	existingUserID := uuid.New()
	scopes := okScopes()

	gomock.InOrder(
		s.service.EXPECT().ResolveUser(gomock.Any(), existingUserID, gomock.Any()).Return(existingUserID, nil),
		s.service.EXPECT().PrepareAuthorization(gomock.Any(), dto.ActorMeta{UserID: existingUserID}, gomock.Any()).Return(scopes, nil, nil),
	)

	got, _, err := s.uc.Execute(s.ctx, dto.ActorMeta{}, s.confirmedOp(operationtype.CreateUser, existingUserID))
	s.Require().NoError(err)
	s.Equal(scopes, got)
}

// вариант 3: подготовка к авторизации без создания (ResolveUser не вызывается);
// отложенный callback login-alert'а пробрасывается наружу без изменений, а клиент (actor)
// передаётся в подготовку авторизации для контекста login-alert'а.
func (s *AuthFlowSuite) TestAuthorizeOnly() {
	userID := uuid.New()
	scopes := okScopes()
	actor := dto.ActorMeta{UserAgent: "test-agent"}

	var called bool

	notify := func(context.Context) { called = true }

	s.service.EXPECT().PrepareAuthorization(gomock.Any(), actor.WithUser(userID), gomock.Any()).Return(scopes, notify, nil)

	got, gotNotify, err := s.uc.Execute(s.ctx, actor, s.confirmedOp(operationtype.AuthorizeUser, userID))
	s.Require().NoError(err)
	s.Equal(scopes, got)

	s.Require().NotNil(gotNotify)
	gotNotify(s.ctx)
	s.True(called)
}

// ошибка ResolveUser прерывает поток - PrepareAuthorization не вызывается.
func (s *AuthFlowSuite) TestResolveUserErrorStops() {
	s.service.EXPECT().ResolveUser(gomock.Any(), uuid.Nil, gomock.Any()).Return(uuid.Nil, errors.New("resolve failed"))

	_, _, err := s.uc.Execute(s.ctx, dto.ActorMeta{}, s.confirmedOp(operationtype.CreateUser, uuid.Nil))
	s.Require().Error(err)
}

// вариант 1: payload операции создания корректно распаковывается в createIn и
// проецируется в authIn = {Realm, LangCode} для подготовки к авторизации.
//
// createIn заполнен целиком и сверяется точным значением, а не gomock.Any(): это
// единственное место, где удерживается, что до сервиса доезжает весь payload -
// включая поля, которые сам обработчик не читает (UserKind, TimeZone, RegisteredIP).
func (s *AuthFlowSuite) TestCreateUserMapsPayloadToAuthorize() {
	newUserID := uuid.New()
	scopes := okScopes()
	createIn := okCreateIn()
	authIn := dto.AuthorizeUserOperation{Realm: createIn.Realm, LangCode: createIn.LangCode}

	gomock.InOrder(
		s.service.EXPECT().ResolveUser(gomock.Any(), uuid.Nil, createIn).Return(newUserID, nil),
		s.service.EXPECT().PrepareAuthorization(gomock.Any(), dto.ActorMeta{UserID: newUserID}, authIn).Return(scopes, nil, nil),
	)

	got, _, err := s.uc.Execute(s.ctx, dto.ActorMeta{}, s.confirmedOpWith(operationtype.CreateUser, uuid.Nil, s.mustMarshal(createIn)))
	s.Require().NoError(err)
	s.Equal(scopes, got)
}

// некорректный payload операции создания - ошибка распаковки, сервис не вызывается.
func (s *AuthFlowSuite) TestCreateUserInvalidPayload() {
	_, _, err := s.uc.Execute(s.ctx, dto.ActorMeta{}, s.confirmedOpWith(operationtype.CreateUser, uuid.Nil, []byte("{")))
	s.Require().ErrorIs(err, sysmesserrors.ErrInternalIncorrectInputData)
}

// некорректный payload операции авторизации - ошибка распаковки, PrepareAuthorization не вызывается.
func (s *AuthFlowSuite) TestAuthorizeInvalidPayload() {
	_, _, err := s.uc.Execute(s.ctx, dto.ActorMeta{}, s.confirmedOpWith(operationtype.AuthorizeUser, uuid.New(), []byte("{")))
	s.Require().ErrorIs(err, sysmesserrors.ErrInternalIncorrectInputData)
}

// payload операции создания синтаксически корректен, но нарушает инвариант (нет email):
// разбор отклоняет его на чтении, сервис не вызывается.
func (s *AuthFlowSuite) TestCreateUserPayloadBrokenInvariant() {
	op := s.confirmedOpWith(operationtype.CreateUser, uuid.Nil, []byte(`{"realm":"site/admin","lang":"en"}`))

	_, _, err := s.uc.Execute(s.ctx, dto.ActorMeta{}, op)
	s.Require().ErrorIs(err, sysmesserrors.ErrInternalIncorrectInputData)
}

// payload операции авторизации синтаксически корректен, но нарушает инвариант (нет realm).
func (s *AuthFlowSuite) TestAuthorizePayloadBrokenInvariant() {
	op := s.confirmedOpWith(operationtype.AuthorizeUser, uuid.New(), []byte(`{"lang":"en"}`))

	_, _, err := s.uc.Execute(s.ctx, dto.ActorMeta{}, op)
	s.Require().ErrorIs(err, sysmesserrors.ErrInternalIncorrectInputData)
}

// вариант 3: payload авторизации корректно распаковывается в authIn и передаётся в PrepareAuthorization.
func (s *AuthFlowSuite) TestAuthorizeMapsPayload() {
	userID := uuid.New()
	scopes := okScopes()
	authIn := okAuthorizeIn()

	s.service.EXPECT().PrepareAuthorization(gomock.Any(), dto.ActorMeta{UserID: userID}, authIn).Return(scopes, nil, nil)

	got, _, err := s.uc.Execute(s.ctx, dto.ActorMeta{}, s.confirmedOpWith(operationtype.AuthorizeUser, userID, s.mustMarshal(authIn)))
	s.Require().NoError(err)
	s.Equal(scopes, got)
}

// ветка авторизации с op.UserID == Nil: PrepareAuthorization вызывается с Nil и распакованным authIn.
func (s *AuthFlowSuite) TestAuthorizeWithNilUserID() {
	scopes := okScopes()
	authIn := okAuthorizeIn()

	s.service.EXPECT().PrepareAuthorization(gomock.Any(), dto.ActorMeta{UserID: uuid.Nil}, authIn).Return(scopes, nil, nil)

	got, _, err := s.uc.Execute(s.ctx, dto.ActorMeta{}, s.confirmedOpWith(operationtype.AuthorizeUser, uuid.Nil, s.mustMarshal(authIn)))
	s.Require().NoError(err)
	s.Equal(scopes, got)
}

// ошибка PrepareAuthorization пробрасывается наружу без изменений.
func (s *AuthFlowSuite) TestPrepareAuthorizationErrorPropagates() {
	wantErr := errors.New("before auth failed")

	s.service.EXPECT().PrepareAuthorization(gomock.Any(), gomock.Any(), gomock.Any()).Return(dto.UserScopes{}, nil, wantErr)

	_, _, err := s.uc.Execute(s.ctx, dto.ActorMeta{}, s.confirmedOp(operationtype.AuthorizeUser, uuid.New()))
	s.Require().ErrorIs(err, wantErr)
}

// актор другого пользователя (или актор с пользователем при операции регистрации без владельца) -
// ошибка проводки вызывающего: сервис не вызывается (мок без EXPECT: любой вызов провалит тест).
func (s *AuthFlowSuite) TestActorNotOwnerRejected() {
	for _, op := range []secureoperation.SecureOperation{
		s.confirmedOp(operationtype.AuthorizeUser, uuid.New()),
		s.confirmedOp(operationtype.CreateUser, uuid.Nil),
	} {
		_, _, err := s.uc.Execute(s.ctx, dto.ActorMeta{UserID: uuid.New()}, op)
		s.Require().ErrorIs(err, sysmesserrors.ErrInternalIncorrectInputData)
	}
}
