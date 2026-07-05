package session_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrlog"
	"github.com/mondegor/go-sysmess/mrstorage"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/mrauth/service/realm"
	"github.com/mondegor/go-components/mrauth/usecase/session"
	"github.com/mondegor/go-components/mrauth/usecase/session/mock"
)

const (
	testRealm          = "site/admin"
	testRealmID uint16 = 1
	altRealm           = "r"
	altRealmID  uint16 = 2
)

// testRealmRegistry - реестр realm'ов, используемый в тестах сессий.
func testRealmRegistry() mrauth.RealmRegistry {
	return realm.New([]realm.Realm{
		{ID: testRealmID, Name: testRealm},
		{ID: altRealmID, Name: altRealm},
	})
}

//go:generate mockgen -source=session_open.go -destination=mock/session_open.go -package=mock
//go:generate mockgen -source=session_continue.go -destination=mock/session_continue.go -package=mock
//go:generate mockgen -source=session_close.go -destination=mock/session_close.go -package=mock
//go:generate mockgen -source=session_list.go -destination=mock/session_list.go -package=mock
//go:generate mockgen -destination=mock/mrstorage.go -package=mock github.com/mondegor/go-sysmess/mrstorage DBTxManager
//go:generate mockgen -destination=mock/mrevent.go -package=mock github.com/mondegor/go-sysmess/mrevent Emitter

func okScopes() dto.UserScopes {
	return dto.UserScopes{UserID: uuid.New(), Realm: "site/admin", Kind: "admin", LangCode: "en"}
}

func okPair() dto.AuthTokenPair {
	return dto.AuthTokenPair{
		Access:  dto.AccessToken{Token: "access"},
		Refresh: dto.RefreshToken{Token: "refresh"},
	}
}

func runJob(_ context.Context, job func(context.Context) error, _ ...mrstorage.TxOption) error {
	return job(context.Background())
}

// ----- OpenSession -----

type OpenSessionSuite struct {
	suite.Suite

	ctrl        *gomock.Controller
	ctx         context.Context
	tx          *mock.MockDBTxManager
	issuer      *mock.MocksessionIssuer
	activity    *mock.MockuserActivityStatCreator
	openCounter *mock.MockopenSessionCounter
	excessQueue *mock.MockexcessQueueProducer
	authFlow    *mock.MockauthFlowHandler
	creator     *mock.MocktokenCreator
	storageOp   *mock.MockoperationConsumer
	uc          *session.OpenSession
	notifyCount int
}

// buildUC - пересобирает OpenSession с лимитом limit для realm/kind из okScopes() ("site/admin"/"admin")
// и дефолтными порогами (soft=0, hard=4).
func (s *OpenSessionSuite) buildUC(limit int) {
	s.buildUCThresholds(limit, 0, 0)
}

// buildUCThresholds - как buildUC, но с явными soft/hard порогами.
func (s *OpenSessionSuite) buildUCThresholds(limit, soft, hard int) {
	s.uc = session.NewOpenSession(
		s.tx,
		s.issuer,
		s.activity,
		s.openCounter,
		s.excessQueue,
		s.authFlow,
		s.creator,
		s.storageOp,
		testRealmRegistry(),
		mrlog.NopLogger(),
		[]session.LimitRealm{{
			ID:         testRealmID,
			KindLimits: []session.UserKindLimit{{Kind: "admin", SessionMax: limit}},
		}},
		soft,
		hard,
	)
}

func TestOpenSessionSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(OpenSessionSuite))
}

func (s *OpenSessionSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.tx = mock.NewMockDBTxManager(s.ctrl)
	s.issuer = mock.NewMocksessionIssuer(s.ctrl)
	s.activity = mock.NewMockuserActivityStatCreator(s.ctrl)
	s.openCounter = mock.NewMockopenSessionCounter(s.ctrl)
	s.excessQueue = mock.NewMockexcessQueueProducer(s.ctrl)
	s.authFlow = mock.NewMockauthFlowHandler(s.ctrl)
	s.creator = mock.NewMocktokenCreator(s.ctrl)
	s.storageOp = mock.NewMockoperationConsumer(s.ctrl)
	s.notifyCount = 0
	s.buildUC(4) // soft=4, hard=8
}

// authSuccessNotify - спай отложенного login-alert callback'а: считает фактические отправки
// user.authorization.success, чтобы проверить, что он уходит только после успешного commit'а.
func (s *OpenSessionSuite) authSuccessNotify() func(context.Context) {
	return func(context.Context) { s.notifyCount++ }
}

func confirmedOp(name string) secureoperation.SecureOperation {
	return secureoperation.SecureOperation{
		Token:   "op-token",
		Name:    name,
		UserID:  uuid.New(),
		Payload: []byte("payload"),
		Status:  operationstatus.Confirmed,
	}
}

// expectOpenSession - типовые ожидания удачного открытия сессии (после проверки лимита).
func (s *OpenSessionSuite) expectOpenSession(scopes dto.UserScopes) {
	s.authFlow.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(scopes, s.authSuccessNotify(), nil)
	s.tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runJob)
	s.issuer.EXPECT().Issue(gomock.Any(), gomock.Any()).Return(uint32(1), nil)
	s.creator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(okPair(), nil)
	s.storageOp.EXPECT().Delete(gomock.Any(), "op-token").Return(nil)
	s.activity.EXPECT().InsertOrUpdate(gomock.Any(), gomock.Any()).Return(nil)
}

