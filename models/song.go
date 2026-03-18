package models

import "gorm.io/gorm"

type Song struct {
	gorm.Model
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	FilePath string `json:"file_path" gorm:"unique"`
}