package dto

import (
	"time"
)

type (
	// Message - сообщение для получателя с возможностью указания времени,
	// когда нужно отправить сообщение.
	Message struct {
		Channel       string
		SendAfter     time.Time
		RetryAttempts uint32
		Data          MessageData
	}

	// MessageData - собирательная структура, которая позволяет
	// хранить один из нескольких типов сообщений в виде json.
	MessageData struct {
		Header   map[string]string `json:"header,omitempty"`
		Email    *DataEmail        `json:"email,omitempty"`
		SMS      *DataSMS          `json:"sms,omitempty"`
		Telegram *DataTelegram     `json:"telegram,omitempty"`
	}

	// DataEmail - тип сообщения, которое отправляется в виде электронного письма на почтовый сервис.
	DataEmail struct {
		ContentType string        `json:"contentType"`
		From        EmailAddress  `json:"from"`
		To          EmailAddress  `json:"to"`
		ReplyTo     *EmailAddress `json:"replyTo,omitempty"`
		Subject     string        `json:"subject"`
		Content     string        `json:"content"`
	}

	// DataSMS - тип сообщения, которое отправляется в виде короткого сообщения на телефон.
	DataSMS struct {
		From    string `json:"from"`
		Phone   string `json:"phone"`
		Content string `json:"content"`
	}

	// DataTelegram - тип сообщения, которое отправляется в виде текста в Telegram сервис.
	DataTelegram struct {
		ChatID  string   `json:"chatId"`
		Tags    []string `json:"tags"`
		Content string   `json:"content"`
	}
)