func (s *OpenSessionSuite) TestNotConfirmed() {
	op := secureoperation.SecureOperation{Name: unit.NameConfirmCreateUser, Status: operationstatus.Opened}

	_, err := s.uc.Execute(s.ctx, dto.SessionMeta{}, op)
	s.Require().ErrorIs(err, secureoperation.ErrOperationIsNotConfirmed)
}

// happy: открытых сессий нет (soft=4 не достигнут) -> сигнал на чистку не ставится, вход проходит.
func (s *OpenSessionSuite) TestCreateUserHappy() {
	s.openCounter.EXPECT().FetchOpenSessionCount(gomock.Any(), gomock.Any(), gomock.Any()).Return(0, nil)
	s.expectOpenSession(okScopes())
	// excessQueue.Enqueue НЕ вызывается

	got, err := s.uc.Execute(s.ctx, dto.SessionMeta{}, confirmedOp(unit.NameConfirmCreateUser))
	s.Require().NoError(err)
	s.Equal(okPair(), got)
	s.Equal(1, s.notifyCount, "login-alert должен уйти ровно один раз после commit'а")
}

func (s *OpenSessionSuite) TestAuthorizeUserHappy() {
	s.openCounter.EXPECT().FetchOpenSessionCount(gomock.Any(), gomock.Any(), gomock.Any()).Return(0, nil)
	s.expectOpenSession(okScopes())

	_, err := s.uc.Execute(s.ctx, dto.SessionMeta{}, confirmedOp(unit.NameAuthorizeUser))
	s.Require().NoError(err)
	s.Equal(1, s.notifyCount, "login-alert должен уйти ровно один раз после commit'а")
}

// при достижении soft (N+1 >= 4) пользователь ставится в очередь на фоновую чистку, вход проходит.
func (s *OpenSessionSuite) TestSoftThresholdEnqueues() {
	scopes := okScopes()

	s.openCounter.EXPECT().FetchOpenSessionCount(gomock.Any(), scopes.UserID, testRealmID).Return(3, nil)
	s.excessQueue.EXPECT().Enqueue(gomock.Any(), scopes.UserID, testRealmID, 4).Return(nil)
	s.expectOpenSession(scopes)

	_, err := s.uc.Execute(s.ctx, dto.SessionMeta{}, confirmedOp(unit.NameConfirmCreateUser))
	s.Require().NoError(err)
}

// при достижении hard (N >= 8) вход временно отклоняется; сигнал на чистку всё равно ставится.
func (s *OpenSessionSuite) TestHardThresholdRejects() {
	scopes := okScopes()

	s.authFlow.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(scopes, s.authSuccessNotify(), nil)
	s.openCounter.EXPECT().FetchOpenSessionCount(gomock.Any(), scopes.UserID, testRealmID).Return(8, nil)
	s.excessQueue.EXPECT().Enqueue(gomock.Any(), scopes.UserID, testRealmID, 4).Return(nil)
	// tx.Do / Issue / Create не вызываются - вход отклонён до открытия транзакции

	_, err := s.uc.Execute(s.ctx, dto.SessionMeta{}, confirmedOp(unit.NameConfirmCreateUser))
	s.Require().ErrorIs(err, mrauth.ErrSessionLimitExceededTryLater)
	s.Zero(s.notifyCount, "на отказе hard-гейта login-alert не шлётся")
}

// сбой постановки в очередь best-effort: не должен валить успешный вход.
func (s *OpenSessionSuite) TestEnqueueErrorIgnored() {
	scopes := okScopes()

	s.openCounter.EXPECT().FetchOpenSessionCount(gomock.Any(), scopes.UserID, testRealmID).Return(3, nil)
	s.excessQueue.EXPECT().Enqueue(gomock.Any(), scopes.UserID, testRealmID, 4).Return(errors.New("enqueue failed"))
	s.expectOpenSession(scopes)

	got, err := s.uc.Execute(s.ctx, dto.SessionMeta{}, confirmedOp(unit.NameConfirmCreateUser))
	s.Require().NoError(err)
	s.Equal(okPair(), got)
}

// под soft-порогом (лимит 5: soft=4) две открытые сессии не дают сигнала на чистку.
func (s *OpenSessionSuite) TestUnderSoftNoEnqueue() {
	s.buildUC(5)

	s.openCounter.EXPECT().FetchOpenSessionCount(gomock.Any(), gomock.Any(), gomock.Any()).Return(2, nil)
	// excessQueue.Enqueue НЕ вызывается: N+1=3 < soft=4
	s.expectOpenSession(okScopes())

	_, err := s.uc.Execute(s.ctx, dto.SessionMeta{}, confirmedOp(unit.NameConfirmCreateUser))
	s.Require().NoError(err)
}

// session_max=0 для kind -> применяется дефолт (4): сигнал ставится с лимитом 4.
func (s *OpenSessionSuite) TestSessionMaxZeroUsesDefault() {
	s.buildUC(0)

	scopes := okScopes()

	s.openCounter.EXPECT().FetchOpenSessionCount(gomock.Any(), scopes.UserID, testRealmID).Return(3, nil)
	s.excessQueue.EXPECT().Enqueue(gomock.Any(), scopes.UserID, testRealmID, 4).Return(nil)
	s.expectOpenSession(scopes)

	_, err := s.uc.Execute(s.ctx, dto.SessionMeta{}, confirmedOp(unit.NameConfirmCreateUser))
	s.Require().NoError(err)
}

