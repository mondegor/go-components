package secureoperation_test

//go:generate mockgen -destination=mock/mrauth.go -package=mock github.com/mondegor/go-components/mrauth TokenGenerator,CodeGenerator,Notifier
//go:generate mockgen -source=opener.go -destination=mock/opener.go -package=mock
