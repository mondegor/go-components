package security_test

import (
	"image"
	"testing"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/bag/totp"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/enum/operationtype"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/usecase/security"
	"github.com/mondegor/go-components/mrauth/usecase/security/mock"
)

//go:generate mockgen -source=render_totp_qr.go -destination=mock/render_totp_qr.go -package=mock

// confirmedOp - подтверждённая операция смены TOTP с указанным payload'ом.
func confirmedOp(userID uuid.UUID, payload string) secureoperation.SecureOperation {
	return secureoperation.SecureOperation{
		Token:   "op-token",
		Type:    operationtype.ChangeTOTP,
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

func (s *RenderTOTPQRSuite) TestGateError() {
	errFetch := errors.New("fetch failed")

	s.fetcher.EXPECT().FetchOne(gomock.Any(), "op-token").Return(secureoperation.SecureOperation{}, errFetch)

	// рендерер не вызывается: операция не прошла проверку
	uc := security.NewRenderTOTPGeneratorQR(s.fetcher, mock.NewMocktotpQRRenderer(s.ctrl))

	_, err := uc.Execute(s.ctx, uuid.New(), "op-token")
	s.Require().ErrorIs(err, errFetch)
}

func (s *RenderTOTPQRSuite) TestRendererError() {
	userID := uuid.New()
	errRender := errors.New("render failed")

	s.fetcher.EXPECT().
		FetchOne(gomock.Any(), "op-token").
		Return(confirmedOp(userID, `{"email":"u@e","secret":"`+testTotpSecret+`"}`), nil)

	renderer := mock.NewMocktotpQRRenderer(s.ctrl)
	renderer.EXPECT().QRImage("u@e", testTotpSecret, gomock.Any(), gomock.Any()).Return(nil, errRender)

	_, err := security.NewRenderTOTPGeneratorQR(s.fetcher, renderer).Execute(s.ctx, userID, "op-token")
	s.Require().ErrorIs(err, errRender)
}

// TestImageEncodeError - изображение, которое нельзя закодировать в png, возвращается ошибкой.
func (s *RenderTOTPQRSuite) TestImageEncodeError() {
	userID := uuid.New()

	s.fetcher.EXPECT().
		FetchOne(gomock.Any(), "op-token").
		Return(confirmedOp(userID, `{"email":"u@e","secret":"`+testTotpSecret+`"}`), nil)

	renderer := mock.NewMocktotpQRRenderer(s.ctrl)
	renderer.EXPECT().QRImage(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(image.NewRGBA(image.Rectangle{}), nil)

	_, err := security.NewRenderTOTPGeneratorQR(s.fetcher, renderer).Execute(s.ctx, userID, "op-token")
	s.Require().Error(err)
}
