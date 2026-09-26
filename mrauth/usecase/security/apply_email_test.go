package security_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/util/conv"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/logstatus"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
	"github.com/mondegor/go-components/mrauth/usecase/security"
	"github.com/mondegor/go-components/mrauth/usecase/security/mock"
)

//go:generate mockgen -source=apply_email.go -destination=mock/apply_email.go -package=mock

const changeEmailPayload = `{"new_email":"new@example.com","email":"user@example.com"}`

// confirmedChangeEmailOp - подтверждённая операция первого шага смены емаила.
func confirmedChangeEmailOp(userID uuid.UUID) secureoperation.SecureOperation {
	return secureoperation.SecureOperation{
		Token:   "op-token",
		Name:    unit.NameConfirmChangeEmailRequest,
		UserID:  userID,
		Payload: []byte(changeEmailPayload),
		Status:  operationstatus.Confirmed,
	}
}

type (
	// sentNote - уведомление, переданное в notifier.
	sentNote struct {
		key   string
		props map[string]any
	}

	ApplyEmailSuite struct {
		baseSuite

		storage   *mock.MockoperationDeleter
		checker   *mock.MockuserEmailChecker
		factory   *mock.MockchangeEmailCreator
		opener    *mock.MockoperationOpener
		notes     *mock.MockNotifier
		userID    uuid.UUID
		confirmOp secureoperation.SecureOperation
		deleted   string
		openedOp  secureoperation.SecureOperation
		openedKey string
		sent      []sentNote
	}
)

func TestApplyEmailSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ApplyEmailSuite))
}

func (s *ApplyEmailSuite) SetupTest() {
	s.baseSuite.SetupTest()

	s.storage = mock.NewMockoperationDeleter(s.ctrl)
	s.checker = mock.NewMockuserEmailChecker(s.ctrl)
	s.factory = mock.NewMockchangeEmailCreator(s.ctrl)
	s.opener = mock.NewMockoperationOpener(s.ctrl)
	s.notes = mock.NewMockNotifier(s.ctrl)
	s.userID = uuid.New()
	s.confirmOp = secureoperation.SecureOperation{
		Token:     "new-op-token",
		Name:      unit.NameConfirmChangeEmail,
		UserID:    s.userID,
		Status:    operationstatus.Opened,
		ExpiresAt: time.Now().Add(72 * time.Hour).UTC().Round(time.Second),
	}
	s.deleted = ""
	s.openedOp = secureoperation.SecureOperation{}
	s.openedKey = ""
	s.sent = nil

	s.storage.EXPECT().
		Delete(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, token string) error {
			s.deleted = token

			return nil
		}).
		AnyTimes()

	s.opener.EXPECT().
		Open(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ dto.ActorMeta, op secureoperation.SecureOperation, noteName string, _ conv.Group) error {
			s.openedOp = op
			s.openedKey = noteName

			return nil
		}).
		AnyTimes()

	s.notes.EXPECT().
		Send(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, key string, props map[string]any) error {
			s.sent = append(s.sent, sentNote{key: key, props: props})

			return nil
		}).
		AnyTimes()
}

func (s *ApplyEmailSuite) newUseCase() *security.ApplyEmail {
	return security.NewApplyEmail(s.txManager, s.storage, s.checker, s.factory, s.opener, s.notes, s.logOperation)
}

func (s *ApplyEmailSuite) actor() dto.ActorMeta {
	return dto.ActorMeta{VisitorID: s.userID}
}

// TestSuccess - первый шаг применяется так: его операция удаляется, открывается операция
// второго шага (код на новый адрес), а на прежний адрес уходит уведомление о запросе смены
// с новым адресом и сроком действия новой операции. Сам емаил не меняется - это делает
// apply-operation по второй операции.
func (s *ApplyEmailSuite) TestSuccess() {
	s.storage.EXPECT().FetchOneForUpdate(gomock.Any(), "op-token").Return(confirmedChangeEmailOp(s.userID), nil)
	s.checker.EXPECT().
		CheckAvailabilityEmail(gomock.Any(), contactaddress.NewEmail("new@example.com")).
		Return(nil)
	s.factory.EXPECT().
		Create(s.userID, dto.ChangeEmailOperation{NewEmail: "new@example.com", Email: "user@example.com"}).
		Return(s.confirmOp, nil)

	userLocation := time.FixedZone("MSK", 3*60*60)

	op, err := s.newUseCase().Execute(s.ctx, s.actor(), userLocation, "op-token")
	s.Require().NoError(err)
	s.Equal(s.confirmOp, op)

	s.Equal("op-token", s.deleted)
	s.Equal(s.confirmOp, s.openedOp)
	s.Equal("confirm.change.email", s.openedKey)

	s.Require().Len(s.sent, 1)
	s.Equal("user.email.change.requested", s.sent[0].key)
	s.Equal("user@example.com", s.sent[0].props["to"])
	s.Equal("new@example.com", s.sent[0].props["newEmail"])
	// срок - текстом в поясе пользователя и с его названием: в письме нет места для RFC3339
	s.Equal(s.confirmOp.ExpiresAt.In(userLocation).Format("2006-01-02 15:04")+" (MSK)", s.sent[0].props["expiresAt"])

	s.Require().Len(s.logEntries, 1)
	s.Equal(unit.NameConfirmChangeEmailRequest, s.logEntries[0].OperationName)
	s.Equal(logstatus.Applied, s.logEntries[0].LogStatus)
}

