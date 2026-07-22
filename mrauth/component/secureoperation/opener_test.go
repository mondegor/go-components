package secureoperation_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	sysmesserrors "github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-core/util/conv"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/component/secureoperation"
	"github.com/mondegor/go-components/mrauth/component/secureoperation/mock"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/logstatus"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	secureoperation_model "github.com/mondegor/go-components/mrauth/model/secureoperation"
)

//go:generate mockgen -source=opener.go -destination=mock/opener.go -package=mock
//go:generate mockgen -destination=mock/mrstorage.go -package=mock github.com/mondegor/go-core/mrstorage DBTxManager
//go:generate mockgen -destination=mock/mrnotifier.go -package=mock github.com/mondegor/go-components/mrnotifier NoteProducer

type OpenerSuite struct {
	suite.Suite

	ctrl         *gomock.Controller
	ctx          context.Context
	txManager    *mock.MockDBTxManager
	storage      *mock.MockoperationOpenerStorage
	notifierAPI  *mock.MockNoteProducer
	logOperation *mock.MockoperationLogger
	logEntries   []entity.SecureOperationLog
	svc          *secureoperation.Opener
}

func TestOpenerSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(OpenerSuite))
}

func (s *OpenerSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.txManager = mock.NewMockDBTxManager(s.ctrl)
	s.storage = mock.NewMockoperationOpenerStorage(s.ctrl)
	s.notifierAPI = mock.NewMockNoteProducer(s.ctrl)
	s.logOperation = mock.NewMockoperationLogger(s.ctrl)
	s.logEntries = nil

	// транзакция выполняет переданное задание как есть
	s.txManager.EXPECT().
		Do(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, job func(ctx context.Context) error, _ ...mrstorage.TxOption) error {
			return job(ctx)
		}).
		AnyTimes()

	// записи журнала собираются, чтобы проверить их состав и порядок
	s.logOperation.EXPECT().
		Log(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, entry entity.SecureOperationLog) {
			s.logEntries = append(s.logEntries, entry)
		}).
		AnyTimes()

	s.svc = secureoperation.NewOpener(s.txManager, s.storage, s.notifierAPI, s.logOperation)
}

// emailOp - создаёт sendable-операцию (Email) в статусе Opened для указанного владельца.
func (s *OpenerSuite) emailOp(userID uuid.UUID) secureoperation_model.SecureOperation {
	op := secureoperation_model.SecureOperation{
		Token:             "token",
		Name:              "confirm.change.email",
		UserID:            userID,
		RemainingAttempts: 3,
		RemainingResends:  5,
		ResendsAt:         time.Now().Add(-time.Minute),
		Status:            operationstatus.Opened,
		ExpiresAt:         time.Now().Add(10 * time.Minute),
	}
	s.Require().NoError(secureoperation_model.WakeUp(&op, []secureoperation_model.ConfirmAction{
		{
			Method:        confirmmethod.Email,
			MaxAttempts:   3,
			MaxResends:    5,
			MinResendTime: 5 * time.Minute,
			Expiry:        10 * time.Minute,
			Address:       "u@e",
			ConfirmCode:   "hash",
			// код в открытом виде живёт только в рамках запроса и уходит в уведомление
			PlainConfirmCode: "123456",
		},
	}))

	return op
}

// вытесненная операция фиксируется в журнале как отозванная, затем пишется открытие новой.
func (s *OpenerSuite) TestOpenSupersedesPrevious() {
	userID := uuid.New()
	op := s.emailOp(userID)

	s.storage.EXPECT().DeleteByUserIDAndName(gomock.Any(), userID, op.Name).Return(nil)
	s.storage.EXPECT().Insert(gomock.Any(), op).Return(nil)
	s.notifierAPI.EXPECT().
		Send(gomock.Any(), "confirm.change.email", gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, props map[string]any) error {
			s.Equal("u@e", props["to"])
			s.Equal("123456", props["confirmCode"])
			s.Equal("ru", props["lang"]) // доп. поля вызывающего доходят до уведомления

			return nil
		})

	err := s.svc.Open(s.ctx, dto.ActorMeta{}, op, "confirm.change.email", conv.Group{"lang": "ru"})
	s.Require().NoError(err)

	s.Require().Len(s.logEntries, 2)
	s.Equal(logstatus.Revoked, s.logEntries[0].LogStatus)
	s.Equal(logreason.Superseded, s.logEntries[0].Reason)
	s.Equal(op.Name, s.logEntries[0].OperationName)
	// владелец операции фиксируется как посетитель, хотя поток пришёл анонимным
	s.Equal(userID, s.logEntries[0].VisitorID)
	s.Equal(logstatus.Opened, s.logEntries[1].LogStatus)
	s.Equal(logreason.Unspecified, s.logEntries[1].Reason)
	s.Equal(userID, s.logEntries[1].VisitorID)
}

