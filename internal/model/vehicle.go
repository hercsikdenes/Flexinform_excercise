package model

import "time"

type Vehicle struct {
	ID            int       `gorm:"column:id;primaryKey;autoIncrement:false"`
	Type          string    `gorm:"column:type;not null"`
	RegisteredAt  time.Time `gorm:"column:registered_at;not null"`
	OwnBrand      bool      `gorm:"column:own_brand;not null"`
	AccidentCount int       `gorm:"column:accident_count;not null"`
	IsActive      bool      `gorm:"column:is_active;not null"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time `gorm:"column:updated_at;autoCreateTime;autoUpdateTime"`
}

func (Vehicle) TableName() string { return "vehicles" }
