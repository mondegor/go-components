package dto

import "github.com/mondegor/go-components/mrnotifier/template/entity"

type (
	// Template - шаблон уведомления.
	Template struct {
		Lang  string
		Props entity.TemplateData
		Vars  []entity.Variable
	}
)