// явный hard-порог (offset=1) отклоняет вход раньше дефолтного: при limit=4 hard=5 (вместо 8).
func (s *OpenSessionSuite) TestCustomHardThresholdRejectsEarlier() {
	s.buildUCThresholds(4, 0, 1) // soft=limit+0=4, hard=limit+1=5

	scopes := okScopes()

	s.authFlow.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(scopes, s.authSuccessNotify(), nil)
	s.openCounter.EXPECT().FetchOpenSessionCount(gomock.Any(), scopes.UserID, testRealmID).Return(5, nil)
	s.excessQueue.EXPECT().Enqueue(gomock.Any(), scopes.UserID, testRealmID, 4).Return(nil)

	_, err := s.uc.Execute(s.ctx, dto.SessionMeta{}, confirmedOp(unit.NameConfirmCreateUser))
	s.Require().ErrorIs(err, mrauth.ErrSessionLimitExceededTryLater)
	s.Zero(s.notifyCount, "на отказе hard-гейта login-alert не шлётся")
}

// при limit=1 и отрицательном отклонении порог зажимается до 1, а не уходит в минус: первый
// вход (0 открытых сессий) не отклоняется (иначе был бы вечный лок).
func (s *OpenSessionSuite) TestThresholdClampMinOne() {
	s.buildUCThresholds(1, -5, -5) // soft=hard=max(1, 1-5)=1

	scopes := okScopes()

	s.openCounter.EXPECT().FetchOpenSessionCount(gomock.Any(), scopes.UserID, testRealmID).Return(0, nil)
	s.excessQueue.EXPECT().Enqueue(gomock.Any(), scopes.UserID, testRealmID, 1).Return(nil) // soft=1: 0+1>=1
	s.expectOpenSession(scopes)

	_, err := s.uc.Execute(s.ctx, dto.SessionMeta{}, confirmedOp(unit.NameConfirmCreateUser))
	s.Require().NoError(err)
}

func (s *OpenSessionSuite) TestSessionLimitFetchError() {
	s.authFlow.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(okScopes(), s.authSuccessNotify(), nil)
	s.openCounter.EXPECT().FetchOpenSessionCount(gomock.Any(), gomock.Any(), gomock.Any()).Return(0, errors.New("fetch failed"))
	// tx.Do / Enqueue не вызываются: подсчёт лимита идёт до них

	_, err := s.uc.Execute(s.ctx, dto.SessionMeta{}, confirmedOp(unit.NameConfirmCreateUser))
	s.Require().Error(err)
	s.Zero(s.notifyCount)
}

// TestOperationConsumeRace - оптимистичная сериализация: конкурентный запрос уже потребил
// (удалил) операцию, поэтому Delete затрагивает 0 строк и возвращает ErrEventStorageNoRecordFound;
// errorWrapper приводит его к ErrRecordNotFound -> транзакция открытия сессии откатывается.
func (s *OpenSessionSuite) TestOperationConsumeRace() {
	s.tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runJob)
	s.authFlow.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(okScopes(), s.authSuccessNotify(), nil)
	s.openCounter.EXPECT().FetchOpenSessionCount(gomock.Any(), gomock.Any(), gomock.Any()).Return(0, nil)
	s.creator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(okPair(), nil)
	s.issuer.EXPECT().Issue(gomock.Any(), gomock.Any()).Return(uint32(1), nil)
	s.storageOp.EXPECT().Delete(gomock.Any(), "op-token").Return(errors.ErrEventStorageNoRecordFound)

	_, err := s.uc.Execute(s.ctx, dto.SessionMeta{}, confirmedOp(unit.NameConfirmCreateUser))
	s.Require().ErrorIs(err, errors.ErrRecordNotFound)
	s.Zero(s.notifyCount, "при откате транзакции login-alert не шлётся")
}

// TestOperationDeleteError - сбой потребления операции откатывает транзакцию открытия сессии.
func (s *OpenSessionSuite) TestOperationDeleteError() {
	s.tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runJob)
	s.authFlow.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(okScopes(), s.authSuccessNotify(), nil)
	s.openCounter.EXPECT().FetchOpenSessionCount(gomock.Any(), gomock.Any(), gomock.Any()).Return(0, nil)
	s.creator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(okPair(), nil)
	s.issuer.EXPECT().Issue(gomock.Any(), gomock.Any()).Return(uint32(1), nil)
	s.storageOp.EXPECT().Delete(gomock.Any(), "op-token").Return(errors.New("delete failed"))

	_, err := s.uc.Execute(s.ctx, dto.SessionMeta{}, confirmedOp(unit.NameConfirmCreateUser))
	s.Require().Error(err)
	s.Zero(s.notifyCount, "при откате транзакции login-alert не шлётся")
}

func (s *OpenSessionSuite) TestUnknownOperation() {
	// неизвестная операция отклоняется до обработчика и транзакции
	_, err := s.uc.Execute(s.ctx, dto.SessionMeta{}, confirmedOp("unknown.operation"))
	s.Require().Error(err)
}

func (s *OpenSessionSuite) TestHandlerError() {
	// обработчик выполняется до транзакции: его ошибка возвращается без вызова tx.Do
	s.authFlow.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(dto.UserScopes{}, nil, errors.New("handler failed"))

	_, err := s.uc.Execute(s.ctx, dto.SessionMeta{}, confirmedOp(unit.NameConfirmCreateUser))
	s.Require().Error(err)
	s.Zero(s.notifyCount)
}

