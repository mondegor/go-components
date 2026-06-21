package security_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/bag/totp"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/usecase/security"
)

type fakeFetcher struct {
	op  secureoperation.SecureOperation
	err error
}

// FetchOne - возвращает заранее заданную операцию (других методов записи у фейка нет).
func (f *fakeFetcher) FetchOne(_ context.Context, _ string) (secureoperation.SecureOperation, error) {
	return f.op, f.err
}

func confirmedOp(userID uuid.UUID, payload string) secureoperation.SecureOperation {
	return secureoperation.SecureOperation{
		Token:   "op-token",
		Name:    "confirm.change.totp",
		UserID:  userID,
		Payload: []byte(payload),
		Status:  operationstatus.Confirmed,
	}
}

func TestApplyTOTPGenerator_RendersQR(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	secret := testTotpSecret
	fetcher := &fakeFetcher{op: confirmedOp(userID, `{"email":"u@e","secret":"`+secret+`"}`)}

	uc := security.NewRenderTOTPGeneratorQR(fetcher, totp.NewAuthenticator("TestIssuer", 64))

	img, err := uc.Execute(context.Background(), userID, "op-token")
	require.NoError(t, err)
	require.Equal(t, "image/png", img.ContentType)
}
