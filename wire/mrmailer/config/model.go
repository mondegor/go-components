package config

import (
	"time"

	processcfg "github.com/mondegor/go-core/mrprocess/config"
)

type (
	// TaskSchedule - настройки задач модуля Mailer, запускаемых по расписанию.
	TaskSchedule struct {
		// Caption             string           `yaml:"caption"`
		MessageProcessor     processcfg.MessageProcessor `yaml:"message_processor"`
		ChangeFromToRetry    processcfg.SchedulerTask    `yaml:"change_from_to_retry"`
		CleanQueue           processcfg.SchedulerTask    `yaml:"clean_queue"`
		SendRetryAttempts    uint8                       `yaml:"send_retry_attempts"`
		SendDelayCorrection  time.Duration               `yaml:"send_delay_correction"`
		ChangeQueueBatchSize uint32                      `yaml:"change_queue_batch_size"`
		ChangeRetryTimeout   time.Duration               `yaml:"change_retry_timeout"`
		ChangeRetryDelayed   time.Duration               `yaml:"change_retry_delayed"`
		CleanQueueBatchSize  uint32                      `yaml:"clean_queue_batch_size"`
	}
)