func (s *OpenSessionSuite) TestTokenCreatorError() {
	s.tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runJob)
	s.authFlow.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(okScopes(), s.authSuccessNotify(), nil)
	s.openCounter.EXPECT().FetchOpenSessionCount(gomock.Any(), gomock.Any(), gomock.Any()).Return(0, nil)
	// сессия выпускается до токенов, поэтому Issue вызывается раньше падающего Create
	s.issuer.EXPECT().Issue(gomock.Any(), gomock.Any()).Return(uint32(1), nil)
	s.creator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(dto.AuthTokenPair{}, errors.New("create failed"))

	_, err := s.uc.Execute(s.ctx, dto.SessionMeta{}, confirmedOp(unit.NameConfirmCreateUser))
	s.Require().Error(err)
	s.Zero(s.notifyCount, "при откате транзакции login-alert не шлётся")
}

// TestActivityErrorIgnored - запись активности идёт вне транзакции best-effort: её сбой не должен
// проваливать уже открытую сессию (commit прошёл, токены выданы), Execute возвращает токен без ошибки.
func (s *OpenSessionSuite) TestActivityErrorIgnored() {
	s.tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runJob)
	s.authFlow.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(okScopes(), s.authSuccessNotify(), nil)
	s.openCounter.EXPECT().FetchOpenSessionCount(gomock.Any(), gomock.Any(), gomock.Any()).Return(0, nil)
	s.creator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(okPair(), nil)
	s.issuer.EXPECT().Issue(gomock.Any(), gomock.Any()).Return(uint32(1), nil)
	s.storageOp.EXPECT().Delete(gomock.Any(), "op-token").Return(nil)
	s.activity.EXPECT().InsertOrUpdate(gomock.Any(), gomock.Any()).Return(errors.New("activity failed"))

	got, err := s.uc.Execute(s.ctx, dto.SessionMeta{}, confirmedOp(unit.NameConfirmCreateUser))
	s.Require().NoError(err)
	s.Equal(okPair(), got)
	s.Equal(1, s.notifyCount, "commit прошёл - login-alert уходит несмотря на сбой записи активности")
}

func (s *OpenSessionSuite) TestSessionIssuerError() {
	s.tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runJob)
	s.authFlow.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(okScopes(), s.authSuccessNotify(), nil)
	s.openCounter.EXPECT().FetchOpenSessionCount(gomock.Any(), gomock.Any(), gomock.Any()).Return(0, nil)
	// issuer не смог выдать session_id -> Create и InsertOrUpdate не вызываются
	s.issuer.EXPECT().Issue(gomock.Any(), gomock.Any()).Return(uint32(0), errors.ErrEventRecordAlreadyExists)

	_, err := s.uc.Execute(s.ctx, dto.SessionMeta{}, confirmedOp(unit.NameConfirmCreateUser))
	s.Require().ErrorIs(err, errors.ErrEventRecordAlreadyExists)
	s.Zero(s.notifyCount, "при откате транзакции login-alert не шлётся")
}

// первый вход отклонён hard-гейтом (login-alert не уходит), повтор проходит -> суммарно
// user.authorization.success отправляется ровно один раз.
func (s *OpenSessionSuite) TestHardRejectThenSuccessNotifiesOnce() {
	scopes := okScopes()
	notify := s.authSuccessNotify()

	s.authFlow.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(scopes, notify, nil).Times(2)
	gomock.InOrder(
		s.openCounter.EXPECT().FetchOpenSessionCount(gomock.Any(), scopes.UserID, testRealmID).Return(8, nil),
		s.openCounter.EXPECT().FetchOpenSessionCount(gomock.Any(), scopes.UserID, testRealmID).Return(0, nil),
	)
	// первый вход (N=8 >= soft): сигнал на чистку + отказ hard-гейта
	s.excessQueue.EXPECT().Enqueue(gomock.Any(), scopes.UserID, testRealmID, 4).Return(nil)
	// второй вход (N=0): успешное открытие сессии
	s.tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runJob)
	s.issuer.EXPECT().Issue(gomock.Any(), gomock.Any()).Return(uint32(1), nil)
	s.creator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(okPair(), nil)
	s.storageOp.EXPECT().Delete(gomock.Any(), "op-token").Return(nil)
	s.activity.EXPECT().InsertOrUpdate(gomock.Any(), gomock.Any()).Return(nil)

	_, err := s.uc.Execute(s.ctx, dto.SessionMeta{}, confirmedOp(unit.NameConfirmCreateUser))
	s.Require().ErrorIs(err, mrauth.ErrSessionLimitExceededTryLater)
	s.Zero(s.notifyCount)

	_, err = s.uc.Execute(s.ctx, dto.SessionMeta{}, confirmedOp(unit.NameConfirmCreateUser))
	s.Require().NoError(err)
	s.Equal(1, s.notifyCount, "суммарно за ретрай login-alert уходит ровно один раз")
}

// ----- ContinueSession -----

type ContinueSessionSuite struct {
	suite.Suite

	ctrl      *gomock.Controller
	ctx       context.Context
	storage   *mock.MockauthTokenStorage
	recreator *mock.MocktokenRecreator
	emitter   *mock.MockEmitter
	uc        *session.ContinueSession
}

func TestContinueSessionSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ContinueSessionSuite))
}

func (s *ContinueSessionSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.storage = mock.NewMockauthTokenStorage(s.ctrl)
	s.recreator = mock.NewMocktokenRecreator(s.ctrl)
	s.emitter = mock.NewMockEmitter(s.ctrl)
	s.uc = session.NewContinueSession(s.storage, s.recreator, s.emitter, mrlog.NopLogger())
}

