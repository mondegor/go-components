package auth_test

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	sysmesserrors "github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrlock"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-core/mrtype"
	"github.com/mondegor/go-core/util/conv"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/logstatus"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
	"github.com/mondegor/go-components/mrauth/usecase/auth"
	"github.com/mondegor/go-components/mrauth/usecase/auth/mock"
)

//go:generate mockgen -source=create_session.go -destination=mock/create_session.go -package=mock
//go:generate mockgen -source=create_user.go -destination=mock/create_user.go -package=mock
//go:generate mockgen -source=user_statistic.go -destination=mock/user_statistic.go -package=mock
//go:generate mockgen -destination=mock/mrstorage.go -package=mock github.com/mondegor/go-core/mrstorage DBTxManager
//go:generate mockgen -destination=mock/mrlock.go -package=mock github.com/mondegor/go-core/mrlock Locker
//go:generate mockgen -destination=mock/mrnotifier.go -package=mock github.com/mondegor/go-components/mrnotifier NoteProducer
//go:generate mockgen -destination=mock/mrauth.go -package=mock github.com/mondegor/go-components/mrauth User2FAConfirmActionCreator

// testIP - IP регистрации, используемый во всех тестах создания пользователя.
func testIP() mrtype.DetailedIP {
	return mrtype.NewIP(netip.MustParseAddr("203.0.113.7"))
}

// testTZ - непустой запрос часового пояса; резолвер возвращает его имя как есть.
func testTZ() dto.TimeZoneInfo {
	return dto.TimeZoneInfo{Name: "Europe/Moscow"}
}

// expectPassThroughTx - транзакция выполняет переданное задание как есть.
func expectPassThroughTx(txManager *mock.MockDBTxManager) {
	txManager.EXPECT().
		Do(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, job func(ctx context.Context) error, _ ...mrstorage.TxOption) error {
			return job(ctx)
		}).
		AnyTimes()
}

func newOpenedEmailOp(t *testing.T) secureoperation.SecureOperation {
	t.Helper()

	op, err := secureoperation.NewOperation(
		"op-token",
		"confirm.create",
		uuid.New(),
		[]secureoperation.ConfirmAction{
			{
				Method:           confirmmethod.Email,
				MaxAttempts:      3,
				MaxResends:       5,
				MinResendTime:    5 * time.Minute,
				Expiry:           10 * time.Minute,
				Address:          "u@e",
				ConfirmCode:      "code123",
				PlainConfirmCode: "code123",
			},
		},
		nil,
	)
	require.NoError(t, err)

	return op
}

type CreateSessionSuite struct {
	suite.Suite

	ctrl         *gomock.Controller
	ctx          context.Context
	txManager    *mock.MockDBTxManager
	checker      *mock.MockuserLoginChecker
	opener       *mock.MockoperationOpener
	factory2FA   *mock.MockUser2FAConfirmActionCreator
	opFactory    *mock.MockcreateSessionOperation
	logOperation *mock.MockoperationLogger
	logEntries   []entity.SecureOperationLog
	openedNote   string
}

func TestCreateSessionSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(CreateSessionSuite))
}

func (s *CreateSessionSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.txManager = mock.NewMockDBTxManager(s.ctrl)
	s.checker = mock.NewMockuserLoginChecker(s.ctrl)
	s.opener = mock.NewMockoperationOpener(s.ctrl)
	s.factory2FA = mock.NewMockUser2FAConfirmActionCreator(s.ctrl)
	s.opFactory = mock.NewMockcreateSessionOperation(s.ctrl)
	s.logOperation = mock.NewMockoperationLogger(s.ctrl)
	s.logEntries = nil
	s.openedNote = ""

	expectPassThroughTx(s.txManager)

	s.opFactory.EXPECT().Name().Return(unit.NameAuthorizeUser).AnyTimes()
	s.factory2FA.EXPECT().CreateByUserLogin(gomock.Any(), gomock.Any()).Return(dto.User2FA{}, nil).AnyTimes()
	s.factory2FA.EXPECT().CreateByUserID(gomock.Any(), gomock.Any()).Return(dto.User2FA{}, nil).AnyTimes()
	s.logOperation.EXPECT().
		Log(gomock.Any(), gomock.Any()).
		Do(func(_ context.Context, entry entity.SecureOperationLog) {
			s.logEntries = append(s.logEntries, entry)
		}).
		AnyTimes()
}

func (s *CreateSessionSuite) newUseCase() *auth.CreateSession {
	return auth.NewCreateSession(
		s.opener,
		s.checker,
		s.factory2FA,
		s.logOperation,
		[]auth.CreateSessionRealm{{Name: "shop", Operation: s.opFactory}},
	)
}

