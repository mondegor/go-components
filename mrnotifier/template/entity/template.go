package entity

import (
	"github.com/mondegor/go-sysmess/mrstatus/itemstatus"
)

type (
	// Template - шаблон уведомления.
	Template struct {
		Name   string
		Lang   string
		Props  TemplateData
		Vars   []string
		Status itemstatus.Enum
	}

	// Variable - переменная шаблона уведомления.
	Variable struct {
		Name         string
		DefaultValue string
	}

	// TemplateData - собирательная структура, которая позволяет
	// хранить один из нескольких типов уведомлений в виде json.
	TemplateData struct {
		Mail      *DataMail      `json:"mail,omitempty"`
		Messenger *DataMessenger `json:"messenger,omitempty"`
		SMS       *DataSMS       `json:"sms,omitempty"`
	}

	// DataMail - тип уведомления, которое отправляется в виде электронного письма на почтовый сервис.
	DataMail struct {
		ContentType    string   `json:"content_type,omitempty"` // text/plain by default
		FromName       string   `json:"from_name,omitempty"`
		To             *string  `json:"to,omitempty"`
		ReplyTo        *string  `json:"reply_to,omitempty"`
		Subject        string   `json:"subject"`
		Preheader      string   `json:"preheader,omitempty"`
		Content        string   `json:"content"`
		ObserverEmails []string `json:"observer_emails,omitempty"`
		IsDisabled     bool     `json:"is_disabled,omitempty"`
	}

	// DataMessenger - тип уведомления, которое отправляется в виде текста в Messenger сервис.
	DataMessenger struct {
		ChatID     string   `json:"chat_id"`
		Tags       []string `json:"tags,omitempty"`
		Subject    string   `json:"subject,omitempty"`
		Content    string   `json:"content"`
		IsDisabled bool     `json:"is_disabled,omitempty"`
	}

	// DataSMS - тип уведомления, которое отправляется в виде короткого сообщения на телефон.
	DataSMS struct {
		From       string `json:"from,omitempty"`
		Phone      string `json:"phone,omitempty"`
		Subject    string `json:"subject,omitempty"`
		Content    string `json:"content"`
		IsDisabled bool   `json:"is_disabled,omitempty"`
	}
)
