package models

import "gorm.io/gorm"

type AppConfig struct {
	gorm.Model
	Resolution string `json:"resolution" gorm:"default:'720'"` // Pilihan: 480, 720, 1080, best
	Codec      string `json:"codec" gorm:"default:'h264'"`      // Pilihan: h264, hevc, vp9, best

	AListURL   string `json:"alist_url" gorm:"default:'http://127.0.0.1:5244'"`
	AListPath  string `json:"alist_path" gorm:"default:'/'"`
}