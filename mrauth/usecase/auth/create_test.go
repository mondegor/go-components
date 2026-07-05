package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	sysmesserrors "github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrlock"
	"github.com/mondegor/go-sysmess/mrstorage"
	"github.com/mondegor/go-sysmess/mrtype"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
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
		gotRegisteredIP string
	}

	fakeLocker struct {
		err error
	}
)

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

func (f fakeSessionOpFactory) Create(dto.User2FA, string, string, contactaddress.ContactAddress) (secureoperation.SecureOperation, error) {
	return f.op, f.err
}

func (f *fakeUserOpFactory) Create(
	user2FA dto.User2FA,
	_ string,
	_ contactaddress.ContactAddress,
	registeredIP string,
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
) *auth.CreateSession {
	return auth.NewCreateSession(
		fakeTx{},
		checker,
		creator,
		notifier,
		fakeUser2FAFactory{},
		[]auth.CreateSessionRealm{{Name: "shop", Operation: opFactory}},
	)
}

func TestCreateSession_Execute(t *testing.T) {
	t.Parallel()

	t.Run("empty login", func(t *testing.T) {
		t.Parallel()

		uc := newCreateSession(fakeUserChecker{}, &fakeOpCreator{}, &fakeNotifier{}, fakeSessionOpFactory{})
		_, err := uc.Execute(context.Background(), "shop", "en", "")
		require.Error(t, err)
	})

	t.Run("unknown realm", func(t *testing.T) {
		t.Parallel()

		uc := newCreateSession(fakeUserChecker{}, &fakeOpCreator{}, &fakeNotifier{}, fakeSessionOpFactory{})
		_, err := uc.Execute(context.Background(), "unknown", "en", "user@example.com")
		require.Error(t, err)
	})

	t.Run("login does not exist", func(t *testing.T) {
		t.Parallel()

		// nil от CheckAvailabilityRealm означает, что логин свободен - значит входить некому.
		uc := newCreateSession(fakeUserChecker{}, &fakeOpCreator{}, &fakeNotifier{}, fakeSessionOpFactory{op: openedEmailOp(t)})
		_, err := uc.Execute(context.Background(), "shop", "en", "user@example.com")
		require.ErrorIs(t, err, mrauth.ErrLoginNotExists)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		creator := &fakeOpCreator{}
		notifier := &fakeNotifier{}
		uc := newCreateSession(fakeUserChecker{err: mrauth.ErrEmailAlreadyExists}, creator, notifier, fakeSessionOpFactory{op: openedEmailOp(t)})

		_, err := uc.Execute(context.Background(), "shop", "en", "user@example.com")
		require.NoError(t, err)
		require.True(t, creator.inserted)
		require.True(t, notifier.sent)
	})

	t.Run("checker error", func(t *testing.T) {
		t.Parallel()

		uc := newCreateSession(fakeUserChecker{err: errors.New("boom")}, &fakeOpCreator{}, &fakeNotifier{}, fakeSessionOpFactory{op: openedEmailOp(t)})
		_, err := uc.Execute(context.Background(), "shop", "en", "user@example.com")
		require.Error(t, err)
	})
}

func newCreateUser(
	checker fakeUserChecker,
	creator *fakeOpCreator,
	notifier *fakeNotifier,
	factory fakeUser2FAFactory,
	locker *fakeLocker,
	opFactory *fakeUserOpFactory,
) *auth.CreateUser {
	return auth.NewCreateUser(
		fakeTx{},
		checker,
		creator,
		notifier,
		factory,
		locker,
		[]auth.CreateUserRealm{{Name: "shop", Operation: opFactory}},
	)
}

