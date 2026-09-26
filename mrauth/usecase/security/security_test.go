package security_test

//go:generate mockgen -source=apply_totp.go -destination=mock/apply_totp.go -package=mock
//go:generate mockgen -source=apply_operation.go -destination=mock/apply_operation.go -package=mock
//go:generate mockgen -source=apply_password.go -destination=mock/apply_password.go -package=mock
//go:generate mockgen -source=totp_operation.go -destination=mock/totp_operation.go -package=mock
//go:generate mockgen -source=change_email.go -destination=mock/change_email.go -package=mock
//go:generate mockgen -destination=mock/mrauth.go -package=mock github.com/mondegor/go-components/mrauth User2FAConfirmActionCreator,OperationHandler,Notifier
