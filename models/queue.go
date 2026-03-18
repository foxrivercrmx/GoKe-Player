package models

import "gorm.io/gorm"

type Queue struct {
	gorm.Model
	SongID uint   `json:"song_id"`
	Song   Song   `json:"song"`
	Status string `json:"status" gorm:"default:'waiting'"` // waiting, playing, done
}