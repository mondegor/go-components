package security

import (
	"bytes"
	"context"
	"image/jpeg"
	"io"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrmodel"
	"github.com/mondegor/go-sysmess/util/conv"
	"github.com/pquerna/otp/totp"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/auth2fatype"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrnotifier"
)

type (
	// ApplyTOTPGenerator - comment struct.
	ApplyTOTPGenerator struct {
		txManager        mrstorage.DBTxManager
		storage          user2faCreator
		storageOperation operationFetcher
		notifierAPI      mrnotifier.NoteProducer
		errorWrapper     errors.Wrapper
		issuer           string
	}

	user2faCreator interface {
		InsertOrUpdate(ctx context.Context, row entity.Auth2fa) error
	}
)

// NewApplyTOTPGenerator - создаёт объект ApplyTOTPGenerator.
func NewApplyTOTPGenerator(
	txManager mrstorage.DBTxManager,
	storage user2faCreator,
	storageOperation operationFetcher,
	notifierAPI mrnotifier.NoteProducer,
	issuer string,
) *ApplyTOTPGenerator {
	return &ApplyTOTPGenerator{
		txManager:        txManager,
		storage:          storage,
		storageOperation: storageOperation,
		notifierAPI:      notifierAPI,
		errorWrapper:     errors.NewServiceRecordNotFoundWrapper(),
		issuer:           issuer,
	}
}

// apply_change.go // to service: validate + store
// change_totp.go отдельный метод

// Execute - comments method.
func (uc *ApplyTOTPGenerator) Execute(ctx context.Context, userID uuid.UUID, operationToken string) (totpQRcode mrmodel.Image, err error) {
	if userID == uuid.Nil {
		return mrmodel.Image{}, errors.ErrInternalIncorrectInputData.WithDetails("userId is empty")
	}

	if operationToken == "" {
		return mrmodel.Image{}, errors.ErrRecordNotFound // TODO: ?может ошибку, что параметр некорректен выдавать?
	}

	op, err := uc.storageOperation.FetchOne(ctx, operationToken)
	if err != nil {
		return mrmodel.Image{}, uc.errorWrapper.Wrap(err)
	}

	if userID != op.UserID {
		return mrmodel.Image{}, errors.ErrAccessForbidden
	}

	// TODO: проверить, что пользователь не заблокирован !!!!!!!

	if !op.Is(operationstatus.Confirmed) {
		return mrmodel.Image{}, errors.New("operation id not confirmed")
	}

	userEmail := string(op.Payload)

	secret, err := totp.Generate(totp.GenerateOpts{
		Issuer:      uc.issuer,
		AccountName: userEmail,
		SecretSize:  64,
	})
	if err != nil {
		return mrmodel.Image{}, uc.errorWrapper.Wrap(err)
	}

	err = uc.txManager.Do(ctx, func(ctx context.Context) error {
		if err = uc.storageOperation.Delete(ctx, op.Token); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		// TODO: Add Operation log:op! ????

		err = uc.storage.InsertOrUpdate(
			ctx,
			entity.Auth2fa{
				UserID: op.UserID,
				Type:   auth2fatype.TOTP,
				Secret: secret.Secret(),
			},
		)
		if err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		if err = uc.notifierAPI.Send(ctx, "user.totp.changed", conv.Group{"to": userEmail}); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		return nil
	})
	if err != nil {
		return mrmodel.Image{}, uc.errorWrapper.Wrap(err)
	}

	img, err := secret.Image(384, 384)
	if err != nil {
		return mrmodel.Image{}, uc.errorWrapper.Wrap(err)
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		return mrmodel.Image{}, uc.errorWrapper.Wrap(err)
	}

	if buf.Len() < 0 {
		return mrmodel.Image{}, uc.errorWrapper.Wrap(errors.New("buffer is negative"))
	}

	// вынести отдельно
	return mrmodel.Image{
		ImageInfo: mrmodel.ImageInfo{
			ContentType: "image/jpeg",
			// OriginalName: "qr",
			// Realm:         "rq",
			Width:  384,
			Height: 384,
			Size:   int64(buf.Len()),
		},
		Body: io.NopCloser(bytes.NewReader(buf.Bytes())),
	}, nil
}