func (s *ContinueSessionSuite) TestEmptyToken() {
	_, err := s.uc.Execute(s.ctx, "en", "")
	s.Require().Error(err)
}

func (s *ContinueSessionSuite) TestHappy() {
	s.recreator.EXPECT().Recreate(gomock.Any(), "rt").Return(okPair(), nil)

	got, err := s.uc.Execute(s.ctx, "en", "rt")
	s.Require().NoError(err)
	s.Equal(okPair(), got)
}

func (s *ContinueSessionSuite) TestReuseRevokesSession() {
	userID := uuid.New()
	revokedErr := repository.NewTokenAlreadyRevokedError(userID, 123)
	s.recreator.EXPECT().Recreate(gomock.Any(), "rt").Return(dto.AuthTokenPair{}, revokedErr)
	s.storage.EXPECT().RevokeTokensBySessionID(gomock.Any(), userID, uint32(123)).Return(nil)
	s.emitter.EXPECT().Emit(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

	_, err := s.uc.Execute(s.ctx, "en", "rt")
	s.Require().ErrorIs(err, mrauth.ErrTokenNotFoundOrExpired)
}

func (s *ContinueSessionSuite) TestNoRecordFound() {
	s.recreator.EXPECT().Recreate(gomock.Any(), "rt").
		Return(dto.AuthTokenPair{}, errors.ErrEventStorageNoRecordFound)

	_, err := s.uc.Execute(s.ctx, "en", "rt")
	s.Require().ErrorIs(err, mrauth.ErrTokenNotFoundOrExpired)
}

func (s *ContinueSessionSuite) TestTokenExpired() {
	s.recreator.EXPECT().Recreate(gomock.Any(), "rt").
		Return(dto.AuthTokenPair{}, repository.ErrTokenExpired)

	_, err := s.uc.Execute(s.ctx, "en", "rt")
	s.Require().ErrorIs(err, mrauth.ErrTokenNotFoundOrExpired)
}

func (s *ContinueSessionSuite) TestOtherError() {
	s.recreator.EXPECT().Recreate(gomock.Any(), "rt").
		Return(dto.AuthTokenPair{}, errors.New("db down"))

	_, err := s.uc.Execute(s.ctx, "en", "rt")
	s.Require().Error(err)
	s.NotErrorIs(err, mrauth.ErrTokenNotFoundOrExpired)
}

// ----- CloseSession -----

type CloseSessionSuite struct {
	suite.Suite

	ctrl   *gomock.Controller
	ctx    context.Context
	closer *mock.MocktokenCloser
	uc     *session.CloseSession
}

func TestCloseSessionSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(CloseSessionSuite))
}

func (s *CloseSessionSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.closer = mock.NewMocktokenCloser(s.ctrl)
	s.uc = session.NewCloseSession(s.closer)
}

func (s *CloseSessionSuite) TestEmptyToken() {
	s.Require().Error(s.uc.Execute(s.ctx, ""))
}

func (s *CloseSessionSuite) TestSuccess() {
	s.closer.EXPECT().Close(gomock.Any(), "rt").Return(nil)

	s.Require().NoError(s.uc.Execute(s.ctx, "rt"))
}

func (s *CloseSessionSuite) TestNoRecordFound() {
	s.closer.EXPECT().Close(gomock.Any(), "rt").Return(errors.ErrEventStorageNoRecordFound)

	err := s.uc.Execute(s.ctx, "rt")
	s.Require().ErrorIs(err, mrauth.ErrTokenInvalid)
}

func (s *CloseSessionSuite) TestOtherError() {
	s.closer.EXPECT().Close(gomock.Any(), "rt").Return(errors.New("db down"))

	err := s.uc.Execute(s.ctx, "rt")
	s.Require().Error(err)
	s.NotErrorIs(err, mrauth.ErrTokenNotFoundOrExpired)
}

// ----- List -----

type ListSuite struct {
	suite.Suite

	ctrl      *gomock.Controller
	ctx       context.Context
	lister    *mock.MocksessionLister
	opener    *mock.MockopenSessionFetcher
	closer    *mock.MocksessionCloser
	resolver  *mock.MocksessionResolver
	userRealm *mock.MockuserRealmFetcher
	userID    uuid.UUID
	uc        *session.List
}

func TestListSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ListSuite))
}

func (s *ListSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.lister = mock.NewMocksessionLister(s.ctrl)
	s.opener = mock.NewMockopenSessionFetcher(s.ctrl)
	s.closer = mock.NewMocksessionCloser(s.ctrl)
	s.resolver = mock.NewMocksessionResolver(s.ctrl)
	s.userRealm = mock.NewMockuserRealmFetcher(s.ctrl)
	s.userID = uuid.New()
	s.uc = session.NewList(s.lister, s.opener, s.closer, s.resolver, s.userRealm, testRealmRegistry(), nil, nil, nil)
}

