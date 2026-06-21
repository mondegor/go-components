package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/mrlock"
	"github.com/mondegor/go-sysmess/mrstorage"
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
		op  secureoperation.SecureOperation
		err error
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

func (f fakeUserOpFactory) Create(string, contactaddress.ContactAddress) (secureoperation.SecureOperation, error) {
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
				Method:        confirmmethod.Email,
				MaxAttempts:   3,
				MaxResends:    5,
				MinResendTime: 5 * time.Minute,
				Expiry:        10 * time.Minute,
				Address:       "u@e",
				ConfirmCode:   "code123",
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
	locker *fakeLocker,
	opFactory fakeUserOpFactory,
) *auth.CreateUser {
	return auth.NewCreateUser(
		fakeTx{},
		checker,
		creator,
		notifier,
		locker,
		[]auth.CreateUserRealm{{Name: "shop", Operation: opFactory}},
	)
}

func TestCreateUser_Execute(t *testing.T) {
	t.Parallel()

	t.Run("unknown realm", func(t *testing.T) {
		t.Parallel()

		uc := newCreateUser(fakeUserChecker{}, &fakeOpCreator{}, &fakeNotifier{}, &fakeLocker{}, fakeUserOpFactory{})
		_, err := uc.Execute(context.Background(), "unknown", "en", "user@example.com")
		require.Error(t, err)
	})

	t.Run("invalid email", func(t *testing.T) {
		t.Parallel()

		uc := newCreateUser(fakeUserChecker{}, &fakeOpCreator{}, &fakeNotifier{}, &fakeLocker{}, fakeUserOpFactory{})
		_, err := uc.Execute(context.Background(), "shop", "en", "bad")
		require.Error(t, err)
	})

	t.Run("lock not obtained", func(t *testing.T) {
		t.Parallel()

		uc := newCreateUser(fakeUserChecker{}, &fakeOpCreator{}, &fakeNotifier{}, &fakeLocker{err: mrlock.ErrLockKeyNotObtained}, fakeUserOpFactory{})
		_, err := uc.Execute(context.Background(), "shop", "en", "user@example.com")
		require.ErrorIs(t, err, mrauth.ErrEmailAlreadyExists)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		creator := &fakeOpCreator{}
		notifier := &fakeNotifier{}
		uc := newCreateUser(fakeUserChecker{}, creator, notifier, &fakeLocker{}, fakeUserOpFactory{op: openedEmailOp(t)})

		_, err := uc.Execute(context.Background(), "shop", "en", "user@example.com")
		require.NoError(t, err)
		require.True(t, creator.inserted)
		require.True(t, notifier.sent)
	})

	t.Run("checker error", func(t *testing.T) {
		t.Parallel()

		uc := newCreateUser(fakeUserChecker{err: errors.New("boom")}, &fakeOpCreator{}, &fakeNotifier{}, &fakeLocker{}, fakeUserOpFactory{op: openedEmailOp(t)})
		_, err := uc.Execute(context.Background(), "shop", "en", "user@example.com")
		require.Error(t, err)
	})
}
