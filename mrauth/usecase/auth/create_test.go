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
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/logstatus"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
	"github.com/mondegor/go-components/mrauth/usecase/auth"
)

type (
	fakeTx struct{}

	fakeNotifier struct {
		sent bool
		err  error
	}

	fakeOpCreator struct {
		inserted bool
		err      error
	}

	fakeUserChecker struct {
		err error
	}

	fakeUser2FAFactory struct {
		user dto.User2FA
		err  error
	}

	fakeSessionOpFactory struct {
		op  secureoperation.SecureOperation
		err error
	}

	fakeUserOpFactory struct {
		op              secureoperation.SecureOperation
		err             error
		gotUser2FA      dto.User2FA
		gotRegisteredIP mrtype.DetailedIP
	}

	fakeLocker struct {
		err error
	}

	fakeOperationLogger struct {
		entries []entity.SecureOperationLog
	}
)

func (f *fakeOperationLogger) Log(_ context.Context, entry entity.SecureOperationLog) {
	f.entries = append(f.entries, entry)
}

func (fakeTx) Do(ctx context.Context, job func(ctx context.Context) error, _ ...mrstorage.TxOption) error {
	return job(ctx)
}

func (f *fakeNotifier) Send(context.Context, string, map[string]any) error {
	if f.err != nil {
		return f.err
	}

	f.sent = true

	return nil
}

func (f *fakeOpCreator) Insert(context.Context, secureoperation.SecureOperation) error {
	if f.err != nil {
		return f.err
	}

	f.inserted = true

	return nil
}

func (f fakeUserChecker) CheckAvailabilityRealm(context.Context, string, contactaddress.ContactAddress) error {
	return f.err
}

func (f fakeUser2FAFactory) CreateByUserID(context.Context, uuid.UUID) (dto.User2FA, error) {
	return f.user, f.err
}

func (f fakeUser2FAFactory) CreateByUserLogin(context.Context, contactaddress.ContactAddress) (dto.User2FA, error) {
	return f.user, f.err
}

func (f fakeSessionOpFactory) Name() string {
	return unit.NameAuthorizeUser
}

func (f fakeSessionOpFactory) Create(dto.User2FA, string, string, contactaddress.ContactAddress) (secureoperation.SecureOperation, error) {
	return f.op, f.err
}

func (f *fakeUserOpFactory) Name() string {
	return unit.NameConfirmCreateUser
}

func (f *fakeUserOpFactory) Create(
	user2FA dto.User2FA,
	_ string,
	_ contactaddress.ContactAddress,
	registeredIP mrtype.DetailedIP,
) (secureoperation.SecureOperation, error) {
	f.gotUser2FA = user2FA
	f.gotRegisteredIP = registeredIP

	return f.op, f.err
}

func (f *fakeLocker) Lock(context.Context, string) (func(), error) {
	return func() {}, f.err
}

func (f *fakeLocker) LockWithExpiry(context.Context, string, time.Duration) (func(), error) {
	if f.err != nil {
		return nil, f.err
	}

	return func() {}, nil
}

