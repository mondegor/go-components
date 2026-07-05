package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/auth2fatype"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/userstatus"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/service"
)

type (
	fakeUserStorage struct {
		user entity.User
		err  error
	}

	fakeUser2faStorage struct {
		row entity.Auth2FA
		err error
	}

	fakeActionFactory struct {
		action secureoperation.ConfirmAction
		err    error
	}
)

func (f fakeUserStorage) FetchOne(context.Context, uuid.UUID) (entity.User, error) {
	return f.user, f.err
}

func (f fakeUserStorage) FetchOneByLogin(context.Context, contactaddress.ContactAddress) (entity.User, error) {
	return f.user, f.err
}

func (f fakeUser2faStorage) FetchOne(context.Context, uuid.UUID) (entity.Auth2FA, error) {
	return f.row, f.err
}

func (f fakeActionFactory) Create(auth2fatype.Enum, string) (secureoperation.ConfirmAction, error) {
	return f.action, f.err
}

func TestFactoryConfirm2FA_CreateByUserLogin(t *testing.T) {
	t.Parallel()

	login := contactaddress.NewEmail("user@example.com")

	t.Run("user not found - error", func(t *testing.T) {
		t.Parallel()

		f := service.NewFactoryConfirm2FA(
			fakeUserStorage{err: errors.ErrEventStorageNoRecordFound},
			fakeUser2faStorage{},
			fakeActionFactory{},
		)

		_, err := f.CreateByUserLogin(context.Background(), login)
		require.ErrorIs(t, err, errors.ErrEventStorageNoRecordFound)
	})

	t.Run("existing user with 2fa - action populated", func(t *testing.T) {
		t.Parallel()

		userID := uuid.New()
		f := service.NewFactoryConfirm2FA(
			fakeUserStorage{user: entity.User{ID: userID, Email: "user@example.com", Status: userstatus.Enabled}},
			fakeUser2faStorage{row: entity.Auth2FA{UserID: userID, Type: auth2fatype.TOTP, Secret: "secret"}},
			fakeActionFactory{action: secureoperation.ConfirmAction{Method: confirmmethod.TOTP}},
		)

		got, err := f.CreateByUserLogin(context.Background(), login)
		require.NoError(t, err)
		assert.Equal(t, userID, got.ID)
		assert.Equal(t, confirmmethod.TOTP, got.Action2FA.Method)
	})

	t.Run("existing user without 2fa - empty action", func(t *testing.T) {
		t.Parallel()

		userID := uuid.New()
		f := service.NewFactoryConfirm2FA(
			fakeUserStorage{user: entity.User{ID: userID, Email: "user@example.com", Status: userstatus.Enabled}},
			fakeUser2faStorage{err: errors.ErrEventStorageNoRecordFound},
			fakeActionFactory{},
		)

		got, err := f.CreateByUserLogin(context.Background(), login)
		require.NoError(t, err)
		assert.Equal(t, userID, got.ID)
		assert.Equal(t, confirmmethod.Enum(0), got.Action2FA.Method)
	})

	t.Run("disabled user - error", func(t *testing.T) {
		t.Parallel()

		f := service.NewFactoryConfirm2FA(
			fakeUserStorage{user: entity.User{ID: uuid.New(), Status: userstatus.Disabled}},
			fakeUser2faStorage{},
			fakeActionFactory{},
		)

		_, err := f.CreateByUserLogin(context.Background(), login)
		require.Error(t, err)
	})
}
