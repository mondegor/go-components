package operation_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	sysmesserrors "github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/logstatus"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/usecase/operation"
	"github.com/mondegor/go-components/mrauth/usecase/operation/mock"
)

//go:generate mockgen -source=confirm_operation.go -destination=mock/confirm_operation.go -package=mock
//go:generate mockgen -source=resend_code.go -destination=mock/resend_code.go -package=mock
//go:generate mockgen -source=revoke_operation.go -destination=mock/revoke_operation.go -package=mock
//go:generate mockgen -source=operation_statistic.go -destination=mock/operation_statistic.go -package=mock
//go:generate mockgen -destination=mock/mrstorage.go -package=mock github.com/mondegor/go-core/mrstorage DBTxManager
//go:generate mockgen -destination=mock/mrnotifier.go -package=mock github.com/mondegor/go-components/mrnotifier NoteProducer

// expectPassThroughTx - транзакция выполняет переданное задание как есть.
func expectPassThroughTx(txManager *mock.MockDBTxManager) {
	txManager.EXPECT().
		Do(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, job func(ctx context.Context) error, _ ...mrstorage.TxOption) error {
			return job(ctx)
		}).
		AnyTimes()
}

func openedEmailOp(t *testing.T) secureoperation.SecureOperation {
	t.Helper()

	op, err := secureoperation.NewOperation(
		"token",
		"op.name",
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

func confirmedOp(t *testing.T) secureoperation.SecureOperation {
	t.Helper()

	op := secureoperation.SecureOperation{
		Token:     "token",
		Name:      "op.name",
		UserID:    uuid.New(),
		Status:    operationstatus.Confirmed,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	require.NoError(t, secureoperation.WakeUp(&op, nil))

	return op
}

type ConfirmOperationSuite struct {
	suite.Suite

	ctrl         *gomock.Controller
	ctx          context.Context
	txManager    *mock.MockDBTxManager
	storage      *mock.MockoperationConfirmer
	notifierAPI  *mock.MockNoteProducer
	preparer     *mock.MockconfirmOperationPreparer
	logOperation *mock.MockoperationLogger
	logEntries   []entity.SecureOperationLog
	uc           *operation.ConfirmOperation
}

func TestConfirmOperationSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ConfirmOperationSuite))
}

func (s *ConfirmOperationSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.txManager = mock.NewMockDBTxManager(s.ctrl)
	s.storage = mock.NewMockoperationConfirmer(s.ctrl)
	s.notifierAPI = mock.NewMockNoteProducer(s.ctrl)
	s.preparer = mock.NewMockconfirmOperationPreparer(s.ctrl)
	s.logOperation = mock.NewMockoperationLogger(s.ctrl)
	s.logEntries = nil

	expectPassThroughTx(s.txManager)
	s.logOperation.EXPECT().
		Log(gomock.Any(), gomock.Any()).
		Do(func(_ context.Context, entry entity.SecureOperationLog) {
			s.logEntries = append(s.logEntries, entry)
		}).
		AnyTimes()

	s.uc = operation.NewConfirmOperation(s.txManager, s.storage, s.notifierAPI, s.preparer, s.logOperation)
}

func (s *ConfirmOperationSuite) expectFetch(op secureoperation.SecureOperation, err error) {
	s.storage.EXPECT().FetchOneForUpdate(gomock.Any(), gomock.Any()).Return(op, err).AnyTimes()
}