// вытеснять нечего (первая операция такого типа): sentinel хранилища ошибкой не считается,
// в журнал попадает только открытие новой операции.
func (s *OpenerSuite) TestOpenWithoutPrevious() {
	userID := uuid.New()
	op := s.emailOp(userID)

	s.storage.EXPECT().DeleteByUserIDAndName(gomock.Any(), userID, op.Name).
		Return(sysmesserrors.ErrEventStorageRecordsNotAffected)
	s.storage.EXPECT().Insert(gomock.Any(), op).Return(nil)
	s.notifierAPI.EXPECT().Send(gomock.Any(), "confirm.change.email", gomock.Any()).Return(nil)

	s.Require().NoError(s.svc.Open(s.ctx, dto.ActorMeta{}, op, "confirm.change.email", nil))

	s.Require().Len(s.logEntries, 1)
	s.Equal(logstatus.Opened, s.logEntries[0].LogStatus)
}

// у операции регистрации нового email владельца нет: гасить нечего и не по чему,
// удаление не вызывается (мок без EXPECT: любой вызов провалит тест).
func (s *OpenerSuite) TestOpenAnonymousSkipsSupersede() {
	op := s.emailOp(uuid.Nil)

	s.storage.EXPECT().Insert(gomock.Any(), op).Return(nil)
	s.notifierAPI.EXPECT().Send(gomock.Any(), "confirm.user.activation", gomock.Any()).Return(nil)

	s.Require().NoError(s.svc.Open(s.ctx, dto.ActorMeta{}, op, "confirm.user.activation", nil))

	s.Require().Len(s.logEntries, 1)
	s.Equal(uuid.Nil, s.logEntries[0].VisitorID)
}

// ошибка сохранения операции возвращается наружу, журнал не пишется.
func (s *OpenerSuite) TestOpenInsertError() {
	userID := uuid.New()
	op := s.emailOp(userID)

	s.storage.EXPECT().DeleteByUserIDAndName(gomock.Any(), userID, op.Name).
		Return(sysmesserrors.ErrEventStorageRecordsNotAffected)
	s.storage.EXPECT().Insert(gomock.Any(), op).Return(errors.New("db is down"))

	s.Require().Error(s.svc.Open(s.ctx, dto.ActorMeta{}, op, "confirm.change.email", nil))
	s.Empty(s.logEntries)
}

// сбой отправки кода откатывает транзакцию: операция считается не открытой.
func (s *OpenerSuite) TestOpenNotifyError() {
	userID := uuid.New()
	op := s.emailOp(userID)

	s.storage.EXPECT().DeleteByUserIDAndName(gomock.Any(), userID, op.Name).Return(nil)
	s.storage.EXPECT().Insert(gomock.Any(), op).Return(nil)
	s.notifierAPI.EXPECT().Send(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("smtp is down"))

	s.Require().Error(s.svc.Open(s.ctx, dto.ActorMeta{}, op, "confirm.change.email", nil))
	s.Empty(s.logEntries)
}

// ошибка гашения прежних операций не даёт создать новую.
func (s *OpenerSuite) TestOpenSupersedeError() {
	userID := uuid.New()
	op := s.emailOp(userID)

	s.storage.EXPECT().DeleteByUserIDAndName(gomock.Any(), userID, op.Name).Return(errors.New("db is down"))

	s.Require().Error(s.svc.Open(s.ctx, dto.ActorMeta{}, op, "confirm.change.email", nil))
	s.Empty(s.logEntries)
}
