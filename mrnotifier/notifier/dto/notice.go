package dto

import (
	"time"
)

type (
	// Notice - уведомление для получателя с возможностью указания времени,
	// когда нужно отправить уведомление.
	Notice struct {
		Channel       string
		SendAfter     time.Time
		RetryAttempts uint32
		Data          NoticeData
	}

	// NoticeData - собирательная структура, которая позволяет
	// хранить один из нескольких типов уведомлений в виде json.
	NoticeData struct {
		Header    map[string]string
		Mail      *DataMail
		Messenger *DataMessenger
		SMS       *DataSMS
	}

	// DataMail - тип уведомления, которое отправляется в виде электронного письма на почтовый сервис.
	DataMail struct {
		ContentType string
		From        string // name | email | name <email>
		To          string
		ReplyTo     string
		Subject     string
		Content     string
	}

	// DataMessenger - тип уведомления, которое отправляется в виде текста в Messenger сервис.
	DataMessenger struct {
		From    string
		ChatID  string
		Content string
	}

	// DataSMS - тип уведомления, которое отправляется в виде короткого сообщения на телефон.
	DataSMS struct {
		From    string
		Phone   string
		Content string
	}
)
