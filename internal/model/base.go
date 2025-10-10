package model

import (
	"time"

	"gorm.io/gorm"
)

type AutoTimeModel struct {
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

// BeforeCreate ensures timestamps are set before creating a record
func (b *AutoTimeModel) BeforeCreate(_ *gorm.DB) error {
	now := time.Now().UTC()

	if b.CreatedAt.IsZero() {
		b.CreatedAt = now
	}

	b.UpdatedAt = now

	return nil
}

// BeforeUpdate ensures UpdatedAt is set before updating a record
func (b *AutoTimeModel) BeforeUpdate(_ *gorm.DB) error {
	b.UpdatedAt = time.Now().UTC()
	return nil
}
