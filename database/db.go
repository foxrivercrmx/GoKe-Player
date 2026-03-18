package database

import (
	"fmt"
	"log"

	"goke-player/models" // Sesuaikan kalau nama module-nya beda
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB(dbPath string) {
	var err error
	// Pake glebarez, mantap tanpa CGO!
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		log.Fatal("Gagal connect database:", err)
	}

	// Auto Migrate table dari models
	err = DB.AutoMigrate(&models.Song{}, &models.Queue{}, &models.AppConfig{})
	if err != nil {
		log.Fatal("Gagal migrasi database:", err)
	}

	// Bikin default config kalau belum ada
	var count int64
	DB.Model(&models.AppConfig{}).Count(&count)
	if count == 0 {
		defaultConfig := models.AppConfig{
			Resolution: "720",
			Codec:      "h264",
		}
		DB.Create(&defaultConfig)
		fmt.Println("⚙️ Konfigurasi default berhasil dibuat!")
	}
	
	fmt.Println("🚀 Database SQLite (Glebarez) berhasil disiapkan!")
}