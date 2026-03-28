package entity

const (
	// ModelNameMessage - название сущности.
	ModelNameMessage = "mrmailer.Message"
)

type (
	// Message - сообщение для получателя.
	Message struct {
		ID      uint64
		Channel string
		Data    MessageData
	}

	// MessageData - структура позволяющая хранить информацию
	// одного из нескольких типов уведомлений в виде json.
	MessageData struct {
		Header    map[string]string `json:"header,omitempty"`
		Mail      *DataMail         `json:"mail,omitempty"`
		Messenger *DataMessenger    `json:"messenger,omitempty"`
		SMS       *DataSMS          `json:"sms,omitempty"`
	}

	// DataMail - тип сообщения, которое отправляется в виде электронного письма на почтовый сервис.
	DataMail struct {
		ContentType string `json:"content_type"`
		From        string `json:"from"` // name | email | name <email>
		To          string `json:"to"`
		ReplyTo     string `json:"reply_to,omitempty"`
		Subject     string `json:"subject"`
		Content     string `json:"content"`
	}

	// DataMessenger - тип сообщения, которое отправляется в виде текста в Messenger сервис.
	DataMessenger struct {
		From    string `json:"from"`
		ChatID  string `json:"chat_id"`
		Content string `json:"content"`
	}

	// DataSMS - тип сообщения, которое отправляется в виде короткого сообщения на телефон.
	DataSMS struct {
		From    string `json:"from"`
		Phone   string `json:"phone"`
		Content string `json:"content"`
	}
)

// MessageID - comments method.
func (e Message) MessageID() uint64 {
	return e.ID
}