func (s *ListSuite) TestGetListFiltersAndMaps() {
	// 0xdeadbeef нет среди открытых -> должна быть отброшена; порядок результата следует порядку строк
	createdAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	lastSeen := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	// FetchOrderedListByUserIDAndSessionIDs возвращает метаданные ровно по открытым сессиям
	// (фильтрация и порядок - на стороне репозитория)
	rows := []entity.Session{
		{UserID: s.userID, SessionID: 0x1f3bc817, UserAgent: "UA1", LastIP: 0x7f000001, CreatedAt: createdAt, UpdatedAt: lastSeen}, // 127.0.0.1, текущая
		{UserID: s.userID, SessionID: 0x0000babc, UserAgent: "UA2", LastIP: 0},                                                     // IP=0 -> ""
	}
	open := []uint32{0x0000babc, 0x1f3bc817}

	s.opener.EXPECT().FetchOpenSessionIDs(gomock.Any(), s.userID, testRealmID).Return(open, nil)
	s.resolver.EXPECT().FetchOneByAccessToken(gomock.Any(), "acc").Return(dto.UserScopes{SessionID: 0x1f3bc817, Realm: testRealm}, nil)
	s.lister.EXPECT().FetchOrderedListByUserIDAndSessionIDs(gomock.Any(), s.userID, open, 4).Return(rows, nil)

	got, err := s.uc.GetList(s.ctx, s.userID, "acc", "")
	s.Require().NoError(err)
	s.Require().Len(got, 2)

	s.Equal(uint32(0x1f3bc817), got[0].SessionID)
	s.True(got[0].IsCurrent) // session_id совпал с текущим
	s.Equal("127.0.0.1", got[0].LastIP)
	s.Equal(createdAt, got[0].CreatedAt)
	s.Equal(lastSeen, got[0].UpdatedAt)

	s.Equal(uint32(0x0000babc), got[1].SessionID)
	s.False(got[1].IsCurrent)
	s.Empty(got[1].LastIP)
}

// при явно указанном чужом realm членство проверяется через userRealm.FetchOne (даёт kind этого
// realm), сессии скоупятся по нему, а логика текущей сессии (инвариант/догрузка) пропускается:
// текущая сессия принадлежит realm токена, поэтому IsCurrent во всей выдаче false.
func (s *ListSuite) TestGetListForeignRealm() {
	uc := session.NewList(
		s.lister,
		s.opener,
		s.closer,
		s.resolver,
		s.userRealm,
		testRealmRegistry(),
		nil,
		nil,
		[]session.LimitRealm{{
			ID:         altRealmID,
			KindLimits: []session.UserKindLimit{{Kind: "k", SessionMax: 2}},
		}},
	)

	open := []uint32{1, 2}
	rows := []entity.Session{
		{UserID: s.userID, SessionID: 1},
		{UserID: s.userID, SessionID: 2},
	}

	// текущий токен realm "site/admin", запрошен другой realm "r" (altRealmID); текущая сессия
	// (99) принадлежит realm токена и в списке чужого realm не встречается
	s.resolver.EXPECT().FetchOneByAccessToken(gomock.Any(), "acc").Return(dto.UserScopes{SessionID: 99, Realm: testRealm, Kind: "other"}, nil)
	s.userRealm.EXPECT().FetchOne(gomock.Any(), s.userID, altRealmID).Return(entity.UserRealm{Kind: "k"}, nil)
	s.opener.EXPECT().FetchOpenSessionIDs(gomock.Any(), s.userID, altRealmID).Return(open, nil)
	// лимит берётся по kind чужого realm ("k" -> 2), а не по kind токена
	s.lister.EXPECT().FetchOrderedListByUserIDAndSessionIDs(gomock.Any(), s.userID, open, 2).Return(rows, nil)

	got, err := uc.GetList(s.ctx, s.userID, "acc", altRealm)
	s.Require().NoError(err)
	s.Require().Len(got, 2)
	s.False(got[0].IsCurrent)
	s.False(got[1].IsCurrent)
}

// чужой realm без открытых сессий: инвариант текущей сессии пропущен, выдача пуста (без паники).
func (s *ListSuite) TestGetListForeignRealmEmpty() {
	s.resolver.EXPECT().FetchOneByAccessToken(gomock.Any(), "acc").Return(dto.UserScopes{SessionID: 1, Realm: testRealm}, nil)
	s.userRealm.EXPECT().FetchOne(gomock.Any(), s.userID, altRealmID).Return(entity.UserRealm{Kind: "k"}, nil)
	s.opener.EXPECT().FetchOpenSessionIDs(gomock.Any(), s.userID, altRealmID).Return([]uint32{}, nil)
	s.lister.EXPECT().FetchOrderedListByUserIDAndSessionIDs(gomock.Any(), s.userID, []uint32{}, gomock.Any()).Return(nil, nil)

	got, err := s.uc.GetList(s.ctx, s.userID, "acc", altRealm)
	s.Require().NoError(err)
	s.Empty(got)
}

// пользователь не является членом запрошенного realm (нет привязки): доступ к чужим сессиям
// запрещён - AccessForbidden (403), открытые сессии не выбираются.
func (s *ListSuite) TestGetListForeignRealmNotMemberFails() {
	s.resolver.EXPECT().FetchOneByAccessToken(gomock.Any(), "acc").Return(dto.UserScopes{SessionID: 1, Realm: testRealm}, nil)
	s.userRealm.EXPECT().FetchOne(gomock.Any(), s.userID, altRealmID).Return(entity.UserRealm{}, errors.ErrEventStorageNoRecordFound)

	_, err := s.uc.GetList(s.ctx, s.userID, "acc", altRealm)
	s.Require().ErrorIs(err, errors.ErrAccessForbidden)
}

// неизвестное имя realm от клиента - клиентская ошибка (не Internal), userRealm/opener не вызываются.
func (s *ListSuite) TestGetListUnknownRealmFails() {
	s.resolver.EXPECT().FetchOneByAccessToken(gomock.Any(), "acc").Return(dto.UserScopes{SessionID: 1, Realm: testRealm}, nil)

	_, err := s.uc.GetList(s.ctx, s.userID, "acc", "unknown/realm")
	s.Require().ErrorIs(err, errors.ErrIncorrectInputData)
}

