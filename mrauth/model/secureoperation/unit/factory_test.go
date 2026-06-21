package unit_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit/action"
)

type (
	fakeTokenGen struct {
		token string
		err   error
	}

	fakeCodeGen struct {
		code string
		hash string
		err  error
	}

	fakeSecretGen struct {
		secret string
		err    error
	}
)

func (g fakeTokenGen) GenToken() (string, error) { return g.token, g.err }

func (g fakeCodeGen) GenCode() (string, error) { return g.code, g.err }

func (g fakeCodeGen) HashedSecret(string) (string, error) {
	if g.err != nil {
		return "", g.err
	}

	return g.hash, nil
}

func (g fakeCodeGen) CompareSecretAndHash(string, string) error { return nil }

func (g fakeSecretGen) GenerateSecret(string) (string, error) { return g.secret, g.err }

// userWith2FA - пользователь с активным вторым фактором (TOTP).
func userWith2FA() dto.User2FA {
	return dto.User2FA{
		ID:        uuid.New(),
		Email:     "user@example.com",
		Action2FA: secureoperation.ConfirmAction{Method: confirmmethod.TOTP, MaxAttempts: 3, Expiry: time.Minute},
	}
}

// userWithout2FA - пользователь без второго фактора.
func userWithout2FA() dto.User2FA {
	return dto.User2FA{ID: uuid.New(), Email: "user@example.com"}
}

func TestChangeEmail_Create(t *testing.T) {
	t.Parallel()

	t.Run("without 2fa - single action", func(t *testing.T) {
		t.Parallel()

		f := unit.NewChangeEmail(fakeTokenGen{token: "tok"}, fakeCodeGen{code: "123456"})

		op, err := f.Create(userWithout2FA(), "new@example.com")
		require.NoError(t, err)
		assert.Equal(t, unit.NameConfirmChangeEmail, op.Name)
		require.Len(t, op.Actions(), 1)

		var p dto.ChangeEmailOperation
		require.NoError(t, json.Unmarshal(op.Payload, &p))
		assert.Equal(t, "new@example.com", p.NewEmail)
		assert.Equal(t, "user@example.com", p.NotifyByEmail)
	})

	t.Run("with 2fa - appends second action", func(t *testing.T) {
		t.Parallel()

		f := unit.NewChangeEmail(fakeTokenGen{token: "tok"}, fakeCodeGen{code: "123456"})

		op, err := f.Create(userWith2FA(), "new@example.com")
		require.NoError(t, err)
		require.Len(t, op.Actions(), 2)
	})

	t.Run("token generator error", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("token failed")
		f := unit.NewChangeEmail(fakeTokenGen{err: wantErr}, fakeCodeGen{code: "123456"})

		_, err := f.Create(userWithout2FA(), "new@example.com")
		require.ErrorIs(t, err, wantErr)
	})
}

func TestChangePassword_Create(t *testing.T) {
	t.Parallel()

	f := unit.NewChangePassword(fakeTokenGen{token: "tok"}, fakeCodeGen{code: "123456", hash: "hashed-pw"})

	op, err := f.Create(userWithout2FA(), "new-password")
	require.NoError(t, err)
	assert.Equal(t, unit.NameConfirmChangePassword, op.Name)

	var p dto.ChangePasswordOperation
	require.NoError(t, json.Unmarshal(op.Payload, &p))
	assert.Equal(t, "hashed-pw", p.NewPassword) // хранится хеш, не открытый пароль
	assert.Equal(t, "user@example.com", p.NotifyByEmail)

	op2fa, err := f.Create(userWith2FA(), "new-password")
	require.NoError(t, err)
	require.Len(t, op2fa.Actions(), 2)
}

func TestChangePhone_Create(t *testing.T) {
	t.Parallel()

	f := unit.NewChangePhone(fakeTokenGen{token: "tok"}, fakeCodeGen{code: "123456"})

	op, err := f.Create(userWithout2FA(), "79991234567")
	require.NoError(t, err)
	assert.Equal(t, unit.NameConfirmChangePhone, op.Name)

	var p dto.ChangePhoneOperation
	require.NoError(t, json.Unmarshal(op.Payload, &p))
	assert.Equal(t, uint64(79991234567), p.NewPhone)
	assert.Equal(t, "user@example.com", p.NotifyByEmail)

	op2fa, err := f.Create(userWith2FA(), "79991234567")
	require.NoError(t, err)
	require.Len(t, op2fa.Actions(), 2)
}

func TestChangePhone_Create_InvalidPhone(t *testing.T) {
	t.Parallel()

	f := unit.NewChangePhone(fakeTokenGen{token: "tok"}, fakeCodeGen{code: "123456"})

	_, err := f.Create(userWithout2FA(), "not-a-number")
	require.Error(t, err)
}

