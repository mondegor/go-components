package build

import (
	"github.com/mondegor/go-components/mrmailer/dto"
	templaterentity "github.com/mondegor/go-components/mrnotifier/template/entity"
)

func (co *NoticeBuilder) buildSMS(_ map[string]string, _ *templaterentity.DataSMS) ([]dto.Message, error) {
	// TODO: требует реализации
	return nil, nil
}
