package entity

import "github.com/mondegor/go-components/mrmailer/dto"

const (
	ModelNameMessage = "mrmailer.Message" // ModelNameMessage - название сущности
)

type (
	// Message - сообщение для получателя.
	Message struct {
		ID      uint64
		Channel string
		Data    dto.MessageData
	}
)