func (s *ListSuite) TestGetListEmptyOpenSetFails() {
	// пустой набор открытых сессий - нарушение инварианта (текущая сессия обязана там быть):
	// Internal-ошибка, lister.FetchOrdered... НЕ вызывается.
	s.resolver.EXPECT().FetchOneByAccessToken(gomock.Any(), "acc").Return(dto.UserScopes{SessionID: 1, Realm: testRealm}, nil)
	s.opener.EXPECT().FetchOpenSessionIDs(gomock.Any(), s.userID, testRealmID).Return([]uint32{}, nil)

	_, err := s.uc.GetList(s.ctx, s.userID, "acc", "")
	s.Require().ErrorIs(err, errors.ErrInternalIncorrectInputData)
}

func (s *ListSuite) TestGetListCurrentSessionNotOpenFails() {
	// текущая сессия отсутствует среди открытых - нарушение инварианта: Internal-ошибка,
	// lister.FetchOrdered... НЕ вызывается.
	s.resolver.EXPECT().FetchOneByAccessToken(gomock.Any(), "acc").Return(dto.UserScopes{SessionID: 99, Realm: testRealm}, nil)
	s.opener.EXPECT().FetchOpenSessionIDs(gomock.Any(), s.userID, testRealmID).Return([]uint32{1, 2}, nil)

	_, err := s.uc.GetList(s.ctx, s.userID, "acc", "")
	s.Require().ErrorIs(err, errors.ErrInternalIncorrectInputData)
}

// текущая сессия, выпавшая за пределы лимита, догружается отдельным запросом и заменяет
// последнюю (наименее активную) строку - в выдаче она присутствует с IsCurrent=true.
func (s *ListSuite) TestGetListCurrentSessionOutsideLimitRefetched() {
	uc := session.NewList(
		s.lister,
		s.opener,
		s.closer,
		s.resolver,
		s.userRealm,
		testRealmRegistry(),
		nil,
		nil,
		[]session.LimitRealm{{
			ID:         altRealmID,
			KindLimits: []session.UserKindLimit{{Kind: "k", SessionMax: 2}},
		}},
	)

	open := []uint32{1, 2, 3}
	// репозиторий обрезал по лимиту 2, текущая сессия 3 (наименее активная) выпала из выдачи
	rows := []entity.Session{
		{UserID: s.userID, SessionID: 1},
		{UserID: s.userID, SessionID: 2},
	}
	current := []entity.Session{{UserID: s.userID, SessionID: 3, UserAgent: "UA3"}}

	s.opener.EXPECT().FetchOpenSessionIDs(gomock.Any(), s.userID, altRealmID).Return(open, nil)
	s.resolver.EXPECT().FetchOneByAccessToken(gomock.Any(), "acc").
		Return(dto.UserScopes{SessionID: 3, Realm: "r", Kind: "k"}, nil)
	s.lister.EXPECT().FetchOrderedListByUserIDAndSessionIDs(gomock.Any(), s.userID, open, 2).Return(rows, nil)
	// догрузка одной текущей сессии (limit=0)
	s.lister.EXPECT().FetchOrderedListByUserIDAndSessionIDs(gomock.Any(), s.userID, []uint32{3}, 0).Return(current, nil)

	got, err := uc.GetList(s.ctx, s.userID, "acc", "")
	s.Require().NoError(err)
	s.Require().Len(got, 2)
	s.Equal(uint32(1), got[0].SessionID)
	s.False(got[0].IsCurrent)
	// последняя строка заменена текущей сессией
	s.Equal(uint32(3), got[1].SessionID)
	s.True(got[1].IsCurrent)
}

func (s *ListSuite) TestGetListCurrentSessionRefetchEmptyFails() {
	uc := session.NewList(
		s.lister,
		s.opener,
		s.closer,
		s.resolver,
		s.userRealm,
		testRealmRegistry(),
		nil,
		nil,
		[]session.LimitRealm{{
			ID:         altRealmID,
			KindLimits: []session.UserKindLimit{{Kind: "k", SessionMax: 2}},
		}},
	)

	open := []uint32{1, 2, 3}
	rows := []entity.Session{
		{UserID: s.userID, SessionID: 1},
		{UserID: s.userID, SessionID: 2},
	}

	s.opener.EXPECT().FetchOpenSessionIDs(gomock.Any(), s.userID, altRealmID).Return(open, nil)
	s.resolver.EXPECT().FetchOneByAccessToken(gomock.Any(), "acc").
		Return(dto.UserScopes{SessionID: 3, Realm: "r", Kind: "k"}, nil)
	s.lister.EXPECT().FetchOrderedListByUserIDAndSessionIDs(gomock.Any(), s.userID, open, 2).Return(rows, nil)
	// догрузка текущей сессии вернула пусто - Internal-ошибка
	s.lister.EXPECT().FetchOrderedListByUserIDAndSessionIDs(gomock.Any(), s.userID, []uint32{3}, 0).Return(nil, nil)

	_, err := uc.GetList(s.ctx, s.userID, "acc", "")
	s.Require().ErrorIs(err, errors.ErrInternalIncorrectInputData)
}

