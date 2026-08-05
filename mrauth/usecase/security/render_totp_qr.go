package security

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"io"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	modelmedia "github.com/mondegor/go-core/mrmodel/media"
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

	totpQRRenderer interface {
		QRImage(accountName, secret string, width, height int) (image.Image, error)
	}
)

// NewRenderTOTPGeneratorQR - создаёт объект RenderTOTPGeneratorQR.
func NewRenderTOTPGeneratorQR(storageOperation operationFetcher, totpRenderer totpQRRenderer) *RenderTOTPGeneratorQR {
	return &RenderTOTPGeneratorQR{
		storageOperation: storageOperation,
		totpRenderer:     totpRenderer,
		errorWrapper:     errors.NewServiceOperationFailedWrapper(),
	}
}

// Execute - проверяет подтверждённую операцию и возвращает QR-код TOTP-генератора,
// построенный из secret, сохранённого в payload операции.
// QR рендерится при каждом запросе и не кэшируется намеренно (показ однократный и операция короткоживущая).
// Также имеется альтернатива GetTOTPGeneratorSecret.
func (uc *RenderTOTPGeneratorQR) Execute(ctx context.Context, userID uuid.UUID, operationToken string) (modelmedia.Image, error) {
	payload, err := fetchConfirmedTOTPPayload(ctx, uc.storageOperation, uc.errorWrapper, userID, operationToken)
	if err != nil {
		return modelmedia.Image{}, err
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
