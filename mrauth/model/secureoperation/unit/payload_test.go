package unit_test

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
)

type (
	// invalidPayloadCase - случай нарушенного инварианта payload'а.
	invalidPayloadCase[T any] struct {
		name string
		in   T
	}
)

// assertPayloadHelpers - проверяет пару хелперов сборки/разбора payload'а одной операции:
// round-trip валидного значения, отказ разбора на повреждённых данных и отказ на каждом
// нарушенном инварианте - как при сборке, так и при разборе.
func assertPayloadHelpers[T any](
	t *testing.T,
	build func(T) ([]byte, error),
	parse func([]byte) (T, error),
	valid T,
	invalidCases []invalidPayloadCase[T],
) {
	t.Helper()

	t.Run("round trip", func(t *testing.T) {
		t.Parallel()

		raw, err := build(valid)
		require.NoError(t, err)

		got, err := parse(raw)
		require.NoError(t, err)
		assert.Equal(t, valid, got)
	})

	t.Run("parse broken json", func(t *testing.T) {
		t.Parallel()

		_, err := parse([]byte("{"))
		require.ErrorIs(t, err, errors.ErrInternalIncorrectInputData)
	})

	t.Run("parse nil", func(t *testing.T) {
		t.Parallel()

		_, err := parse(nil)
		require.ErrorIs(t, err, errors.ErrInternalIncorrectInputData)
	})

	for _, tt := range invalidCases {
		t.Run("build: "+tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := build(tt.in)
			require.ErrorIs(t, err, errors.ErrInternalIncorrectInputData)
		})

		t.Run("parse: "+tt.name, func(t *testing.T) {
			t.Parallel()

			// payload сериализуется в обход Build - проверяется именно защита на чтении
			raw, err := json.Marshal(tt.in)
			require.NoError(t, err)

			_, err = parse(raw)
			require.ErrorIs(t, err, errors.ErrInternalIncorrectInputData)
		})
	}
}

func TestCreateUserPayload(t *testing.T) {
	t.Parallel()

	valid := dto.CreateUserOperation{
		Realm:        "site/admin",
		UserKind:     "customer",
		LangCode:     "en",
		TimeZone:     "Europe/Moscow",
		Email:        "user@example.com",
		RegisteredIP: mrtype.NewIP(netip.MustParseAddr("203.0.113.7")),
	}

	broken := func(f func(*dto.CreateUserOperation)) dto.CreateUserOperation {
		v := valid
		f(&v)

		return v
	}

	assertPayloadHelpers(
		t,
		unit.BuildCreateUserPayload,
		unit.ParseCreateUserPayload,
		valid,
		[]invalidPayloadCase[dto.CreateUserOperation]{
			{name: "empty realm", in: broken(func(v *dto.CreateUserOperation) { v.Realm = "" })},
			{name: "empty langCode", in: broken(func(v *dto.CreateUserOperation) { v.LangCode = "" })},
			{name: "empty timeZone", in: broken(func(v *dto.CreateUserOperation) { v.TimeZone = "" })},
			{name: "empty email", in: broken(func(v *dto.CreateUserOperation) { v.Email = "" })},
		},
	)
}

// UserKind и RegisteredIP не входят в инварианты payload'а создания пользователя,
// поэтому их отсутствие не должно мешать ни сборке, ни разбору.
func TestCreateUserPayload_OptionalFieldsAreNotRequired(t *testing.T) {
	t.Parallel()

	in := dto.CreateUserOperation{
		Realm:    "site/admin",
		LangCode: "en",
		TimeZone: "Europe/Moscow",
		Email:    "user@example.com",
	}

	raw, err := unit.BuildCreateUserPayload(in)
	require.NoError(t, err)

	got, err := unit.ParseCreateUserPayload(raw)
	require.NoError(t, err)
	assert.Equal(t, in, got)
}

