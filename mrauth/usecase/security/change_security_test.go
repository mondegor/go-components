package security_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/usecase/security"
)

type (
	fakeOpCreator struct {
		inserted bool
		err      error
	}

	fakeUser2FAFactory struct {
		user dto.User2FA
		err  error
	}

	fakeValueOpFactory struct {
		op  secureoperation.SecureOperation
		err error
	}

	fakeOpFactory struct {
		op  secureoperation.SecureOperation
		err error
	}

	fakeEmailChecker struct {
		err error
	}

	fakePhoneChecker struct {
		err error
	}
)

func (f *fakeOpCreator) Insert(context.Context, secureoperation.SecureOperation) error {
	if f.err != nil {
		return f.err
	}

	f.inserted = true

	return nil
}

func (f fakeUser2FAFactory) CreateByUserID(context.Context, uuid.UUID) (dto.User2FA, error) {
	return f.user, f.err
}

func (f fakeUser2FAFactory) CreateByUserLogin(context.Context, contactaddress.ContactAddress) (dto.User2FA, error) {
	return f.user, f.err
}

func (f fakeValueOpFactory) Create(dto.User2FA, string) (secureoperation.SecureOperation, error) {
	return f.op, f.err
}

func (f fakeOpFactory) Create(dto.User2FA) (secureoperation.SecureOperation, error) {
	return f.op, f.err
}

func (f fakeEmailChecker) CheckAvailabilityEmail(context.Context, contactaddress.ContactAddress) error {
	return f.err
}

func (f fakePhoneChecker) CheckAvailabilityPhone(context.Context, contactaddress.ContactAddress) error {
	return f.err
}

// openedEmailOp - sendable-операция Email, при Notify отправляющая код через notifier.
func openedEmailOp(t *testing.T) secureoperation.SecureOperation {
	t.Helper()

	op, err := secureoperation.NewOperation(
		"op-token",
		"confirm.change",
		uuid.New(),
		[]secureoperation.ConfirmAction{
			{
				Method:           confirmmethod.Email,
				MaxAttempts:      3,
				MaxResends:       5,
				MinResendTime:    5 * time.Minute,
				Expiry:           10 * time.Minute,
				Address:          "u@e",
				ConfirmCode:      "code123", // в хранилище идёт хеш
				PlainConfirmCode: "code123", // открытый код - для отправки через Notify
			},
		},
		nil,
	)
	require.NoError(t, err)

	return op
}

func userWithEmail() dto.User2FA {
	return dto.User2FA{ID: uuid.New(), Email: "user@example.com"}
}