func TestChangeTOTP_Create(t *testing.T) {
	t.Parallel()

	f := unit.NewChangeTOTP(
		fakeTokenGen{token: "tok"},
		fakeCodeGen{code: "123456"},
		fakeSecretGen{secret: "TOTPSECRET"},
	)

	op, err := f.Create(userWithout2FA())
	require.NoError(t, err)
	assert.Equal(t, unit.NameConfirmChangeTOTP, op.Name)

	var p dto.ChangeTotpOperation
	require.NoError(t, json.Unmarshal(op.Payload, &p))
	assert.Equal(t, "TOTPSECRET", p.Secret)
	assert.Equal(t, "user@example.com", p.Email)

	op2fa, err := f.Create(userWith2FA())
	require.NoError(t, err)
	require.Len(t, op2fa.Actions(), 2)
}

func TestCreateUser_Create(t *testing.T) {
	t.Parallel()

	f := unit.NewCreateUser("shop", "customer", fakeTokenGen{token: "tok"}, fakeCodeGen{code: "123456"})

	op, err := f.Create("en", contactaddress.NewEmail("user@example.com"))
	require.NoError(t, err)
	assert.Equal(t, unit.NameConfirmCreateUser, op.Name)
	assert.Equal(t, uuid.Nil, op.UserID)
	require.Len(t, op.Actions(), 1)

	var p dto.CreateUserOperation
	require.NoError(t, json.Unmarshal(op.Payload, &p))
	assert.Equal(t, "shop", p.Realm)
	assert.Equal(t, "customer", p.UserKind)
	assert.Equal(t, "en", p.LangCode)
	assert.Equal(t, "user@example.com", p.Email)
}

func TestDisable2FA_Create(t *testing.T) {
	t.Parallel()

	t.Run("with active 2fa", func(t *testing.T) {
		t.Parallel()

		f := unit.NewDisable2FA(fakeTokenGen{token: "tok"}, fakeCodeGen{code: "123456"})

		op, err := f.Create(userWith2FA())
		require.NoError(t, err)
		assert.Equal(t, unit.NameConfirmDisable2FA, op.Name)
		require.Len(t, op.Actions(), 2)

		var p dto.Disable2faOperation
		require.NoError(t, json.Unmarshal(op.Payload, &p))
		assert.Equal(t, "user@example.com", p.Email)
	})

	t.Run("already disabled fails", func(t *testing.T) {
		t.Parallel()

		f := unit.NewDisable2FA(fakeTokenGen{token: "tok"}, fakeCodeGen{code: "123456"})

		_, err := f.Create(userWithout2FA())
		require.ErrorContains(t, err, "2fa already disabled")
	})
}

func TestAuthorizeUser_Create(t *testing.T) {
	t.Parallel()

	f := unit.NewAuthorizeUser(fakeTokenGen{token: "tok"}, fakeCodeGen{code: "123456"})

	op, err := f.Create(userWithout2FA(), "shop", "en", contactaddress.NewEmail("login@example.com"))
	require.NoError(t, err)
	assert.Equal(t, unit.NameAuthorizeUser, op.Name)

	var p dto.AuthorizeUserOperation
	require.NoError(t, json.Unmarshal(op.Payload, &p))
	assert.Equal(t, "shop", p.Realm)
	assert.Equal(t, "en", p.LangCode)
}

func TestAuthorizeUser_Create_PhoneConvertedToEmail(t *testing.T) {
	t.Parallel()

	// confirmPhoneByEmail по умолчанию true: телефонный логин подтверждается по email.
	f := unit.NewAuthorizeUser(fakeTokenGen{token: "tok"}, fakeCodeGen{code: "123456"})

	op, err := f.Create(userWith2FA(), "shop", "en", contactaddress.NewPhone("79991234567"))
	require.NoError(t, err)
	require.Len(t, op.Actions(), 2)

	action, ok := op.FirstAction()
	require.True(t, ok)
	assert.Equal(t, confirmmethod.Email, action.Method)
}

func TestAuthorizeUser_Create_PhoneLoginWithOptions(t *testing.T) {
	t.Parallel()

	f := unit.NewAuthorizeUser(
		fakeTokenGen{token: "tok"},
		fakeCodeGen{code: "123456"},
		unit.WithAuthorizeUserConfirmByEmailOpts(action.WithMaxAttempts(5)),
		unit.WithAuthorizeUserConfirmByPhoneOpts(action.WithMaxAttempts(5)),
		unit.WithAuthorizeUserConfirmPhoneByEmail(false),
	)

	op, err := f.Create(userWithout2FA(), "shop", "en", contactaddress.NewPhone("79991234567"))
	require.NoError(t, err)

	action, ok := op.FirstAction()
	require.True(t, ok)
	assert.Equal(t, confirmmethod.Phone, action.Method)
}
