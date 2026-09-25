package model

import "time"

type CarOwnership struct {
	ID              int       `gorm:"column:id;primaryKey;autoIncrement:false"`
	ClientID        string    `gorm:"column:client_id;not null"`
	VehicleID       int       `gorm:"column:vehicle_id;not null"`
	ClientCarNumber int       `gorm:"column:client_car_number;not null"`
	IsActive        bool      `gorm:"column:is_active;not null"`
	CreatedAt       time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt       time.Time `gorm:"column:updated_at;autoCreateTime;autoUpdateTime"`
}

func (CarOwnership) TableName() string { return "car_ownerships" }