func TestChangeEmailProperty_Execute(t *testing.T) {
	t.Parallel()

	t.Run("nil userID", func(t *testing.T) {
		t.Parallel()

		uc := security.NewChangeEmailProperty(
			fakeTx{}, &fakeOpCreator{}, fakeEmailChecker{}, &fakeNotifier{}, fakeUser2FAFactory{}, fakeValueOpFactory{},
		)
		_, err := uc.Execute(context.Background(), uuid.Nil, "new@example.com")
		require.Error(t, err)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		creator := &fakeOpCreator{}
		notifier := &fakeNotifier{}
		uc := security.NewChangeEmailProperty(
			fakeTx{}, creator, fakeEmailChecker{}, notifier,
			fakeUser2FAFactory{user: userWithEmail()}, fakeValueOpFactory{op: openedEmailOp(t)},
		)

		_, err := uc.Execute(context.Background(), uuid.New(), "new@example.com")
		require.NoError(t, err)
		require.True(t, creator.inserted)
		require.True(t, notifier.sent)
	})

	t.Run("invalid email", func(t *testing.T) {
		t.Parallel()

		uc := security.NewChangeEmailProperty(
			fakeTx{}, &fakeOpCreator{}, fakeEmailChecker{}, &fakeNotifier{}, fakeUser2FAFactory{}, fakeValueOpFactory{},
		)
		_, err := uc.Execute(context.Background(), uuid.New(), "bad")
		require.Error(t, err)
	})

	t.Run("email unavailable", func(t *testing.T) {
		t.Parallel()

		uc := security.NewChangeEmailProperty(
			fakeTx{}, &fakeOpCreator{}, fakeEmailChecker{err: errors.New("taken")}, &fakeNotifier{},
			fakeUser2FAFactory{}, fakeValueOpFactory{op: openedEmailOp(t)},
		)
		_, err := uc.Execute(context.Background(), uuid.New(), "new@example.com")
		require.Error(t, err)
	})

	t.Run("user2fa factory error", func(t *testing.T) {
		t.Parallel()

		uc := security.NewChangeEmailProperty(
			fakeTx{}, &fakeOpCreator{}, fakeEmailChecker{}, &fakeNotifier{},
			fakeUser2FAFactory{err: errors.New("no user")}, fakeValueOpFactory{op: openedEmailOp(t)},
		)
		_, err := uc.Execute(context.Background(), uuid.New(), "new@example.com")
		require.Error(t, err)
	})

	t.Run("insert error", func(t *testing.T) {
		t.Parallel()

		uc := security.NewChangeEmailProperty(
			fakeTx{}, &fakeOpCreator{err: errors.New("insert failed")}, fakeEmailChecker{}, &fakeNotifier{},
			fakeUser2FAFactory{user: userWithEmail()}, fakeValueOpFactory{op: openedEmailOp(t)},
		)
		_, err := uc.Execute(context.Background(), uuid.New(), "new@example.com")
		require.Error(t, err)
	})
}

func TestChangePasswordProperty_Execute(t *testing.T) {
	t.Parallel()

	t.Run("nil userID", func(t *testing.T) {
		t.Parallel()

		uc := security.NewChangePasswordProperty(fakeTx{}, &fakeOpCreator{}, &fakeNotifier{}, fakeUser2FAFactory{}, fakeValueOpFactory{})
		_, err := uc.Execute(context.Background(), uuid.Nil, "new-password")
		require.Error(t, err)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		creator := &fakeOpCreator{}
		notifier := &fakeNotifier{}
		uc := security.NewChangePasswordProperty(
			fakeTx{}, creator, notifier, fakeUser2FAFactory{user: userWithEmail()}, fakeValueOpFactory{op: openedEmailOp(t)},
		)

		_, err := uc.Execute(context.Background(), uuid.New(), "new-password")
		require.NoError(t, err)
		require.True(t, creator.inserted)
		require.True(t, notifier.sent)
	})

	t.Run("factory error", func(t *testing.T) {
		t.Parallel()

		uc := security.NewChangePasswordProperty(
			fakeTx{}, &fakeOpCreator{}, &fakeNotifier{}, fakeUser2FAFactory{}, fakeValueOpFactory{err: errors.New("factory failed")},
		)
		_, err := uc.Execute(context.Background(), uuid.New(), "new-password")
		require.Error(t, err)
	})
}

func TestChangePhoneProperty_Execute(t *testing.T) {
	t.Parallel()

	t.Run("nil userID", func(t *testing.T) {
		t.Parallel()

		uc := security.NewChangePhoneProperty(
			fakeTx{}, &fakeOpCreator{}, fakePhoneChecker{}, &fakeNotifier{}, fakeUser2FAFactory{}, fakeValueOpFactory{},
		)
		_, err := uc.Execute(context.Background(), uuid.Nil, "79991234567")
		require.Error(t, err)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		creator := &fakeOpCreator{}
		notifier := &fakeNotifier{}
		uc := security.NewChangePhoneProperty(
			fakeTx{}, creator, fakePhoneChecker{}, notifier,
			fakeUser2FAFactory{user: userWithEmail()}, fakeValueOpFactory{op: openedEmailOp(t)},
		)

		_, err := uc.Execute(context.Background(), uuid.New(), "79991234567")
		require.NoError(t, err)
		require.True(t, creator.inserted)
		require.True(t, notifier.sent)
	})

	t.Run("invalid phone", func(t *testing.T) {
		t.Parallel()

		uc := security.NewChangePhoneProperty(
			fakeTx{}, &fakeOpCreator{}, fakePhoneChecker{}, &fakeNotifier{}, fakeUser2FAFactory{}, fakeValueOpFactory{},
		)
		_, err := uc.Execute(context.Background(), uuid.New(), "bad")
		require.Error(t, err)
	})

	t.Run("phone unavailable", func(t *testing.T) {
		t.Parallel()

		uc := security.NewChangePhoneProperty(
			fakeTx{}, &fakeOpCreator{}, fakePhoneChecker{err: errors.New("taken")}, &fakeNotifier{},
			fakeUser2FAFactory{}, fakeValueOpFactory{op: openedEmailOp(t)},
		)
		_, err := uc.Execute(context.Background(), uuid.New(), "79991234567")
		require.Error(t, err)
	})
}