func (s *ListSuite) TestGetListResolversEnrich() {
	uc := session.NewList(
		s.lister,
		s.opener,
		s.closer,
		s.resolver,
		s.userRealm,
		testRealmRegistry(),
		func(ua string) (string, string) { return "app:" + ua, "dev:" + ua },
		func(ip string) string { return "loc:" + ip },
		nil,
	)

	rows := []entity.Session{{UserID: s.userID, SessionID: 1, UserAgent: "UA", LastIP: 0x7f000001}}

	s.opener.EXPECT().FetchOpenSessionIDs(gomock.Any(), s.userID, testRealmID).Return([]uint32{1}, nil)
	s.resolver.EXPECT().FetchOneByAccessToken(gomock.Any(), "acc").Return(dto.UserScopes{SessionID: 1, Realm: testRealm}, nil)
	s.lister.EXPECT().FetchOrderedListByUserIDAndSessionIDs(gomock.Any(), s.userID, []uint32{1}, 4).Return(rows, nil)

	got, err := uc.GetList(s.ctx, s.userID, "acc", "")
	s.Require().NoError(err)
	s.Require().Len(got, 1)
	s.Equal("app:UA", got[0].AppName)
	s.Equal("dev:UA", got[0].DeviceName)
	s.Equal("loc:127.0.0.1", got[0].Location)
}

func (s *ListSuite) TestGetListResolverErrorFatal() {
	// резолв текущего токена идёт первым и фатально: из scopes берутся Realm/Kind для лимита,
	// поэтому при его ошибке opener и lister не вызываются, а GetList возвращает ошибку.
	s.resolver.EXPECT().FetchOneByAccessToken(gomock.Any(), "acc").Return(dto.UserScopes{}, errors.New("token revoked"))

	got, err := s.uc.GetList(s.ctx, s.userID, "acc", "")
	s.Require().Error(err)
	s.Empty(got)
}

// при превышении лимита показываются только новейшие сессии в его рамках.
func (s *ListSuite) TestGetListPassesLimitAndPreservesOrder() {
	uc := session.NewList(
		s.lister,
		s.opener,
		s.closer,
		s.resolver,
		s.userRealm,
		testRealmRegistry(),
		nil,
		nil,
		[]session.LimitRealm{{
			ID:         altRealmID,
			KindLimits: []session.UserKindLimit{{Kind: "k", SessionMax: 2}},
		}},
	)

	t2 := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)
	// репозиторий уже обрезал по лимиту и упорядочил новыми вперёд; usecase лишь сохраняет порядок
	rows := []entity.Session{
		{UserID: s.userID, SessionID: 3, CreatedAt: t3},
		{UserID: s.userID, SessionID: 2, CreatedAt: t2},
	}
	open := []uint32{1, 2, 3}

	s.opener.EXPECT().FetchOpenSessionIDs(gomock.Any(), s.userID, altRealmID).Return(open, nil)
	s.resolver.EXPECT().FetchOneByAccessToken(gomock.Any(), "acc").
		Return(dto.UserScopes{SessionID: 3, Realm: "r", Kind: "k"}, nil)
	// лимит realm "r"/kind "k" == 2 передаётся в репозиторий
	s.lister.EXPECT().FetchOrderedListByUserIDAndSessionIDs(gomock.Any(), s.userID, open, 2).Return(rows, nil)

	got, err := uc.GetList(s.ctx, s.userID, "acc", "")
	s.Require().NoError(err)
	s.Require().Len(got, 2)
	s.Equal(uint32(3), got[0].SessionID)
	s.Equal(uint32(2), got[1].SessionID)
}

func (s *ListSuite) TestGetListOpenFetcherError() {
	// резолв токена проходит, но выборка открытых сессий падает -> lister не вызывается
	s.resolver.EXPECT().FetchOneByAccessToken(gomock.Any(), "acc").Return(dto.UserScopes{SessionID: 1, Realm: testRealm}, nil)
	s.opener.EXPECT().FetchOpenSessionIDs(gomock.Any(), s.userID, testRealmID).Return(nil, errors.New("db down"))

	_, err := s.uc.GetList(s.ctx, s.userID, "acc", "")
	s.Require().Error(err)
}

func (s *ListSuite) TestGetListListerError() {
	s.opener.EXPECT().FetchOpenSessionIDs(gomock.Any(), s.userID, testRealmID).Return([]uint32{1}, nil)
	s.resolver.EXPECT().FetchOneByAccessToken(gomock.Any(), "acc").Return(dto.UserScopes{SessionID: 1, Realm: testRealm}, nil)
	s.lister.EXPECT().FetchOrderedListByUserIDAndSessionIDs(gomock.Any(), s.userID, []uint32{1}, 4).Return(nil, errors.New("db down"))

	_, err := s.uc.GetList(s.ctx, s.userID, "acc", "")
	s.Require().Error(err)
}

func (s *ListSuite) TestCloseEmptyInput() {
	// closer.RevokeTokensBySessionIDs НЕ должен вызываться
	s.Require().Error(s.uc.Close(s.ctx, s.userID, nil))
}

func (s *ListSuite) TestCloseSuccess() {
	ids := []uint32{0x1f3bc817, 0x0000babc}
	s.closer.EXPECT().RevokeTokensBySessionIDs(gomock.Any(), s.userID, ids).Return(nil)

	s.Require().NoError(s.uc.Close(s.ctx, s.userID, ids))
}

func (s *ListSuite) TestCloseError() {
	ids := []uint32{1}
	s.closer.EXPECT().RevokeTokensBySessionIDs(gomock.Any(), s.userID, ids).Return(errors.New("db down"))

	s.Require().Error(s.uc.Close(s.ctx, s.userID, ids))
}
