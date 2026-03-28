package dto

import (
	"time"

	"github.com/mondegor/go-components/mrmailer/entity"
)

type (
	// Message - сообщение для получателя с возможностью указания времени,
	// когда нужно отправить сообщение.
	Message struct {
		Channel       string
		SendAfter     time.Time
		RetryAttempts int16
		Data          MessageData
	}

	// MessageData - собирательная структура, которая позволяет
	// хранить один из нескольких типов сообщений в виде json.
	MessageData = entity.MessageData

	// DataMail - тип сообщения, которое отправляется в виде электронного письма на почтовый сервис.
	DataMail = entity.DataMail

	// DataMessenger - тип сообщения, которое отправляется в виде текста в Messenger сервис.
	DataMessenger = entity.DataMessenger

	// DataSMS - тип сообщения, которое отправляется в виде короткого сообщения на телефон.
	DataSMS = entity.DataSMS
)
