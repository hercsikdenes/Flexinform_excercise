package model

import "time"

type Client struct {
	ID         string    `gorm:"column:id;primaryKey"`
	Name       string    `gorm:"column:name;not null"`
	PersonalID string    `gorm:"column:personal_id;not null;uniqueIndex"`
	IsActive   bool      `gorm:"column:is_active;not null"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoCreateTime;autoUpdateTime"`
}

func (Client) TableName() string { return "clients" }
