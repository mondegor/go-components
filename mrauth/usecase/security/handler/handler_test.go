package handler_test

//go:generate mockgen -destination=mock/mrstorage.go -package=mock github.com/mondegor/go-core/mrstorage DBTxManager
//go:generate mockgen -destination=mock/mrnotifier.go -package=mock github.com/mondegor/go-components/mrnotifier NoteProducer