func TestAuthorizeUserPayload(t *testing.T) {
	t.Parallel()

	valid := dto.AuthorizeUserOperation{
		Realm:    "site/admin",
		LangCode: "en",
	}

	assertPayloadHelpers(
		t,
		unit.BuildAuthorizeUserPayload,
		unit.ParseAuthorizeUserPayload,
		valid,
		[]invalidPayloadCase[dto.AuthorizeUserOperation]{
			{name: "empty realm", in: dto.AuthorizeUserOperation{LangCode: "en"}},
			{name: "empty langCode", in: dto.AuthorizeUserOperation{Realm: "site/admin"}},
		},
	)
}

func TestChangeEmailPayload(t *testing.T) {
	t.Parallel()

	valid := dto.ChangeEmailOperation{
		NewEmail: "new@example.com",
		Email:    "old@example.com",
	}

	assertPayloadHelpers(
		t,
		unit.BuildChangeEmailPayload,
		unit.ParseChangeEmailPayload,
		valid,
		[]invalidPayloadCase[dto.ChangeEmailOperation]{
			{name: "empty newEmail", in: dto.ChangeEmailOperation{Email: "old@example.com"}},
			{name: "empty email", in: dto.ChangeEmailOperation{NewEmail: "new@example.com"}},
		},
	)
}

func TestChangePasswordPayload(t *testing.T) {
	t.Parallel()

	valid := dto.ChangePasswordOperation{
		NewPassword: "hashed-password",
		Email:       "user@example.com",
	}

	assertPayloadHelpers(
		t,
		unit.BuildChangePasswordPayload,
		unit.ParseChangePasswordPayload,
		valid,
		[]invalidPayloadCase[dto.ChangePasswordOperation]{
			{name: "empty newPassword", in: dto.ChangePasswordOperation{Email: "user@example.com"}},
			{name: "empty email", in: dto.ChangePasswordOperation{NewPassword: "hashed-password"}},
		},
	)
}

func TestChangePhonePayload(t *testing.T) {
	t.Parallel()

	valid := dto.ChangePhoneOperation{
		NewPhone: 79001234567,
		Email:    "user@example.com",
	}

	assertPayloadHelpers(
		t,
		unit.BuildChangePhonePayload,
		unit.ParseChangePhonePayload,
		valid,
		[]invalidPayloadCase[dto.ChangePhoneOperation]{
			{name: "zero newPhone", in: dto.ChangePhoneOperation{Email: "user@example.com"}},
			{name: "empty email", in: dto.ChangePhoneOperation{NewPhone: 79001234567}},
		},
	)
}

func TestChangeTOTPPayload(t *testing.T) {
	t.Parallel()

	valid := dto.ChangeTOTPOperation{
		Email:  "user@example.com",
		Secret: "JBSWY3DPEHPK3PXP",
	}

	assertPayloadHelpers(
		t,
		unit.BuildChangeTOTPPayload,
		unit.ParseChangeTOTPPayload,
		valid,
		[]invalidPayloadCase[dto.ChangeTOTPOperation]{
			{name: "empty secret", in: dto.ChangeTOTPOperation{Email: "user@example.com"}},
			{name: "empty email", in: dto.ChangeTOTPOperation{Secret: "JBSWY3DPEHPK3PXP"}},
		},
	)
}

func TestDisable2FAPayload(t *testing.T) {
	t.Parallel()

	valid := dto.Disable2FAOperation{Email: "user@example.com"}

	assertPayloadHelpers(
		t,
		unit.BuildDisable2FAPayload,
		unit.ParseDisable2FAPayload,
		valid,
		[]invalidPayloadCase[dto.Disable2FAOperation]{
			{name: "empty email", in: dto.Disable2FAOperation{}},
		},
	)
}

func TestRegenerateRecoveryPayload(t *testing.T) {
	t.Parallel()

	valid := dto.OperationWithUserEmail{Email: "user@example.com"}

	assertPayloadHelpers(
		t,
		unit.BuildRegenerateRecoveryPayload,
		unit.ParseRegenerateRecoveryPayload,
		valid,
		[]invalidPayloadCase[dto.OperationWithUserEmail]{
			{name: "empty email", in: dto.OperationWithUserEmail{}},
		},
	)
}
