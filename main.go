package main

import (
	"embed"
	"flag"
	"log"
	"net/http"
	"os"

	"goke-player/database"
	"goke-player/handlers"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/websocket/v2"
)

//go:embed public/*
var embedDirStatic embed.FS

func main() {
	// 1. Setup Flag (Config dari command line)
	// Format: flag.String("nama_flag", "default_value", "deskripsi")
	storageDir := flag.String("storage", "./storage", "Path ke folder penyimpanan video karaoke")
	dbPath := flag.String("db", "karaoke.db", "Path ke file database SQLite")
	port := flag.String("port", "3000", "Port server jalan")
	flag.Parse() // Wajib dipanggil buat ngebaca inputannya

	// 2. Inisialisasi Database (Kita perlu update InitDB nerima parameter path)
	database.InitDB(*dbPath)

	// Cek dan buat folder storage kalau belum ada sesuai path dari flag
	if _, err := os.Stat(*storageDir); os.IsNotExist(err) {
		log.Printf("Folder storage %s tidak ditemukan, membuat baru...", *storageDir)
		os.MkdirAll(*storageDir, 0755) // Pake MkdirAll biar aman kalau path-nya bersarang
	}

	handlers.StoragePath = *storageDir

	// 3. Setup Fiber App
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true, // Biar log-nya bersih di journalctl systemd
	})
	app.Use(logger.New())
	app.Use(cors.New())

	// Middleware WebSocket
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	// 5. API Routes
	api := app.Group("/api")
	api.Get("/songs", handlers.GetSongs)
	api.Post("/queue", handlers.AddToQueue)
	api.Get("/queue", handlers.GetQueue)
	api.Post("/scan", handlers.ScanLibrary)
	api.Get("/config", handlers.GetConfig)
	api.Post("/config", handlers.UpdateConfig)
	api.Get("/youtube/search", handlers.SearchYouTube)
	api.Post("/youtube/queue", handlers.QueueYouTube)
	api.Post("/alist/scan", handlers.ScanAList)
	api.Delete("/queue/clear", handlers.ClearQueue)
	api.Delete("/songs/clear", handlers.ClearSongs)

	// 6. WebSocket
	app.Get("/ws", websocket.New(handlers.HandleWebSocket))
	go handlers.HandleMessages()

	// 4. Static Files (Gunakan variabel dari flag)
	app.Static("/video", *storageDir)
	// Static files untuk UI (Player & Remote) sekarang dibaca dari dalam binary!
	app.Use("/", filesystem.New(filesystem.Config{
		Root:       http.FS(embedDirStatic),
		PathPrefix: "public",
		Browse:     false,
	}))

	// 7. Jalankan Server
	log.Printf("🎤 Server Karaoke jalan di port :%s", *port)
	log.Printf("📂 Folder Storage: %s", *storageDir)
	log.Printf("🗄️ Database: %s", *dbPath)
	
	log.Fatal(app.Listen(":" + *port))
}