// expectOpen - компонент открытия операции отрабатывает успешно; запоминается имя
// шаблона уведомления, которое usecase ему передал.
func (s *CreateSessionSuite) expectOpen() {
	s.opener.EXPECT().
		Open(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ dto.ActorMeta, _ secureoperation.SecureOperation, noteName string, _ conv.Group) error {
			s.openedNote = noteName

			return nil
		})
}

// expectCheckLogin - результат проверки доступности логина в realm.
func (s *CreateSessionSuite) expectCheckLogin(err error) {
	s.checker.EXPECT().CheckAvailabilityRealm(gomock.Any(), gomock.Any(), gomock.Any()).Return(err).AnyTimes()
}

func (s *CreateSessionSuite) TestEmptyLogin() {
	_, err := s.newUseCase().Execute(s.ctx, dto.ActorMeta{}, "shop", "en", "")
	s.Require().Error(err)
}

func (s *CreateSessionSuite) TestUnknownRealm() {
	_, err := s.newUseCase().Execute(s.ctx, dto.ActorMeta{}, "unknown", "en", "user@example.com")
	s.Require().Error(err)
}

func (s *CreateSessionSuite) TestLoginDoesNotExist() {
	// nil от CheckAvailabilityRealm означает, что логин свободен - значит входить некому.
	s.expectCheckLogin(nil)
	s.opFactory.EXPECT().
		Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(newOpenedEmailOp(s.T()), nil).
		AnyTimes()

	_, err := s.newUseCase().Execute(s.ctx, dto.ActorMeta{}, "shop", "en", "user@example.com")
	s.Require().ErrorIs(err, mrauth.ErrLoginNotExists)
	s.Require().Len(s.logEntries, 1)
	s.Equal(logstatus.Blocked, s.logEntries[0].LogStatus)
	s.Equal(logreason.LoginNotExists, s.logEntries[0].Reason)
	s.Equal(unit.NameAuthorizeUser, s.logEntries[0].OperationName)
}

func (s *CreateSessionSuite) TestSuccess() {
	op := newOpenedEmailOp(s.T())

	s.expectCheckLogin(mrauth.ErrEmailAlreadyExists)
	s.opFactory.EXPECT().
		Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(op, nil).
		AnyTimes()
	s.expectOpen()

	_, err := s.newUseCase().Execute(s.ctx, dto.ActorMeta{}, "shop", "en", "user@example.com")
	s.Require().NoError(err)
	s.Equal("confirm.create.session.by.email", s.openedNote)
	// запись об открытии операции (и о вытеснении прежних) пишет компонент Opener,
	// поэтому здесь журнал остаётся пустым - проверка в его собственном тесте
	s.Empty(s.logEntries)
}

func (s *CreateSessionSuite) TestCheckerError() {
	s.expectCheckLogin(errors.New("boom"))
	s.opFactory.EXPECT().
		Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(newOpenedEmailOp(s.T()), nil).
		AnyTimes()

	_, err := s.newUseCase().Execute(s.ctx, dto.ActorMeta{}, "shop", "en", "user@example.com")
	s.Require().Error(err)
}

type CreateUserSuite struct {
	suite.Suite

	ctrl         *gomock.Controller
	ctx          context.Context
	txManager    *mock.MockDBTxManager
	checker      *mock.MockuserLoginChecker
	opener       *mock.MockoperationOpener
	factory2FA   *mock.Mockuser2faActionCreator
	locker       *mock.MockLocker
	opFactory    *mock.MockcreateUserOperation
	tzResolver   *mock.MocktimeZoneResolver
	logOperation *mock.MockoperationLogger
	logEntries   []entity.SecureOperationLog

	// аргументы, доехавшие до компонента открытия операции
	openedNote  string
	openedActor dto.ActorMeta

	// аргументы, доехавшие до фабрики операции создания пользователя
	gotUser2FA      dto.User2FA
	gotTimeZone     string
	gotRegisteredIP mrtype.DetailedIP
}

func TestCreateUserSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(CreateUserSuite))
}