func openedEmailOp(t *testing.T) secureoperation.SecureOperation {
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

func newCreateSession(
	checker fakeUserChecker,
	creator *fakeOpCreator,
	notifier *fakeNotifier,
	opFactory fakeSessionOpFactory,
	logOperation *fakeOperationLogger,
) *auth.CreateSession {
	return auth.NewCreateSession(
		fakeTx{},
		checker,
		creator,
		notifier,
		fakeUser2FAFactory{},
		logOperation,
		[]auth.CreateSessionRealm{{Name: "shop", Operation: opFactory}},
	)
}

func TestCreateSession_EmptyLogin(t *testing.T) {
	t.Parallel()

	uc := newCreateSession(fakeUserChecker{}, &fakeOpCreator{}, &fakeNotifier{}, fakeSessionOpFactory{}, &fakeOperationLogger{})
	_, err := uc.Execute(context.Background(), dto.ActorMeta{}, "shop", "en", "")
	require.Error(t, err)
}

func TestCreateSession_UnknownRealm(t *testing.T) {
	t.Parallel()

	uc := newCreateSession(fakeUserChecker{}, &fakeOpCreator{}, &fakeNotifier{}, fakeSessionOpFactory{}, &fakeOperationLogger{})
	_, err := uc.Execute(context.Background(), dto.ActorMeta{}, "unknown", "en", "user@example.com")
	require.Error(t, err)
}

func TestCreateSession_LoginDoesNotExist(t *testing.T) {
	t.Parallel()

	// nil от CheckAvailabilityRealm означает, что логин свободен - значит входить некому.
	logOperation := &fakeOperationLogger{}
	uc := newCreateSession(fakeUserChecker{}, &fakeOpCreator{}, &fakeNotifier{}, fakeSessionOpFactory{op: openedEmailOp(t)}, logOperation)
	_, err := uc.Execute(context.Background(), dto.ActorMeta{}, "shop", "en", "user@example.com")
	require.ErrorIs(t, err, mrauth.ErrLoginNotExists)
	require.Len(t, logOperation.entries, 1)
	require.Equal(t, logstatus.Blocked, logOperation.entries[0].LogStatus)
	require.Equal(t, logreason.LoginNotExists, logOperation.entries[0].Reason)
	require.Equal(t, unit.NameAuthorizeUser, logOperation.entries[0].OperationName)
}

func TestCreateSession_Success(t *testing.T) {
	t.Parallel()

	creator := &fakeOpCreator{}
	notifier := &fakeNotifier{}
	logOperation := &fakeOperationLogger{}
	op := openedEmailOp(t)
	uc := newCreateSession(fakeUserChecker{err: mrauth.ErrEmailAlreadyExists}, creator, notifier, fakeSessionOpFactory{op: op}, logOperation)

	_, err := uc.Execute(context.Background(), dto.ActorMeta{}, "shop", "en", "user@example.com")
	require.NoError(t, err)
	require.True(t, creator.inserted)
	require.True(t, notifier.sent)
	require.Len(t, logOperation.entries, 1)
	require.Equal(t, logstatus.Opened, logOperation.entries[0].LogStatus)
	require.Equal(t, logreason.Unspecified, logOperation.entries[0].Reason)
	// вход инициирует существующий пользователь: он и попадает в журнал
	require.Equal(t, op.UserID, logOperation.entries[0].VisitorID)
}

func TestCreateSession_CheckerError(t *testing.T) {
	t.Parallel()

	uc := newCreateSession(
		fakeUserChecker{err: errors.New("boom")},
		&fakeOpCreator{},
		&fakeNotifier{},
		fakeSessionOpFactory{op: openedEmailOp(t)},
		&fakeOperationLogger{},
	)
	_, err := uc.Execute(context.Background(), dto.ActorMeta{}, "shop", "en", "user@example.com")
	require.Error(t, err)
}

func newCreateUser(
	checker fakeUserChecker,
	creator *fakeOpCreator,
	notifier *fakeNotifier,
	factory fakeUser2FAFactory,
	locker *fakeLocker,
	opFactory *fakeUserOpFactory,
	logOperation *fakeOperationLogger,
) *auth.CreateUser {
	return auth.NewCreateUser(
		fakeTx{},
		checker,
		creator,
		notifier,
		factory,
		locker,
		logOperation,
		[]auth.CreateUserRealm{{Name: "shop", Operation: opFactory}},
	)
}

func TestCreateUser_UnknownRealm(t *testing.T) {
	t.Parallel()

	uc := newCreateUser(
		fakeUserChecker{},
		&fakeOpCreator{},
		&fakeNotifier{},
		fakeUser2FAFactory{},
		&fakeLocker{},
		&fakeUserOpFactory{},
		&fakeOperationLogger{},
	)
	_, err := uc.Execute(context.Background(), "unknown", "en", "user@example.com", mrtype.NewIP(netip.MustParseAddr("203.0.113.7")))
	require.Error(t, err)
}

func TestCreateUser_InvalidEmail(t *testing.T) {
	t.Parallel()

	uc := newCreateUser(
		fakeUserChecker{},
		&fakeOpCreator{},
		&fakeNotifier{},
		fakeUser2FAFactory{},
		&fakeLocker{},
		&fakeUserOpFactory{},
		&fakeOperationLogger{},
	)
	_, err := uc.Execute(context.Background(), "shop", "en", "bad", mrtype.NewIP(netip.MustParseAddr("203.0.113.7")))
	require.Error(t, err)
}

func TestCreateUser_LockNotObtained(t *testing.T) {
	t.Parallel()

	logOperation := &fakeOperationLogger{}
	uc := newCreateUser(
		fakeUserChecker{},
		&fakeOpCreator{},
		&fakeNotifier{},
		fakeUser2FAFactory{},
		&fakeLocker{err: mrlock.ErrLockKeyNotObtained},
		&fakeUserOpFactory{},
		logOperation,
	)
	_, err := uc.Execute(context.Background(), "shop", "en", "user@example.com", mrtype.NewIP(netip.MustParseAddr("203.0.113.7")))
	require.ErrorIs(t, err, mrauth.ErrSignupAlreadyInProgressTryLater)
	require.Len(t, logOperation.entries, 1)
	require.Equal(t, logstatus.Blocked, logOperation.entries[0].LogStatus)
	require.Equal(t, logreason.Throttled, logOperation.entries[0].Reason)
	require.Equal(t, unit.NameConfirmCreateUser, logOperation.entries[0].OperationName)
}

func TestCreateUser_Success(t *testing.T) {
	t.Parallel()

	creator := &fakeOpCreator{}
	notifier := &fakeNotifier{}
	opFactory := &fakeUserOpFactory{op: openedEmailOp(t)}
	logOperation := &fakeOperationLogger{}
	uc := newCreateUser(fakeUserChecker{}, creator, notifier, fakeUser2FAFactory{}, &fakeLocker{}, opFactory, logOperation)

	_, err := uc.Execute(context.Background(), "shop", "en", "user@example.com", mrtype.NewIP(netip.MustParseAddr("203.0.113.7")))
	require.NoError(t, err)
	require.True(t, creator.inserted)
	require.True(t, notifier.sent)
	require.Equal(t, mrtype.NewIP(netip.MustParseAddr("203.0.113.7")), opFactory.gotRegisteredIP, "IP регистрации доезжает до фабрики операции")
	require.Len(t, logOperation.entries, 1)
	require.Equal(t, logstatus.Opened, logOperation.entries[0].LogStatus)
}

func TestCreateUser_CheckerError(t *testing.T) {
	t.Parallel()

	uc := newCreateUser(
		fakeUserChecker{err: errors.New("boom")},
		&fakeOpCreator{},
		&fakeNotifier{},
		fakeUser2FAFactory{},
		&fakeLocker{},
		&fakeUserOpFactory{op: openedEmailOp(t)},
		&fakeOperationLogger{},
	)
	_, err := uc.Execute(context.Background(), "shop", "en", "user@example.com", mrtype.NewIP(netip.MustParseAddr("203.0.113.7")))
	require.Error(t, err)
}

func TestCreateUser_2FAFactoryError(t *testing.T) {
	t.Parallel()

	factory := fakeUser2FAFactory{err: errors.New("2fa boom")}
	uc := newCreateUser(
		fakeUserChecker{},
		&fakeOpCreator{},
		&fakeNotifier{},
		factory,
		&fakeLocker{},
		&fakeUserOpFactory{op: openedEmailOp(t)},
		&fakeOperationLogger{},
	)
	_, err := uc.Execute(context.Background(), "shop", "en", "user@example.com", mrtype.NewIP(netip.MustParseAddr("203.0.113.7")))
	require.Error(t, err)
}

// TestCreateUser_NewEmailEmpty2FAForwarded - логин не принадлежит существующему пользователю:
// фабрика 2FA возвращает ErrEventStorageNoRecordFound, usecase продолжает выполнение с пустым User2FA.
func TestCreateUser_NewEmailEmpty2FAForwarded(t *testing.T) {
	t.Parallel()

	factory := fakeUser2FAFactory{err: sysmesserrors.ErrEventStorageNoRecordFound}
	opFactory := &fakeUserOpFactory{op: openedEmailOp(t)}
	uc := newCreateUser(fakeUserChecker{}, &fakeOpCreator{}, &fakeNotifier{}, factory, &fakeLocker{}, opFactory, &fakeOperationLogger{})

	_, err := uc.Execute(context.Background(), "shop", "en", "user@example.com", mrtype.NewIP(netip.MustParseAddr("203.0.113.7")))
	require.NoError(t, err)
	require.Equal(t, dto.User2FA{}, opFactory.gotUser2FA)
}

func TestCreateUser_ExistingUser2FAForwarded(t *testing.T) {
	t.Parallel()

	user2FA := dto.User2FA{
		ID:        uuid.New(),
		Email:     "user@example.com",
		Action2FA: secureoperation.ConfirmAction{Method: confirmmethod.TOTP},
	}
	opFactory := &fakeUserOpFactory{op: openedEmailOp(t)}
	uc := newCreateUser(
		fakeUserChecker{},
		&fakeOpCreator{},
		&fakeNotifier{},
		fakeUser2FAFactory{user: user2FA},
		&fakeLocker{},
		opFactory,
		&fakeOperationLogger{},
	)

	_, err := uc.Execute(context.Background(), "shop", "en", "user@example.com", mrtype.NewIP(netip.MustParseAddr("203.0.113.7")))
	require.NoError(t, err)
	require.Equal(t, user2FA, opFactory.gotUser2FA)
}
