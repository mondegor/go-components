package security

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"io"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	modelmedia "github.com/mondegor/go-core/mrmodel/media"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
)

const (
	totpQRSize = 384
)

type (
	// RenderTOTPGeneratorQR - возвращает QR-код TOTP-генератора, secret которого
	// уже сохранён в payload подтверждённой операции (привязка - на verify-шаге).
	RenderTOTPGeneratorQR struct {
		storageOperation operationFetcher
		totpRenderer     totpQRRenderer
		errorWrapper     errors.Wrapper
	}

	operationFetcher interface {
		FetchOne(ctx context.Context, token string) (secureoperation.SecureOperation, error)
	}

	totpQRRenderer interface {
		QRImage(accountName, secret string, width, height int) (image.Image, error)
	}
)

// NewRenderTOTPGeneratorQR - создаёт объект RenderTOTPGeneratorQR.
func NewRenderTOTPGeneratorQR(storageOperation operationFetcher, totpRenderer totpQRRenderer) *RenderTOTPGeneratorQR {
	return &RenderTOTPGeneratorQR{
		storageOperation: storageOperation,
		totpRenderer:     totpRenderer,
		errorWrapper:     errors.NewServiceRecordNotFoundWrapper(),
	}
}

// Execute - проверяет подтверждённую операцию и возвращает QR-код TOTP-генератора,
// построенный из secret, сохранённого в payload операции.
// QR рендерится при каждом запросе намеренно (показ при enrollment однократный, операция
// короткоживущая и owner-scoped) и не кэшируется; внешний rate-limit - ответственность хоста.
func (uc *RenderTOTPGeneratorQR) Execute(ctx context.Context, userID uuid.UUID, operationToken string) (modelmedia.Image, error) {
	if userID == uuid.Nil {
		return modelmedia.Image{}, errors.ErrInternalIncorrectInputData.WithDetails("userId is empty")
	}

	if operationToken == "" {
		return modelmedia.Image{}, errors.ErrRecordNotFound // TODO: возможно, стоит возвращать ошибку о некорректном параметре
	}

	op, err := uc.storageOperation.FetchOne(ctx, operationToken)
	if err != nil {
		return modelmedia.Image{}, uc.errorWrapper.Wrap(err)
	}

	if userID != op.UserID {
		return modelmedia.Image{}, errors.ErrAccessForbidden
	}

	// TODO: проверить, что пользователь не заблокирован

	if op.Name != unit.NameConfirmChangeTOTP {
		return modelmedia.Image{}, errors.ErrAccessForbidden
	}

	if !op.Is(operationstatus.Confirmed) {
		return modelmedia.Image{}, errors.New("operation is not confirmed")
	}

	var payload dto.ChangeTotpOperation

	if err = json.Unmarshal(op.Payload, &payload); err != nil {
		return modelmedia.Image{}, uc.errorWrapper.Wrap(err)
	}

	if payload.Secret == "" {
		return modelmedia.Image{}, errors.New("operation has no staged secret")
	}

	img, err := uc.totpRenderer.QRImage(payload.Email, payload.Secret, totpQRSize, totpQRSize)
	if err != nil {
		return modelmedia.Image{}, uc.errorWrapper.Wrap(err)
	}

	return totpQRImage(img)
}

// totpQRImage - кодирует изображение QR-кода TOTP-генератора в png.
func totpQRImage(img image.Image) (modelmedia.Image, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return modelmedia.Image{}, err
	}

	return modelmedia.Image{
		ImageInfo: modelmedia.ImageInfo{
			ContentType: "image/png",
			Width:       totpQRSize,
			Height:      totpQRSize,
			Size:        int64(buf.Len()),
		},
		Body: io.NopCloser(bytes.NewReader(buf.Bytes())),
	}, nil
}
