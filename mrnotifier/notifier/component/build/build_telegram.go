package build

import (
	"github.com/mondegor/go-components/mrmailer/dto"
	templaterentity "github.com/mondegor/go-components/mrnotifier/template/entity"
)

func (co *NoticeBuilder) buildTelegram(_ map[string]string, _ *templaterentity.DataTelegram) ([]dto.Message, error) {
	// TODO: требует реализации
	return nil, nil
}