func TestChangeTOTPGeneratorProperty_Execute(t *testing.T) {
	t.Parallel()

	t.Run("nil userID", func(t *testing.T) {
		t.Parallel()

		uc := security.NewChangeTOTPGeneratorProperty(fakeTx{}, &fakeOpCreator{}, &fakeNotifier{}, fakeUser2FAFactory{}, fakeOpFactory{})
		_, err := uc.Execute(context.Background(), uuid.Nil)
		require.Error(t, err)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		creator := &fakeOpCreator{}
		notifier := &fakeNotifier{}
		uc := security.NewChangeTOTPGeneratorProperty(
			fakeTx{}, creator, notifier, fakeUser2FAFactory{user: userWithEmail()}, fakeOpFactory{op: openedEmailOp(t)},
		)

		_, err := uc.Execute(context.Background(), uuid.New())
		require.NoError(t, err)
		require.True(t, creator.inserted)
		require.True(t, notifier.sent)
	})

	t.Run("factory error", func(t *testing.T) {
		t.Parallel()

		uc := security.NewChangeTOTPGeneratorProperty(
			fakeTx{}, &fakeOpCreator{}, &fakeNotifier{}, fakeUser2FAFactory{}, fakeOpFactory{err: errors.New("factory failed")},
		)
		_, err := uc.Execute(context.Background(), uuid.New())
		require.Error(t, err)
	})

	t.Run("insert error", func(t *testing.T) {
		t.Parallel()

		uc := security.NewChangeTOTPGeneratorProperty(
			fakeTx{}, &fakeOpCreator{err: errors.New("insert failed")}, &fakeNotifier{},
			fakeUser2FAFactory{user: userWithEmail()}, fakeOpFactory{op: openedEmailOp(t)},
		)
		_, err := uc.Execute(context.Background(), uuid.New())
		require.Error(t, err)
	})
}

func TestDisable2FA_Execute(t *testing.T) {
	t.Parallel()

	t.Run("nil userID", func(t *testing.T) {
		t.Parallel()

		uc := security.NewDisable2FA(fakeTx{}, &fakeOpCreator{}, &fakeNotifier{}, fakeUser2FAFactory{}, fakeOpFactory{})
		_, err := uc.Execute(context.Background(), uuid.Nil)
		require.Error(t, err)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		creator := &fakeOpCreator{}
		notifier := &fakeNotifier{}
		uc := security.NewDisable2FA(
			fakeTx{}, creator, notifier, fakeUser2FAFactory{user: userWithEmail()}, fakeOpFactory{op: openedEmailOp(t)},
		)

		_, err := uc.Execute(context.Background(), uuid.New())
		require.NoError(t, err)
		require.True(t, creator.inserted)
		require.True(t, notifier.sent)
	})

	t.Run("factory error", func(t *testing.T) {
		t.Parallel()

		uc := security.NewDisable2FA(
			fakeTx{}, &fakeOpCreator{}, &fakeNotifier{}, fakeUser2FAFactory{}, fakeOpFactory{err: errors.New("factory failed")},
		)
		_, err := uc.Execute(context.Background(), uuid.New())
		require.Error(t, err)
	})
}