func TestCreateUser_Execute(t *testing.T) {
	t.Parallel()

	t.Run("unknown realm", func(t *testing.T) {
		t.Parallel()

		uc := newCreateUser(fakeUserChecker{}, &fakeOpCreator{}, &fakeNotifier{}, fakeUser2FAFactory{}, &fakeLocker{}, &fakeUserOpFactory{})
		_, err := uc.Execute(context.Background(), "unknown", "en", "user@example.com", mrtype.NewIP(3405803783))
		require.Error(t, err)
	})

	t.Run("invalid email", func(t *testing.T) {
		t.Parallel()

		uc := newCreateUser(fakeUserChecker{}, &fakeOpCreator{}, &fakeNotifier{}, fakeUser2FAFactory{}, &fakeLocker{}, &fakeUserOpFactory{})
		_, err := uc.Execute(context.Background(), "shop", "en", "bad", mrtype.NewIP(3405803783))
		require.Error(t, err)
	})

	t.Run("lock not obtained", func(t *testing.T) {
		t.Parallel()

		uc := newCreateUser(
			fakeUserChecker{},
			&fakeOpCreator{},
			&fakeNotifier{},
			fakeUser2FAFactory{},
			&fakeLocker{err: mrlock.ErrLockKeyNotObtained},
			&fakeUserOpFactory{},
		)
		_, err := uc.Execute(context.Background(), "shop", "en", "user@example.com", mrtype.NewIP(3405803783))
		require.ErrorIs(t, err, mrauth.ErrSignupAlreadyInProgressTryLater)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		creator := &fakeOpCreator{}
		notifier := &fakeNotifier{}
		opFactory := &fakeUserOpFactory{op: openedEmailOp(t)}
		uc := newCreateUser(fakeUserChecker{}, creator, notifier, fakeUser2FAFactory{}, &fakeLocker{}, opFactory)

		_, err := uc.Execute(context.Background(), "shop", "en", "user@example.com", mrtype.NewIP(3405803783))
		require.NoError(t, err)
		require.True(t, creator.inserted)
		require.True(t, notifier.sent)
		require.Equal(t, "203.0.113.7", opFactory.gotRegisteredIP, "IP регистрации доезжает до фабрики операции")
	})

	t.Run("checker error", func(t *testing.T) {
		t.Parallel()

		uc := newCreateUser(
			fakeUserChecker{err: errors.New("boom")},
			&fakeOpCreator{},
			&fakeNotifier{},
			fakeUser2FAFactory{},
			&fakeLocker{},
			&fakeUserOpFactory{op: openedEmailOp(t)},
		)
		_, err := uc.Execute(context.Background(), "shop", "en", "user@example.com", mrtype.NewIP(3405803783))
		require.Error(t, err)
	})

	t.Run("2fa factory error", func(t *testing.T) {
		t.Parallel()

		factory := fakeUser2FAFactory{err: errors.New("2fa boom")}
		uc := newCreateUser(fakeUserChecker{}, &fakeOpCreator{}, &fakeNotifier{}, factory, &fakeLocker{}, &fakeUserOpFactory{op: openedEmailOp(t)})
		_, err := uc.Execute(context.Background(), "shop", "en", "user@example.com", mrtype.NewIP(3405803783))
		require.Error(t, err)
	})

	t.Run("new email - empty 2fa forwarded to operation", func(t *testing.T) {
		t.Parallel()

		// логин не принадлежит существующему пользователю: фабрика 2FA возвращает
		// ErrEventStorageNoRecordFound, usecase продолжает выполнение с пустым User2FA
		factory := fakeUser2FAFactory{err: sysmesserrors.ErrEventStorageNoRecordFound}
		opFactory := &fakeUserOpFactory{op: openedEmailOp(t)}
		uc := newCreateUser(fakeUserChecker{}, &fakeOpCreator{}, &fakeNotifier{}, factory, &fakeLocker{}, opFactory)

		_, err := uc.Execute(context.Background(), "shop", "en", "user@example.com", mrtype.NewIP(3405803783))
		require.NoError(t, err)
		require.Equal(t, dto.User2FA{}, opFactory.gotUser2FA)
	})

	t.Run("existing user 2fa forwarded to operation", func(t *testing.T) {
		t.Parallel()

		user2FA := dto.User2FA{
			ID:        uuid.New(),
			Email:     "user@example.com",
			Action2FA: secureoperation.ConfirmAction{Method: confirmmethod.TOTP},
		}
		opFactory := &fakeUserOpFactory{op: openedEmailOp(t)}
		uc := newCreateUser(fakeUserChecker{}, &fakeOpCreator{}, &fakeNotifier{}, fakeUser2FAFactory{user: user2FA}, &fakeLocker{}, opFactory)

		_, err := uc.Execute(context.Background(), "shop", "en", "user@example.com", mrtype.NewIP(3405803783))
		require.NoError(t, err)
		require.Equal(t, user2FA, opFactory.gotUser2FA)
	})
}
