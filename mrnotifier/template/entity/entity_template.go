package entity

import (
	"github.com/mondegor/go-components/mrmailer/dto"
)

const (
	ModelNameTemplate = "mrnotifier.template.Template" // ModelNameTemplate - название сущности
)

type (
	// Template - шаблон уведомления.
	Template struct {
		Lang  string
		Props TemplateData
		Vars  []Variable
	}

	// Variable - переменная шаблона уведомления.
	Variable struct {
		Name         string
		DefaultValue string
	}

	// TemplateData - собирательная структура, которая позволяет
	// хранить один из нескольких типов уведомлений в виде json.
	TemplateData struct {
		Email    *DataEmail    `json:"email,omitempty"`
		SMS      *DataSMS      `json:"sms,omitempty"`
		Telegram *DataTelegram `json:"telegram,omitempty"`
	}

	// DataEmail - тип уведомления, которое отправляется в виде электронного письма на почтовый сервис.
	DataEmail struct {
		ContentType    string             `json:"contentType,omitempty"` // text/plain by default
		From           *dto.EmailAddress  `json:"from,omitempty"`
		To             *dto.EmailAddress  `json:"to,omitempty"`
		ReplyTo        *dto.EmailAddress  `json:"replyTo,omitempty"`
		Subject        string             `json:"subject"`
		Preheader      string             `json:"preheader,omitempty"`
		Content        string             `json:"content"`
		ObserverEmails []dto.EmailAddress `json:"observerEmails,omitempty"`
		IsDisabled     bool               `json:"isDisabled,omitempty"`
	}

	// DataSMS - тип уведомления, которое отправляется в виде короткого сообщения на телефон.
	DataSMS struct {
		From       string `json:"from,omitempty"`
		Phone      string `json:"phone,omitempty"`
		Content    string `json:"content"`
		IsDisabled bool   `json:"isDisabled,omitempty"`
	}

	// DataTelegram - тип уведомления, которое отправляется в виде текста в Telegram сервис.
	DataTelegram struct {
		ChatID     string   `json:"chatId"`
		Tags       []string `json:"tags"`
		Content    string   `json:"content"`
		IsDisabled bool     `json:"isDisabled,omitempty"`
	}
)
