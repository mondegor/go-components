package entity

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
		Email     *DataEmail     `json:"email,omitempty"`
		SMS       *DataSMS       `json:"sms,omitempty"`
		Messenger *DataMessenger `json:"messenger,omitempty"`
	}

	// DataEmail - тип уведомления, которое отправляется в виде электронного письма на почтовый сервис.
	DataEmail struct {
		ContentType    string   `json:"contentType,omitempty"` // text/plain by default
		FromName       string   `json:"fromName,omitempty"`
		To             *string  `json:"to,omitempty"`
		ReplyTo        *string  `json:"replyTo,omitempty"`
		Subject        string   `json:"subject"`
		Preheader      string   `json:"preheader,omitempty"`
		Content        string   `json:"content"`
		ObserverEmails []string `json:"observerEmails,omitempty"`
		IsDisabled     bool     `json:"isDisabled,omitempty"`
	}

	// DataSMS - тип уведомления, которое отправляется в виде короткого сообщения на телефон.
	DataSMS struct {
		From       string `json:"from,omitempty"`
		Phone      string `json:"phone,omitempty"`
		Subject    string `json:"subject,omitempty"`
		Content    string `json:"content"`
		IsDisabled bool   `json:"isDisabled,omitempty"`
	}

	// DataMessenger - тип уведомления, которое отправляется в виде текста в Messenger сервис.
	DataMessenger struct {
		ChatID     string   `json:"chatId"`
		Tags       []string `json:"tags,omitempty"`
		Subject    string   `json:"subject,omitempty"`
		Content    string   `json:"content"`
		IsDisabled bool     `json:"isDisabled,omitempty"`
	}
)
