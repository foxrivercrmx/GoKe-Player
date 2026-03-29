package handlers

import (
	"bytes"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"goke-player/database"
	"goke-player/models"

	"github.com/gofiber/websocket/v2"
)

type Client struct {
	IsPlayer bool
}

var (
	Clients   = make(map[*websocket.Conn]*Client)
	Broadcast = make(chan map[string]interface{})
	Mutex     = sync.Mutex{}
	YTCookiesFile string
	YTJSRuntime   string
)

// Fungsi baru untuk ngurus perpindahan antrian
func ProcessNextSong() {
	database.DB.Model(&models.Queue{}).Where("status = ?", "playing").Update("status", "done")

	var nextQueue models.Queue
	result := database.DB.Preload("Song").Where("status = ?", "waiting").Order("created_at asc").First(&nextQueue)

	if result.Error == nil {
		database.DB.Model(&nextQueue).Update("status", "playing")

		var streamURL string

		// Cek apakah ini lagu lokal atau YouTube
		if strings.HasPrefix(nextQueue.Song.FilePath, "youtube://") {
			ytID := strings.TrimPrefix(nextQueue.Song.FilePath, "youtube://")
			
			// Ambil settingan dari database
			var config models.AppConfig
			database.DB.First(&config)
			
			// Racik format yt-dlp. Biar aman di STB, kita ambil single file (best)
			// Contoh: "best[height<=720][ext=mp4]/best"
			formatStr := "best[height<=" + config.Resolution + "]"
			if config.Codec == "h264" {
				formatStr += "[ext=mp4]"
			}
			formatStr += "/best" // Fallback kalau kriteria di atas nggak nemu

			log.Println("🔄 [YT EXTRACT] Mengekstrak URL untuk ID:", ytID, "dengan format:", formatStr)

			ytArgs := []string{}
			if YTCookiesFile != "" {
				ytArgs = append(ytArgs, "--cookies", YTCookiesFile)
			}
			if YTJSRuntime != "" {
				ytArgs = append(ytArgs, "--js-runtimes", YTJSRuntime)
			}
			ytArgs = append(ytArgs, "-g", "-f", formatStr, "https://www.youtube.com/watch?v="+ytID)
			
			// Panggil yt-dlp -g (get url)
			cmd := exec.Command("yt-dlp", ytArgs...)
			var out bytes.Buffer
			cmd.Stdout = &out
			
			err := cmd.Run()
			if err == nil {
				// Bersihkan \n dari output terminal
				streamURL = strings.TrimSpace(out.String()) 
				log.Println("✅ [YT EXTRACT] Berhasil dapat URL streaming!")
			} else {
				log.Println("❌ [YT EXTRACT] Gagal ekstrak URL!")
				// Skip ke lagu berikutnya kalau error
				go ProcessNextSong()
				return
			}
		} else if strings.HasPrefix(nextQueue.Song.FilePath, "alist://") {
			// ☁️ LOGIKA ALIST: Ambil settingan dari database
			var config models.AppConfig
			database.DB.First(&config)
			
			// Buang tanda "alist://" buat ngambil path aslinya
			alistPath := strings.TrimPrefix(nextQueue.Song.FilePath, "alist://")
			
			// AList pakai sisipan "/d" buat Direct Download / Streaming
			baseURL := strings.TrimRight(config.AListURL, "/")
			streamURL = baseURL + "/d" + alistPath
			
			log.Println("☁️ [ALIST] Memutar lagu dari:", streamURL)
		} else {
			// Kalau lagu lokal, langsung pake path biasa
			streamURL = "/video/" + filepath.Base(nextQueue.Song.FilePath)
		}

		// Kirim URL (Lokal atau YT) ke TV
		Broadcast <- map[string]interface{}{
			"action": "PLAY_SONG",
			"url":    streamURL,
			"title":  nextQueue.Song.Artist + " - " + nextQueue.Song.Title,
		}
	} else {
		Broadcast <- map[string]interface{}{
			"action": "STOP",
		}
	}

	Broadcast <- map[string]interface{}{
		"action": "QUEUE_UPDATED",
	}
}

func HandleWebSocket(c *websocket.Conn) {
	Mutex.Lock()
	isPlayer := c.Query("type") == "player"
	Clients[c] = &Client{IsPlayer: isPlayer}
	Mutex.Unlock()

	defer func() {
		Mutex.Lock()
		delete(Clients, c)
		Mutex.Unlock()
		c.Close()
	}()

	for {
		var msg map[string]interface{}
		if err := c.ReadJSON(&msg); err != nil {
			break
		}

		action, _ := msg["action"].(string)

		// Cegat perintah NEXT (dari HP) atau SONG_ENDED (dari STB)
		if action == "NEXT" || action == "SONG_ENDED" {
			ProcessNextSong()
		} else {
			// Kalau perintah lain kayak PAUSE/RESUME, langsung lempar aja
			Broadcast <- msg
		}
	}
}

func HandleMessages() {
	for {
		msg := <-Broadcast
		Mutex.Lock()
		for client := range Clients {
			err := client.WriteJSON(msg)
			if err != nil {
				client.Close()
				delete(Clients, client)
			}
		}
		Mutex.Unlock()
	}
}