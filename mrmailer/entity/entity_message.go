package entity

import "github.com/mondegor/go-components/mrmailer/dto"

const (
	// ModelNameMessage - название сущности.
	ModelNameMessage = "mrmailer.Message"
)

type (
	// Message - сообщение для получателя.
	Message struct {
		ID      uint64
		Channel string
		Data    dto.MessageData
	}
)
