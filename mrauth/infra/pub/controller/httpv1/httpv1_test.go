package httpv1_test

//go:generate mockgen -source=auth.go -destination=mock/auth.go -package=mock
//go:generate mockgen -destination=mock/validate.go -package=mock github.com/mondegor/go-components/mrauth/validate RequestParser
//go:generate mockgen -destination=mock/mrcore.go -package=mock github.com/mondegor/go-webcore/mrcore Localizer
//go:generate mockgen -destination=mock/mrserver.go -package=mock github.com/mondegor/go-webcore/mrserver ResponseSender
