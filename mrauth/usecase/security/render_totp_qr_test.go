package security_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/bag/totp"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/usecase/security"
	"github.com/mondegor/go-components/mrauth/usecase/security/mock"
)

// confirmedOp - подтверждённая операция смены TOTP с указанным payload'ом.
func confirmedOp(userID uuid.UUID, payload string) secureoperation.SecureOperation {
	return secureoperation.SecureOperation{
		Token:   "op-token",
		Name:    "confirm.change.totp",
		UserID:  userID,
		Payload: []byte(payload),
		Status:  operationstatus.Confirmed,
	}
}

type RenderTOTPQRSuite struct {
	baseSuite

	fetcher *mock.MockoperationFetcher
}

func TestRenderTOTPQRSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(RenderTOTPQRSuite))
}

func (s *RenderTOTPQRSuite) SetupTest() {
	s.baseSuite.SetupTest()

	s.fetcher = mock.NewMockoperationFetcher(s.ctrl)
}

func (s *RenderTOTPQRSuite) TestRendersQR() {
	userID := uuid.New()

	s.fetcher.EXPECT().
		FetchOne(gomock.Any(), "op-token").
		Return(confirmedOp(userID, `{"email":"u@e","secret":"`+testTotpSecret+`"}`), nil)

	uc := security.NewRenderTOTPGeneratorQR(s.fetcher, totp.NewAuthenticator("TestIssuer", 64))

	img, err := uc.Execute(s.ctx, userID, "op-token")
	s.Require().NoError(err)
	s.Equal("image/png", img.ContentType)
}
