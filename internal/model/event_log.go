package model

import "time"

type EventLog struct {
	ID          int        `gorm:"column:id;primaryKey;autoIncrement:false"`
	OwnershipID int        `gorm:"column:ownership_id;not null"`
	LogNumber   int        `gorm:"column:log_number;not null"`
	Event       string     `gorm:"column:event;not null"`
	EventTime   *time.Time `gorm:"column:event_time"`
	DocumentID  *string    `gorm:"column:document_id"`
	CreatedAt   time.Time  `gorm:"column:created_at;autoCreateTime"`
}

func (EventLog) TableName() string { return "event_log" }