func (s *ConfirmOperationSuite) expectPrepare(
	op secureoperation.SecureOperation,
	commit func(ctx context.Context) error,
	err error,
) {
	s.preparer.EXPECT().
		Prepare(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(op, commit, err).
		AnyTimes()
}

func (s *ConfirmOperationSuite) execute(code string) (secureoperation.SecureOperation, error) {
	return s.uc.Execute(s.ctx, dto.ActorMeta{}, "en", "token", code)
}

// пустым токеном не может быть найдена ни одна операция: снаружи это неотличимо
// от неизвестного и истёкшего токена и отдаётся тем же сентинелом.
func (s *ConfirmOperationSuite) TestEmptyToken() {
	_, err := s.uc.Execute(s.ctx, dto.ActorMeta{}, "en", "", "code")
	s.Require().ErrorIs(err, secureoperation.ErrOperationInvalid)
}

// язык приходит уже определённым по запросу, поэтому пустой - ошибка проводки.
func (s *ConfirmOperationSuite) TestEmptyLangCode() {
	s.storage.EXPECT().FetchOneForUpdate(gomock.Any(), gomock.Any()).Times(0)

	_, err := s.uc.Execute(s.ctx, dto.ActorMeta{}, "", "token", "code")
	s.Require().ErrorIs(err, sysmesserrors.ErrInternalIncorrectInputData)
}

// TestUnknownTokenIsDomainError - операции по предъявленному токену нет: usecase обязан сам
// перевести отсутствие записи в доменную ошибку, не полагаясь на перевод в контроллере.
func (s *ConfirmOperationSuite) TestUnknownTokenIsDomainError() {
	s.expectFetch(secureoperation.SecureOperation{}, sysmesserrors.ErrEventStorageNoRecordFound)

	_, err := s.execute("code")
	s.Require().ErrorIs(err, secureoperation.ErrOperationInvalid)
	s.Require().NotErrorIs(err, sysmesserrors.ErrRecordNotFound)
}

// TestRecordNotFoundAfterLockIsInternal - строка уже выбрана через FetchOneForUpdate, то есть
// заблокирована в этой же транзакции: её исчезновение на записи - нарушение инварианта, а не
// недействительный токен. Перевод "записи нет" вешается только на поиск по токену, иначе
// повреждение данных замаскируется под обычный ответ клиенту.
func (s *ConfirmOperationSuite) TestRecordNotFoundAfterLockIsInternal() {
	op := openedEmailOp(s.T())
	s.expectFetch(op, nil)
	s.expectPrepare(op, nil, nil)
	s.storage.EXPECT().
		Replace(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(sysmesserrors.ErrEventStorageNoRecordFound)

	_, err := s.execute("code123")
	s.Require().Error(err)
	s.Require().NotErrorIs(err, secureoperation.ErrOperationInvalid)
	s.Require().NotErrorIs(err, sysmesserrors.ErrRecordNotFound)
}

func (s *ConfirmOperationSuite) TestFetchError() {
	wantErr := errors.New("fetch failed")
	s.expectFetch(secureoperation.SecureOperation{}, wantErr)

	_, err := s.execute("code")
	s.Require().ErrorIs(err, wantErr)
}

func (s *ConfirmOperationSuite) TestNoAttempts() {
	op := openedEmailOp(s.T())
	s.expectFetch(op, nil)
	s.expectPrepare(op, nil, secureoperation.ErrNoAttemptsToConfirmOperation)

	_, err := s.execute("code")
	s.Require().ErrorIs(err, secureoperation.ErrNoAttemptsToConfirmOperation)
	s.Require().Len(s.logEntries, 1)
	s.Equal(logstatus.Blocked, s.logEntries[0].LogStatus)
	s.Equal(logreason.AttemptsExhausted, s.logEntries[0].Reason)
}

func (s *ConfirmOperationSuite) TestWrongCodeAttemptsRemain() {
	op := openedEmailOp(s.T())
	s.expectFetch(op, nil)
	s.expectPrepare(op, nil, secureoperation.ErrConfirmCodeIsIncorrect)
	s.storage.EXPECT().UpdateFailedAttempt(gomock.Any(), gomock.Any()).Return(int16(2), nil)

	out, err := s.execute("bad")
	s.Require().ErrorIs(err, secureoperation.ErrConfirmCodeIsIncorrect)
	s.Equal(int16(2), out.RemainingAttempts)
	s.Require().Len(s.logEntries, 1)
	s.Equal(logstatus.ConfirmFailed, s.logEntries[0].LogStatus)
	s.Equal(logreason.WrongCode, s.logEntries[0].Reason)
	// поток анонимный, но владелец операции известен после её чтения
	s.Equal(op.UserID, s.logEntries[0].VisitorID)
}

func (s *ConfirmOperationSuite) TestWrongCodeNoAttemptsLeft() {
	op := openedEmailOp(s.T())
	s.expectFetch(op, nil)
	s.expectPrepare(op, nil, secureoperation.ErrConfirmCodeIsIncorrect)
	s.storage.EXPECT().UpdateFailedAttempt(gomock.Any(), gomock.Any()).Return(int16(0), nil)

	_, err := s.execute("bad")
	s.Require().ErrorIs(err, secureoperation.ErrNoAttemptsToConfirmOperation)
	s.Require().Len(s.logEntries, 1)
	s.Equal(logstatus.Blocked, s.logEntries[0].LogStatus)
	s.Equal(logreason.AttemptsExhausted, s.logEntries[0].Reason)
}

func (s *ConfirmOperationSuite) TestSuccessNotConfirmedNotifies() {
	op := openedEmailOp(s.T())
	s.expectFetch(op, nil)
	s.expectPrepare(op, nil, nil)
	s.storage.EXPECT().Replace(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	s.notifierAPI.EXPECT().Send(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	_, err := s.execute("code123")
	s.Require().NoError(err)
	// действие подтверждено, но операция ещё не завершена: только CONFIRM_SUCCESS
	s.Require().Len(s.logEntries, 1)
	s.Equal(logstatus.ConfirmSuccess, s.logEntries[0].LogStatus)
}

func (s *ConfirmOperationSuite) TestSuccessConfirmedRunsCommit() {
	committed := false

	// в хранилище операция ещё Opened, Prepare подтверждает её
	// и возвращает commit второго фактора
	s.expectFetch(openedEmailOp(s.T()), nil)
	s.expectPrepare(confirmedOp(s.T()), func(context.Context) error {
		committed = true

		return nil
	}, nil)
	s.storage.EXPECT().Replace(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	// подтверждённая операция не отправляет код
	s.notifierAPI.EXPECT().Send(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	_, err := s.execute("code123")
	s.Require().NoError(err)
	s.True(committed)
	// финальное подтверждение фиксируется одной записью CONFIRMED (без отдельной CONFIRM_SUCCESS)
	s.Require().Len(s.logEntries, 1)
	s.Equal(logstatus.Confirmed, s.logEntries[0].LogStatus)
	s.Equal(logreason.Unspecified, s.logEntries[0].Reason)
}

// TestAlreadyConfirmedIsIdempotent - повторное подтверждение уже подтверждённой
// операции замыкается накоротко: Prepare/Replace не вызываются, операция возвращается как успех
// (нужно, чтобы поток открытия сессии можно было безопасно повторить после сбоя сессии).
func (s *ConfirmOperationSuite) TestAlreadyConfirmedIsIdempotent() {
	s.expectFetch(confirmedOp(s.T()), nil)
	// короткое замыкание: ни подготовка, ни запись, ни уведомление не выполняются
	s.preparer.EXPECT().Prepare(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	s.storage.EXPECT().Replace(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	s.notifierAPI.EXPECT().Send(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	out, err := s.execute("code123")
	s.Require().NoError(err)
	s.True(out.Is(operationstatus.Confirmed))
	// повтор ничего не меняет, поэтому и в журнал не пишется: событие CONFIRMED уже
	// зафиксировано при первом подтверждении
	s.Empty(s.logEntries)
}

// TestEmptySecretOnConfirmedOperation - подтверждённая операция без секрета: подтверждать нечего,
// поэтому пустой код не является ошибкой. Так приходит вход, у которого последнее звено пройдено
// отдельным методом подтверждения, а также повтор входа после отказа по лимиту сессий или сбоя.
func (s *ConfirmOperationSuite) TestEmptySecretOnConfirmedOperation() {
	s.expectFetch(confirmedOp(s.T()), nil)
	// короткое замыкание: ни подготовка, ни учёт неудачной попытки не выполняются
	s.preparer.EXPECT().Prepare(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	s.storage.EXPECT().Replace(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	s.storage.EXPECT().UpdateFailedAttempt(gomock.Any(), gomock.Any()).Times(0)
	s.notifierAPI.EXPECT().Send(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	out, err := s.execute("")
	s.Require().NoError(err)
	s.True(out.Is(operationstatus.Confirmed))
	s.Empty(s.logEntries)
}

// TestEmptySecretOnOpenedOperation - секрет не передан, а звено ещё открыто: подтверждать нечем.
// Отдаётся отдельной ошибкой, а не как неверный код: клиент должен отличать «ввод не сделан» от
// «введено неверно». Попытка не расходуется и возвращаются реальные счётчики операции - иначе
// клиент увидел бы исправную операцию с нулевыми остатками.
func (s *ConfirmOperationSuite) TestEmptySecretOnOpenedOperation() {
	op := openedEmailOp(s.T())
	s.expectFetch(op, nil)
	s.preparer.EXPECT().Prepare(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	s.storage.EXPECT().UpdateFailedAttempt(gomock.Any(), gomock.Any()).Times(0)

	out, err := s.execute("")
	s.Require().ErrorIs(err, secureoperation.ErrConfirmCodeIsRequired)
	s.Require().NotErrorIs(err, secureoperation.ErrConfirmCodeIsIncorrect)
	s.Positive(out.RemainingAttempts)
	s.Equal(op.RemainingAttempts, out.RemainingAttempts)
	// пропуск поля клиентом не является попыткой подтверждения, поэтому в журнал не пишется
	s.Empty(s.logEntries)
}

func (s *ConfirmOperationSuite) TestSuccessAuth2FARaceRejectedAsWrongCode() {
	op := openedEmailOp(s.T()) // в хранилище операция ещё Opened

	s.expectFetch(op, nil)
	// второй фактор уже израсходован конкурентным подтверждением
	s.expectPrepare(confirmedOp(s.T()), func(context.Context) error {
		return mrauth.ErrEventAuth2FACodeAlreadyUsed
	}, nil)
	s.storage.EXPECT().Replace(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.notifierAPI.EXPECT().Send(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	gotOp, err := s.execute("code123")
	s.Require().ErrorIs(err, secureoperation.ErrConfirmCodeIsIncorrect) // гонка отдаётся как неверный код
	s.Require().NotErrorIs(err, mrauth.ErrEventAuth2FACodeAlreadyUsed)  // внутренний сигнал наружу не уходит
	s.Equal(secureoperation.SecureOperation{}, gotOp)                   // транзакция откатилась
	// TOTP-replay фиксируется в журнале даже при откате транзакции
	s.Require().Len(s.logEntries, 1)
	s.Equal(logstatus.ConfirmFailed, s.logEntries[0].LogStatus)
	s.Equal(logreason.TOTPReplay, s.logEntries[0].Reason)
	s.Equal(op.UserID, s.logEntries[0].VisitorID)
}

// TestCommitFailureIsNotReplay - commit второго фактора упал по причине, не связанной
// с его расходованием (например, сбой alerter'а внутри транзакции). Такой сбой не должен
// маскироваться под повтор кода: наружу идёт внутренняя ошибка, а не «неверный код»,
// и в журнал безопасности не пишется TOTP_REPLAY.
func (s *ConfirmOperationSuite) TestCommitFailureIsNotReplay() {
	wantErr := errors.New("alerter failed")

	s.expectFetch(openedEmailOp(s.T()), nil)
	s.expectPrepare(confirmedOp(s.T()), func(context.Context) error {
		return wantErr
	}, nil)
	s.storage.EXPECT().Replace(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.notifierAPI.EXPECT().Send(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	gotOp, err := s.execute("code123")
	s.Require().ErrorIs(err, wantErr)
	s.Require().NotErrorIs(err, secureoperation.ErrConfirmCodeIsIncorrect)
	s.Equal(secureoperation.SecureOperation{}, gotOp) // транзакция откатилась
	s.Empty(s.logEntries)
}

type ResendCodeSuite struct {
	suite.Suite

	ctrl         *gomock.Controller
	ctx          context.Context
	txManager    *mock.MockDBTxManager
	storage      *mock.MockoperationResender
	notifierAPI  *mock.MockNoteProducer
	preparer     *mock.MockresendOperationPreparer
	logOperation *mock.MockoperationLogger
	logEntries   []entity.SecureOperationLog
	uc           *operation.ResendCode
}

func TestResendCodeSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ResendCodeSuite))
}

func (s *ResendCodeSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.txManager = mock.NewMockDBTxManager(s.ctrl)
	s.storage = mock.NewMockoperationResender(s.ctrl)
	s.notifierAPI = mock.NewMockNoteProducer(s.ctrl)
	s.preparer = mock.NewMockresendOperationPreparer(s.ctrl)
	s.logOperation = mock.NewMockoperationLogger(s.ctrl)
	s.logEntries = nil

	expectPassThroughTx(s.txManager)
	s.logOperation.EXPECT().
		Log(gomock.Any(), gomock.Any()).
		Do(func(_ context.Context, entry entity.SecureOperationLog) {
			s.logEntries = append(s.logEntries, entry)
		}).
		AnyTimes()

	s.uc = operation.NewResendCode(s.txManager, s.storage, s.notifierAPI, s.preparer, s.logOperation)
}

// пустым токеном не может быть найдена ни одна операция: снаружи это неотличимо
// от неизвестного и истёкшего токена и отдаётся тем же сентинелом.
func (s *ResendCodeSuite) TestEmptyToken() {
	_, err := s.uc.Execute(s.ctx, dto.ActorMeta{}, "en", "")
	s.Require().ErrorIs(err, secureoperation.ErrOperationInvalid)
}

// язык приходит уже определённым по запросу, поэтому пустой - ошибка проводки.
func (s *ResendCodeSuite) TestEmptyLangCode() {
	s.storage.EXPECT().FetchOneForUpdate(gomock.Any(), gomock.Any()).Times(0)

	_, err := s.uc.Execute(s.ctx, dto.ActorMeta{}, "", "token")
	s.Require().ErrorIs(err, sysmesserrors.ErrInternalIncorrectInputData)
}

func (s *ResendCodeSuite) TestRestricted() {
	op := openedEmailOp(s.T())
	s.storage.EXPECT().FetchOneForUpdate(gomock.Any(), gomock.Any()).Return(op, nil)
	s.preparer.EXPECT().
		Prepare(gomock.Any()).
		Return(op, secureoperation.ErrSendingNewMessagesIsTemporarilyRestricted)

	_, err := s.uc.Execute(s.ctx, dto.ActorMeta{}, "en", "token")
	s.Require().ErrorIs(err, secureoperation.ErrSendingNewMessagesIsTemporarilyRestricted)
	s.Require().Len(s.logEntries, 1)
	s.Equal(logstatus.Blocked, s.logEntries[0].LogStatus)
	s.Equal(logreason.Throttled, s.logEntries[0].Reason)
}

// TestNoAttemptsToResend - повторные отправки израсходованы окончательно. Это бизнес-результат,
// а не сбой: транзакция обязана закоммититься, а операция - вернуться вместе с ошибкой, иначе
// клиент не получит актуальные счётчики в operation_state.
func (s *ResendCodeSuite) TestNoAttemptsToResend() {
	op := openedEmailOp(s.T())
	s.storage.EXPECT().FetchOneForUpdate(gomock.Any(), gomock.Any()).Return(op, nil)
	s.preparer.EXPECT().
		Prepare(gomock.Any()).
		Return(op, secureoperation.ErrNoAttemptsToResendCode)

	got, err := s.uc.Execute(s.ctx, dto.ActorMeta{}, "en", "token")
	s.Require().ErrorIs(err, secureoperation.ErrNoAttemptsToResendCode)
	s.Equal(op.Token, got.Token, "операция должна вернуться вместе с ошибкой")
	s.Require().Len(s.logEntries, 1)
	s.Equal(logstatus.Blocked, s.logEntries[0].LogStatus)
	s.Equal(
		logreason.ResendsExhausted,
		s.logEntries[0].Reason,
		"окончательное исчерпание отправок в журнале отличается от временного троттла",
	)
}

// TestNotSupported - повторная отправка по 2FA-действию неприменима. Отказ адресован клиенту,
// поэтому сентинел обязан выйти наружу неизменным (иначе вместо 400 с кодом уедет 500),
// но событием журнала не является: это неверный вызов, а не атака или лимит.
func (s *ResendCodeSuite) TestNotSupported() {
	op := openedEmailOp(s.T())
	s.storage.EXPECT().FetchOneForUpdate(gomock.Any(), gomock.Any()).Return(op, nil)
	s.preparer.EXPECT().
		Prepare(gomock.Any()).
		Return(secureoperation.SecureOperation{}, secureoperation.ErrResendCodeIsNotSupported)
	s.storage.EXPECT().Replace(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	s.notifierAPI.EXPECT().Send(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	_, err := s.uc.Execute(s.ctx, dto.ActorMeta{}, "en", "token")
	s.Require().ErrorIs(err, secureoperation.ErrResendCodeIsNotSupported)
	s.Require().NotErrorIs(err, sysmesserrors.ErrInternalServiceOperationFailed)
	s.Empty(s.logEntries)
}

func (s *ResendCodeSuite) TestSuccess() {
	op := openedEmailOp(s.T())
	s.storage.EXPECT().FetchOneForUpdate(gomock.Any(), gomock.Any()).Return(op, nil)
	s.preparer.EXPECT().Prepare(gomock.Any()).Return(op, nil)
	s.storage.EXPECT().Replace(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	s.notifierAPI.EXPECT().Send(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	_, err := s.uc.Execute(s.ctx, dto.ActorMeta{}, "en", "token")
	s.Require().NoError(err)
	s.Require().Len(s.logEntries, 1)
	s.Equal(logstatus.ResentCode, s.logEntries[0].LogStatus)
	// поток анонимный, но владелец операции известен после её чтения
	s.Equal(op.UserID, s.logEntries[0].VisitorID)
}

type RevokeOperationSuite struct {
	suite.Suite

	ctrl         *gomock.Controller
	ctx          context.Context
	storage      *mock.MockoperationRevoker
	logStorage   *mock.MockoperationLogStorage
	logOperation *mock.MockoperationLogger
	logEntries   []entity.SecureOperationLog
	uc           *operation.RevokeOperation
}

func TestRevokeOperationSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(RevokeOperationSuite))
}

func (s *RevokeOperationSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.storage = mock.NewMockoperationRevoker(s.ctrl)
	s.logStorage = mock.NewMockoperationLogStorage(s.ctrl)
	s.logOperation = mock.NewMockoperationLogger(s.ctrl)
	s.logEntries = nil

	s.logOperation.EXPECT().
		Log(gomock.Any(), gomock.Any()).
		Do(func(_ context.Context, entry entity.SecureOperationLog) {
			s.logEntries = append(s.logEntries, entry)
		}).
		AnyTimes()

	s.uc = operation.NewRevokeOperation(s.storage, s.logOperation)
}

// пустым токеном не может быть найдена ни одна операция: снаружи это неотличимо
// от неизвестного и истёкшего токена и отдаётся тем же сентинелом.
func (s *RevokeOperationSuite) TestEmptyToken() {
	s.Require().ErrorIs(s.uc.Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()}, ""), secureoperation.ErrOperationInvalid)
}

// поток отзыва доступен только залогиненным, поэтому анонимный вызывающий - ошибка проводки.
func (s *RevokeOperationSuite) TestEmptyUserID() {
	s.storage.EXPECT().FetchOne(gomock.Any(), gomock.Any()).Times(0)

	err := s.uc.Execute(s.ctx, dto.ActorMeta{}, "token")
	s.Require().ErrorIs(err, sysmesserrors.ErrInternalIncorrectInputData)
}

func (s *RevokeOperationSuite) TestSuccess() {
	op := openedEmailOp(s.T())
	s.storage.EXPECT().FetchOne(gomock.Any(), "token").Return(op, nil)
	s.storage.EXPECT().Delete(gomock.Any(), "token").Return(nil)

	s.Require().NoError(s.uc.Execute(s.ctx, dto.ActorMeta{VisitorID: op.UserID}, "token"))
	// операция читается перед удалением, поэтому в журнал попадает, что именно отозвано
	s.Require().Len(s.logEntries, 1)
	s.Equal(logstatus.Revoked, s.logEntries[0].LogStatus)
	s.Equal(op.Name, s.logEntries[0].OperationName)
	s.Equal(confirmmethod.Email, s.logEntries[0].ConfirmMethod)
	s.Equal(op.UserID, s.logEntries[0].VisitorID)
}

func (s *RevokeOperationSuite) TestFetchError() {
	wantErr := errors.New("fetch failed")
	s.storage.EXPECT().FetchOne(gomock.Any(), gomock.Any()).Return(secureoperation.SecureOperation{}, wantErr)
	s.storage.EXPECT().Delete(gomock.Any(), gomock.Any()).Times(0)

	s.Require().ErrorIs(s.uc.Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()}, "token"), wantErr)
	s.Empty(s.logEntries)
}

func (s *RevokeOperationSuite) TestDeleteError() {
	wantErr := errors.New("delete failed")

	op := openedEmailOp(s.T())
	s.storage.EXPECT().FetchOne(gomock.Any(), gomock.Any()).Return(op, nil)
	s.storage.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(wantErr)

	s.Require().ErrorIs(s.uc.Execute(s.ctx, dto.ActorMeta{VisitorID: op.UserID}, "token"), wantErr)
	s.Empty(s.logEntries)
}

// отозвать можно только собственную операцию: владение её токеном доступа не даёт.
func (s *RevokeOperationSuite) TestForeignOperation() {
	op := openedEmailOp(s.T())
	s.storage.EXPECT().FetchOne(gomock.Any(), gomock.Any()).Return(op, nil)
	s.storage.EXPECT().Delete(gomock.Any(), gomock.Any()).Times(0)

	err := s.uc.Execute(s.ctx, dto.ActorMeta{VisitorID: uuid.New()}, "token")
	s.Require().ErrorIs(err, sysmesserrors.ErrAccessForbidden)
	// обращение к чужой операции фиксируется в журнале за реальным вызывающим, а не за владельцем
	s.Require().Len(s.logEntries, 1)
	s.Equal(logstatus.Blocked, s.logEntries[0].LogStatus)
	s.Equal(logreason.AccessForbidden, s.logEntries[0].Reason)
	s.NotEqual(op.UserID, s.logEntries[0].VisitorID)
}

func (s *RevokeOperationSuite) TestStatisticSuccess() {
	s.logStorage.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(nil)

	s.Require().NoError(operation.NewStatistic(s.logStorage).Execute(s.ctx, []entity.SecureOperationLog{}))
}

func (s *RevokeOperationSuite) TestStatisticInsertError() {
	wantErr := errors.New("insert failed")
	s.logStorage.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(wantErr)

	s.Require().ErrorIs(operation.NewStatistic(s.logStorage).Execute(s.ctx, nil), wantErr)
}
