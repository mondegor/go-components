package security

import (
	"bytes"
	"context"
	"errors"
	"image/jpeg"
	"io"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrargs"
	"github.com/mondegor/go-sysmess/mrerr"
	"github.com/mondegor/go-sysmess/mrerr/mr"
	"github.com/mondegor/go-sysmess/mrtype"
	"github.com/pquerna/otp/totp"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum"
	"github.com/mondegor/go-components/mrnotifier"
)

type (
	// ApplyTOTPGenerator - компонент для извлечения настроек, которые хранятся в хранилище данных.
	ApplyTOTPGenerator struct {
		txManager        mrstorage.DBTxManager
		storage          mrauth.User2faStorage
		storageOperation mrauth.SecureOperationStorage
		notifierAPI      mrnotifier.NoticeProducer
		errorWrapper     mrerr.UseCaseErrorWrapper
		issuer           string
	}
)

// NewApplyTOTPGenerator - создаёт объект ApplyTOTPGenerator.
func NewApplyTOTPGenerator(
	txManager mrstorage.DBTxManager,
	storage mrauth.User2faStorage,
	storageOperation mrauth.SecureOperationStorage,
	notifierAPI mrnotifier.NoticeProducer,
	errorWrapper mrerr.UseCaseErrorWrapper,
	issuer string,
) *ApplyTOTPGenerator {
	return &ApplyTOTPGenerator{
		txManager:        txManager,
		storage:          storage,
		storageOperation: storageOperation,
		notifierAPI:      notifierAPI,
		errorWrapper:     mrerr.NewUseCaseErrorWrapper(errorWrapper, entity.ModelNameSecureOperation),
		issuer:           issuer,
	}
}

// apply_change.go // to service: validate + store
// change_totp.go отдельный метод

// Execute - comments method.
func (uc *ApplyTOTPGenerator) Execute(ctx context.Context, userID uuid.UUID, operationToken string) (totpQRcode mrtype.Image, err error) {
	if operationToken == "" {
		return mrtype.Image{}, mr.ErrUseCaseEntityNotFound.New() // TODO: ?может ошибку, что параметр некорректен выдавать?
	}

	op, err := uc.storageOperation.FetchOne(ctx, operationToken)
	if err != nil {
		return mrtype.Image{}, uc.errorWrapper.WrapErrorNotFoundOrFailed(err)
	}

	if userID == uuid.Nil || userID != op.UserID {
		return mrtype.Image{}, mr.ErrUseCaseAccessForbidden.New()
	}

	// TODO: проверить, что пользователь не заблокирован !!!!!!!

	if op.Status != enum.OperationStatusConfirmed {
		return mrtype.Image{}, errors.New("operation id not confirmed")
	}

	userEmail := string(op.Payload)

	secret, err := totp.Generate(totp.GenerateOpts{
		Issuer:      uc.issuer,
		AccountName: userEmail,
		SecretSize:  64,
	})
	if err != nil {
		return mrtype.Image{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	err = uc.txManager.Do(ctx, func(ctx context.Context) error {
		if err = uc.storageOperation.Delete(ctx, op.Token); err != nil {
			return uc.errorWrapper.WrapErrorFailed(err)
		}

		// TODO: Add Operation log:op! ????

		err = uc.storage.InsertOrUpdate(
			ctx,
			entity.Auth2fa{
				UserID: op.UserID,
				Type:   enum.Auth2faTypeTOTP,
				Secret: secret.Secret(),
			},
		)
		if err != nil {
			return uc.errorWrapper.WrapErrorFailed(err)
		}

		if err = uc.notifierAPI.SendNotice(ctx, "user.totp.changed", mrargs.Group{"to": userEmail}); err != nil {
			return uc.errorWrapper.WrapErrorFailed(err)
		}

		return nil
	})
	if err != nil {
		return mrtype.Image{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	img, err := secret.Image(384, 384)
	if err != nil {
		return mrtype.Image{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		return mrtype.Image{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	if buf.Len() < 0 {
		return mrtype.Image{}, uc.errorWrapper.WrapErrorFailed(errors.New("buffer is negative"))
	}

	// вынести отдельно
	return mrtype.Image{
		ImageInfo: mrtype.ImageInfo{
			ContentType: "image/jpeg",
			// OriginalName: "qr",
			// Realm:         "rq",
			Width:  384,
			Height: 384,
			Size:   uint64(buf.Len()), //nolint:gosec
		},
		Body: io.NopCloser(bytes.NewReader(buf.Bytes())),
	}, nil
}
