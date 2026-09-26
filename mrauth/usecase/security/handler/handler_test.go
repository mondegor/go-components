package handler_test

//go:generate mockgen -destination=mock/mrstorage.go -package=mock github.com/mondegor/go-core/mrstorage DBTxManager
//go:generate mockgen -destination=mock/mrauth.go -package=mock github.com/mondegor/go-components/mrauth Notifier