func (s *CreateUserSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.txManager = mock.NewMockDBTxManager(s.ctrl)
	s.checker = mock.NewMockuserLoginChecker(s.ctrl)
	s.opener = mock.NewMockoperationOpener(s.ctrl)
	s.factory2FA = mock.NewMockuser2faActionCreator(s.ctrl)
	s.locker = mock.NewMockLocker(s.ctrl)
	s.opFactory = mock.NewMockcreateUserOperation(s.ctrl)
	s.tzResolver = mock.NewMocktimeZoneResolver(s.ctrl)
	s.logOperation = mock.NewMockoperationLogger(s.ctrl)
	s.logEntries = nil
	s.openedNote = ""
	s.openedActor = dto.ActorMeta{}
	s.gotUser2FA = dto.User2FA{}
	s.gotTimeZone = ""
	s.gotRegisteredIP = mrtype.DetailedIP{}

	expectPassThroughTx(s.txManager)

	s.opFactory.EXPECT().Name().Return(unit.NameConfirmCreateUser).AnyTimes()
	s.logOperation.EXPECT().
		Log(gomock.Any(), gomock.Any()).
		Do(func(_ context.Context, entry entity.SecureOperationLog) {
			s.logEntries = append(s.logEntries, entry)
		}).
		AnyTimes()

	// резолвер возвращает запрошенное имя пояса, а при его отсутствии - UTC;
	// подбор по смещению проверяется отдельно в тестах самого резолвера
	s.tzResolver.EXPECT().
		Resolve(gomock.Any()).
		DoAndReturn(func(in dto.TimeZoneInfo) string {
			if in.Name == "" {
				return "UTC"
			}

			return in.Name
		}).
		AnyTimes()
}

// expectHappyDeps - дефолтные ответы блокировки, проверки логина и фабрики 2FA.
// Вызывается каждым тестом явно: gomock выбирает первое подходящее ожидание, поэтому
// заданный в SetupTest дефолт перекрыть уже нельзя.
func (s *CreateUserSuite) expectHappyDeps() {
	s.expectLock(nil)
	s.expectCheckLogin(nil)
	s.expect2FA(dto.User2FA{}, nil)
}

func (s *CreateUserSuite) newUseCase() *auth.CreateUser {
	return auth.NewCreateUser(
		s.opener,
		s.checker,
		s.factory2FA,
		s.locker,
		s.logOperation,
		s.tzResolver,
		[]auth.CreateUserRealm{{Name: "shop", Operation: s.opFactory}},
	)
}

// expectOpen - компонент открытия операции отрабатывает успешно; запоминаются имя шаблона
// уведомления и метаданные посетителя, которые usecase ему передал.
func (s *CreateUserSuite) expectOpen() {
	s.opener.EXPECT().
		Open(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, actor dto.ActorMeta, _ secureoperation.SecureOperation, noteName string, _ conv.Group) error {
			s.openedNote = noteName
			s.openedActor = actor

			return nil
		})
}

func (s *CreateUserSuite) expectCheckLogin(err error) {
	s.checker.EXPECT().CheckAvailabilityRealm(gomock.Any(), gomock.Any(), gomock.Any()).Return(err).AnyTimes()
}

func (s *CreateUserSuite) expect2FA(user dto.User2FA, err error) {
	s.factory2FA.EXPECT().CreateByUserLogin(gomock.Any(), gomock.Any()).Return(user, err).AnyTimes()
}

func (s *CreateUserSuite) expectLock(err error) {
	if err != nil {
		s.locker.EXPECT().Lock(gomock.Any(), gomock.Any()).Return(nil, err).AnyTimes()
		s.locker.EXPECT().LockWithExpiry(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, err).AnyTimes()

		return
	}

	unlock := func() {}
	s.locker.EXPECT().Lock(gomock.Any(), gomock.Any()).Return(unlock, nil).AnyTimes()
	s.locker.EXPECT().LockWithExpiry(gomock.Any(), gomock.Any(), gomock.Any()).Return(unlock, nil).AnyTimes()
}

// expectCreateOperation - фабрика операции запоминает доехавшие до неё аргументы.
func (s *CreateUserSuite) expectCreateOperation(op secureoperation.SecureOperation, err error) {
	s.opFactory.EXPECT().
		Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			user2FA dto.User2FA,
			_ string,
			timeZone string,
			_ any,
			registeredIP mrtype.DetailedIP,
		) (secureoperation.SecureOperation, error) {
			s.gotUser2FA = user2FA
			s.gotTimeZone = timeZone
			s.gotRegisteredIP = registeredIP

			return op, err
		}).
		AnyTimes()
}

func (s *CreateUserSuite) TestUnknownRealm() {
	s.expectHappyDeps()

	_, err := s.newUseCase().Execute(s.ctx, "unknown", "en", testTZ(), "user@example.com", testIP())
	s.Require().Error(err)
}

func (s *CreateUserSuite) TestInvalidEmail() {
	s.expectHappyDeps()

	_, err := s.newUseCase().Execute(s.ctx, "shop", "en", testTZ(), "bad", testIP())
	s.Require().Error(err)
}

