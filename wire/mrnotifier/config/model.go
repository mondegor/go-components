package config

import (
	"time"

	workercfg "github.com/mondegor/go-sysmess/mrworker/config"
)

type (
	// TaskSchedule - настройки задач модуля Notifier, запускаемых по расписанию.
	TaskSchedule struct {
		// Caption            string           `yaml:"caption"`
		NoticeProcessor      workercfg.MessageProcessor `yaml:"notice_processor"`
		ChangeFromToRetry    workercfg.SchedulerTask    `yaml:"change_from_to_retry"`
		CleanQueue           workercfg.SchedulerTask    `yaml:"clean_queue"`
		SendRetryAttempts    uint8                      `yaml:"send_retry_attempts"`
		ChangeQueueBatchSize uint32                     `yaml:"change_queue_batch_size"`
		ChangeRetryTimeout   time.Duration              `yaml:"change_retry_timeout"`
		ChangeRetryDelayed   time.Duration              `yaml:"change_retry_delayed"`
		CleanQueueBatchSize  uint32                     `yaml:"clean_queue_batch_size"`
	}
)