// TestNoticeTimeWithoutLocation - пояс пользователя не определён: срок выводится в UTC,
// и это тоже названо в тексте.
func (s *ApplyEmailSuite) TestNoticeTimeWithoutLocation() {
	s.storage.EXPECT().FetchOneForUpdate(gomock.Any(), "op-token").Return(confirmedChangeEmailOp(s.userID), nil)
	s.checker.EXPECT().CheckAvailabilityEmail(gomock.Any(), gomock.Any()).Return(nil)
	s.factory.EXPECT().Create(gomock.Any(), gomock.Any()).Return(s.confirmOp, nil)

	_, err := s.newUseCase().Execute(s.ctx, s.actor(), nil, "op-token")
	s.Require().NoError(err)
	s.Require().Len(s.sent, 1)
	s.Equal(s.confirmOp.ExpiresAt.UTC().Format("2006-01-02 15:04")+" (UTC)", s.sent[0].props["expiresAt"])
}

// TestRejected - чужая, неподходящая или не подтверждённая операция не применяется:
// ничего не удаляется и не открывается, а в журнал пишется блокировка.
func (s *ApplyEmailSuite) TestRejected() {
	type testCase struct {
		name       string
		mutate     func(op *secureoperation.SecureOperation)
		wantErr    error
		wantReason logreason.Enum
	}

	tests := []testCase{
		{
			name:       "operation of another user",
			mutate:     func(op *secureoperation.SecureOperation) { op.UserID = uuid.New() },
			wantErr:    errors.ErrAccessForbidden,
			wantReason: logreason.AccessForbidden,
		},
		{
			// токен второго шага применяется через apply-operation, а не здесь
			name:       "operation of the second step",
			mutate:     func(op *secureoperation.SecureOperation) { op.Name = unit.NameConfirmChangeEmail },
			wantErr:    errors.ErrAccessForbidden,
			wantReason: logreason.AccessForbidden,
		},
		{
			name:       "operation of another type",
			mutate:     func(op *secureoperation.SecureOperation) { op.Name = unit.NameConfirmChangePhone },
			wantErr:    errors.ErrAccessForbidden,
			wantReason: logreason.AccessForbidden,
		},
		{
			name:       "operation is not confirmed",
			mutate:     func(op *secureoperation.SecureOperation) { op.Status = operationstatus.Opened },
			wantErr:    mrauth.ErrOperationIsNotConfirmed,
			wantReason: logreason.NotConfirmed,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			op := confirmedChangeEmailOp(s.userID)
			tt.mutate(&op)

			s.storage.EXPECT().FetchOneForUpdate(gomock.Any(), "op-token").Return(op, nil)

			_, err := s.newUseCase().Execute(s.ctx, s.actor(), nil, "op-token")
			s.Require().ErrorIs(err, tt.wantErr)

			s.Empty(s.deleted)
			s.Empty(s.openedKey)
			s.Empty(s.sent)
			s.Require().Len(s.logEntries, 1)
			s.Equal(logstatus.Blocked, s.logEntries[0].LogStatus)
			s.Equal(tt.wantReason, s.logEntries[0].Reason)
		})
	}
}

// TestEmailTaken - новый адрес заняли, пока шло подтверждение первого шага: второй шаг
// не открывается, первая операция остаётся (транзакция откатывается), клиент получает
// EmailAlreadyExists и начинает смену заново с другим адресом.
func (s *ApplyEmailSuite) TestEmailTaken() {
	s.storage.EXPECT().FetchOneForUpdate(gomock.Any(), "op-token").Return(confirmedChangeEmailOp(s.userID), nil)
	s.checker.EXPECT().CheckAvailabilityEmail(gomock.Any(), gomock.Any()).Return(mrauth.ErrEmailAlreadyExists)

	_, err := s.newUseCase().Execute(s.ctx, s.actor(), nil, "op-token")
	s.Require().ErrorIs(err, mrauth.ErrEmailAlreadyExists)

	s.Empty(s.deleted)
	s.Empty(s.openedKey)
	s.Empty(s.sent)
}

// TestUnknownToken - неизвестный токен - доменная ошибка недействительной операции, а не 404.
func (s *ApplyEmailSuite) TestUnknownToken() {
	s.storage.EXPECT().
		FetchOneForUpdate(gomock.Any(), "op-token").
		Return(secureoperation.SecureOperation{}, errors.ErrEventStorageNoRecordFound)

	_, err := s.newUseCase().Execute(s.ctx, s.actor(), nil, "op-token")
	s.Require().ErrorIs(err, mrauth.ErrOperationInvalid)
	s.Empty(s.logEntries)
}

// TestInvalidInput - пустой токен - недействительная операция (запрос вниз не идёт),
// пустой пользователь - нарушение инварианта метода, доступного только авторизованному.
func (s *ApplyEmailSuite) TestInvalidInput() {
	_, err := s.newUseCase().Execute(s.ctx, s.actor(), nil, "")
	s.Require().ErrorIs(err, mrauth.ErrOperationInvalid)

	_, err = s.newUseCase().Execute(s.ctx, dto.ActorMeta{}, nil, "op-token")
	s.Require().ErrorIs(err, errors.ErrInternalIncorrectInputData)
}