func (s *CreateUserSuite) TestLockNotObtained() {
	s.expectLock(mrlock.ErrLockKeyNotObtained)
	s.expectCheckLogin(nil)
	s.expect2FA(dto.User2FA{}, nil)
	s.expectCreateOperation(newOpenedEmailOp(s.T()), nil)

	_, err := s.newUseCase().Execute(s.ctx, "shop", "en", testTZ(), "user@example.com", testIP())
	s.Require().ErrorIs(err, mrauth.ErrSignupAlreadyInProgressTryLater)
	s.Require().Len(s.logEntries, 1)
	s.Equal(logstatus.Blocked, s.logEntries[0].LogStatus)
	s.Equal(logreason.Throttled, s.logEntries[0].Reason)
	s.Equal(unit.NameConfirmCreateUser, s.logEntries[0].OperationName)
}

func (s *CreateUserSuite) TestSuccess() {
	s.expectHappyDeps()
	s.expectCreateOperation(newOpenedEmailOp(s.T()), nil)
	s.expectOpen()

	_, err := s.newUseCase().Execute(s.ctx, "shop", "en", testTZ(), "user@example.com", testIP())
	s.Require().NoError(err)
	s.Equal(testIP(), s.gotRegisteredIP, "IP регистрации доезжает до фабрики операции")
	s.Equal("Europe/Moscow", s.gotTimeZone, "в payload операции попадает уже подобранный пояс")
	s.Equal("confirm.user.activation", s.openedNote)
	// поток регистрации анонимный: форензику несёт IP, а не идентификатор посетителя
	s.Equal(testIP(), s.openedActor.ClientIP)
	s.Equal(uuid.Nil, s.openedActor.VisitorID)
	// запись об открытии операции пишет компонент Opener - проверка в его собственном тесте
	s.Empty(s.logEntries)
}

// TestTimeZoneResolvedOnEntry - пояс подбирается до записи payload'а, поэтому
// незаполненный запрос доезжает до фабрики операции уже как UTC, а не как пустая строка.
func (s *CreateUserSuite) TestTimeZoneResolvedOnEntry() {
	s.expectHappyDeps()
	s.expectCreateOperation(newOpenedEmailOp(s.T()), nil)
	s.expectOpen()

	_, err := s.newUseCase().Execute(s.ctx, "shop", "en", dto.TimeZoneInfo{}, "user@example.com", testIP())
	s.Require().NoError(err)
	s.Equal("UTC", s.gotTimeZone)
}

func (s *CreateUserSuite) TestCheckerError() {
	s.expectCheckLogin(errors.New("boom"))
	s.expectLock(nil)
	s.expect2FA(dto.User2FA{}, nil)
	s.expectCreateOperation(newOpenedEmailOp(s.T()), nil)

	_, err := s.newUseCase().Execute(s.ctx, "shop", "en", testTZ(), "user@example.com", testIP())
	s.Require().Error(err)
}

func (s *CreateUserSuite) Test2FAFactoryError() {
	s.expect2FA(dto.User2FA{}, errors.New("2fa boom"))
	s.expectLock(nil)
	s.expectCheckLogin(nil)
	s.expectCreateOperation(newOpenedEmailOp(s.T()), nil)

	_, err := s.newUseCase().Execute(s.ctx, "shop", "en", testTZ(), "user@example.com", testIP())
	s.Require().Error(err)
}

// TestNewEmailEmpty2FAForwarded - логин не принадлежит существующему пользователю:
// фабрика 2FA возвращает ErrEventStorageNoRecordFound, usecase продолжает выполнение
// с пустым User2FA.
func (s *CreateUserSuite) TestNewEmailEmpty2FAForwarded() {
	s.expectLock(nil)
	s.expectCheckLogin(nil)
	s.expect2FA(dto.User2FA{}, sysmesserrors.ErrEventStorageNoRecordFound)
	s.expectCreateOperation(newOpenedEmailOp(s.T()), nil)
	s.expectOpen()

	_, err := s.newUseCase().Execute(s.ctx, "shop", "en", testTZ(), "user@example.com", testIP())
	s.Require().NoError(err)
	s.Equal(dto.User2FA{}, s.gotUser2FA)
}

func (s *CreateUserSuite) TestExistingUser2FAForwarded() {
	s.expectLock(nil)
	s.expectCheckLogin(nil)

	user2FA := dto.User2FA{
		ID:        uuid.New(),
		Email:     "user@example.com",
		Action2FA: secureoperation.ConfirmAction{Method: confirmmethod.TOTP},
	}

	s.expect2FA(user2FA, nil)
	s.expectCreateOperation(newOpenedEmailOp(s.T()), nil)
	s.expectOpen()

	_, err := s.newUseCase().Execute(s.ctx, "shop", "en", testTZ(), "user@example.com", testIP())
	s.Require().NoError(err)
	s.Equal(user2FA, s.gotUser2FA)
}
