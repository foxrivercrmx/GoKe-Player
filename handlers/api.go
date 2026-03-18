package handlers

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"goke-player/database"
	"goke-player/models"

	"github.com/gofiber/fiber/v2"
)

var StoragePath string

// Ambil list lagu (bisa dicari juga)
// Pastikan "strconv" udah ter-import di bagian atas ya Kang
func GetSongs(c *fiber.Ctx) error {
	query := c.Query("q")
	
	// Tangkap parameter halaman (page), default ke 1 kalau kosong
	page, _ := strconv.Atoi(c.Query("page", "1"))
	if page < 1 {
		page = 1
	}
	
	limit := 20 // Kita batesin 20 lagu per tarikan biar enteng
	offset := (page - 1) * limit

	var songs []models.Song
	var total int64

	db := database.DB.Model(&models.Song{})

	// Kalau ada pencarian
	if query != "" {
		db = db.Where("title LIKE ? OR artist LIKE ?", "%"+query+"%", "%"+query+"%")
	}

	// Hitung total lagu dulu buat info ke HP (apakah masih ada halaman berikutnya)
	db.Count(&total)

	// Tarik data dengan limit dan offset
	db.Offset(offset).Limit(limit).Order("artist asc, title asc").Find(&songs)

	// Hitung total halaman
	totalPages := int((total + int64(limit) - 1) / int64(limit))

	// Kirim response dalam bentuk object terstruktur
	return c.JSON(fiber.Map{
		"data":        songs,
		"total":       total,
		"page":        page,
		"limit":       limit,
		"total_pages": totalPages,
	})
}

// Masukin lagu ke antrian
func AddToQueue(c *fiber.Ctx) error {
	type Request struct {
		SongID uint `json:"song_id"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	// Cek apakah ada lagu yang lagi jalan
	var playingCount int64
	database.DB.Model(&models.Queue{}).Where("status = ?", "playing").Count(&playingCount)

	// WAJIB jadikan "waiting" dulu, biarin ProcessNextSong yang mikir
	queue := models.Queue{SongID: req.SongID, Status: "waiting"}
	database.DB.Create(&queue)
	database.DB.Preload("Song").First(&queue, queue.ID)

	// Tetap kasih tau HP kalau antrian update
	Broadcast <- map[string]interface{}{
		"action": "QUEUE_UPDATED",
	}

	// Kalau lagi nggak ada yang muter, panggil ProcessNextSong buat ngeksekusi
	if playingCount == 0 {
		go ProcessNextSong()
	}

	return c.JSON(fiber.Map{"message": "Masuk antrian!", "queue": queue})
}

// Ambil list antrian yang lagi jalan atau nunggu
func GetQueue(c *fiber.Ctx) error {
	var queues []models.Queue
	database.DB.Preload("Song").Where("status IN ?", []string{"waiting", "playing"}).Order("created_at asc").Find(&queues)
	return c.JSON(queues)
}

// ScanLibrary buat nyari file .mp4 baru di folder storage
func ScanLibrary(c *fiber.Ctx) error {
	log.Println("=========================================")
	log.Println("🔍 MEMULAI PROSES SCAN LAGU")
	log.Println("📁 Target Folder:", StoragePath)
	
	// 1. Cek dulu foldernya beneran ada dan bisa diakses nggak
	if _, err := os.Stat(StoragePath); os.IsNotExist(err) {
		log.Println("❌ ERROR: Folder storage tidak ditemukan atau path salah!")
		return c.Status(400).JSON(fiber.Map{"error": "Folder storage tidak ditemukan: " + StoragePath})
	}

	added := 0

	err := filepath.Walk(StoragePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Println("⚠️ WARNING: Gagal akses path:", path, "| Error:", err)
			return nil // Sengaja return nil biar nggak langsung berhenti kalau ada 1 file nyangkut
		}
		
		if !info.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))
			if ext == ".mp4" || ext == ".mkv" || ext == ".webm" {
				log.Println("📄 Nemu video:", path)
				
				// 2. Cek database
				var count int64
				result := database.DB.Model(&models.Song{}).Where("file_path = ?", path).Count(&count)
				
				if result.Error != nil {
					log.Println("❌ ERROR DB saat ngecek file:", result.Error)
					return nil
				}
				
				if count == 0 {
					// 3. Ekstrak nama artis & judul
					filename := strings.TrimSuffix(filepath.Base(path), ext)
					parts := strings.SplitN(filename, "-", 2)
					
					artist := "Unknown"
					title := filename
					
					if len(parts) == 2 {
						artist = strings.TrimSpace(parts[0])
						title = strings.TrimSpace(parts[1])
					}
					
					newSong := models.Song{
						Title:    title,
						Artist:   artist,
						FilePath: path,
					}
					
					// 4. Simpan ke database
					if errDB := database.DB.Create(&newSong).Error; errDB != nil {
						log.Println("❌ ERROR saat nyimpen ke DB:", errDB)
					} else {
						log.Printf("✅ BERHASIL ditambah: %s - %s\n", artist, title)
						added++
					}
				} else {
					log.Println("⏩ Skip: Udah ada di database ->", path)
				}
			}
		}
		return nil
	})

	if err != nil {
		log.Println("❌ PROSES SCAN BERHENTI KARENA ERROR:", err)
		return c.Status(500).JSON(fiber.Map{"error": "Gagal membaca folder storage: " + err.Error()})
	}

	log.Println("🎉 SCAN SELESAI! Total lagu baru:", added)
	log.Println("=========================================")

	return c.JSON(fiber.Map{
		"message": "Scan selesai!",
		"added":   added,
	})
}

// GetConfig buat ngambil pengaturan saat ini
func GetConfig(c *fiber.Ctx) error {
	var config models.AppConfig
	// Ambil data pertama aja karena kita cuma butuh 1 row config
	database.DB.First(&config)
	return c.JSON(config)
}

// UpdateConfig buat nyimpen pengaturan baru dari HP
func UpdateConfig(c *fiber.Ctx) error {
	var req models.AppConfig
	
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Data tidak valid"})
	}

	var config models.AppConfig
	database.DB.First(&config)

	// Timpa data lama dengan data baru
	config.Resolution = req.Resolution
	config.Codec = req.Codec
	config.AListURL = req.AListURL   // Tambahin ini
	config.AListPath = req.AListPath // Tambahin ini
	
	database.DB.Save(&config)

	return c.JSON(fiber.Map{
		"message": "Pengaturan berhasil disimpan!",
		"config":  config,
	})
}

// SearchYouTube nyari video di YouTube via yt-dlp
// Pastikan package "log" sudah ter-import di bagian atas ya Kang
func SearchYouTube(c *fiber.Ctx) error {
	query := c.Query("q")
	
	log.Println("=========================================")
	log.Println("🔍 [DEBUG YT] Menerima request pencarian:", query)

	if query == "" {
		log.Println("❌ [DEBUG YT] Query kosong!")
		return c.Status(400).JSON(fiber.Map{"error": "Mau nyari lagu apa Kang?"})
	}

	searchQuery := "ytsearch5:" + query + " karaoke"
	log.Println("🚀 [DEBUG YT] Eksekusi command: yt-dlp", searchQuery, "--dump-json", "--flat-playlist")
	
	cmd := exec.Command("yt-dlp", searchQuery, "--dump-json", "--flat-playlist")
	
	// Tangkap output sukses dan output error secara terpisah
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr 

	if err := cmd.Run(); err != nil {
		log.Println("❌ [DEBUG YT] Gagal eksekusi command yt-dlp:", err)
		log.Println("❌ [DEBUG YT] Pesan Error yt-dlp (stderr):", stderr.String())
		return c.Status(500).JSON(fiber.Map{"error": "Gagal nyari di YouTube."})
	}

	log.Printf("✅ [DEBUG YT] yt-dlp sukses! Ukuran output: %d bytes\n", len(out.String()))

	var results []map[string]interface{}
	
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	log.Printf("📊 [DEBUG YT] Ditemukan %d baris hasil (sebelum di-parse)\n", len(lines))

	for i, line := range lines {
		if line == "" {
			continue
		}
		
		var item map[string]interface{}
		if err := json.Unmarshal([]byte(line), &item); err == nil {
			
			// Logika ngambil Thumbnail
			thumbnailURL := ""
			if th, ok := item["thumbnail"].(string); ok {
				thumbnailURL = th
			} else if ths, ok := item["thumbnails"].([]interface{}); ok && len(ths) > 0 {
				if firstTh, ok := ths[0].(map[string]interface{}); ok {
					if url, ok := firstTh["url"].(string); ok {
						thumbnailURL = strings.Split(url, "?")[0] 
					}
				}
			}

			// Tampilkan di terminal ID dan Judul yang berhasil di-parse
			log.Printf("🎵 [DEBUG YT] %d: %v - %v (ID: %v)\n", i+1, item["uploader"], item["title"], item["id"])

			results = append(results, map[string]interface{}{
				"id":        item["id"],
				"title":     item["title"],
				"artist":    item["uploader"],
				"thumbnail": thumbnailURL,
				"source":    "youtube",
			})
		} else {
			log.Printf("⚠️ [DEBUG YT] Gagal nge-parse JSON di baris %d: %v\n", i+1, err)
		}
	}

	log.Printf("🎉 [DEBUG YT] Berhasil menyiapkan %d lagu buat dikirim ke HP\n", len(results))
	log.Println("=========================================")

	return c.JSON(results)
}

// QueueYouTube masukin lagu dari YouTube ke antrian
func QueueYouTube(c *fiber.Ctx) error {
	type Request struct {
		ID     string `json:"id"`
		Title  string `json:"title"`
		Artist string `json:"artist"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Data tidak valid"})
	}

	filePath := "youtube://" + req.ID

	// Cek apakah lagu ini udah pernah diputar sebelumnya
	var song models.Song
	result := database.DB.Where("file_path = ?", filePath).First(&song)
	
	if result.Error != nil {
		// Kalau belum ada, kita tambahin ke database
		song = models.Song{
			Title:    req.Title,
			Artist:   req.Artist,
			FilePath: filePath,
		}
		database.DB.Create(&song)
	}

	// Cek apakah ada lagu yang lagi jalan
	var playingCount int64
	database.DB.Model(&models.Queue{}).Where("status = ?", "playing").Count(&playingCount)

	// WAJIB masuk sebagai "waiting" dulu biar gak langsung di-kill sama ProcessNextSong
	queue := models.Queue{SongID: song.ID, Status: "waiting"}
	database.DB.Create(&queue)

	// Refresh queue di HP
	Broadcast <- map[string]interface{}{
		"action": "QUEUE_UPDATED",
	}

	// Kalau lagi nggak ada yang muter, panggil ProcessNextSong buat ngeksekusi
	if playingCount == 0 {
		go ProcessNextSong() 
	}

	return c.JSON(fiber.Map{"message": "Berhasil masuk antrian YouTube!"})
}

// ScanAList buat nyedot daftar lagu dari API AList
func ScanAList(c *fiber.Ctx) error {
	var config models.AppConfig
	database.DB.First(&config)

	if config.AListURL == "" || config.AListPath == "" {
		return c.Status(400).JSON(fiber.Map{"error": "URL atau Path AList belum disetting Kang!"})
	}

	// Endpoint API AList v3 buat ngelist isi folder
	apiURL := strings.TrimRight(config.AListURL, "/") + "/api/fs/list"
	
	payload := map[string]interface{}{
		"path": config.AListPath,
		"password": "",
		"page": 1,
		"per_page": 0, // 0 = Ambil semua file sekaligus biar malas bolak-balik
		"refresh": false,
	}
	
	jsonPayload, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Gagal ngeracik request."})
	}
	req.Header.Set("Content-Type", "application/json")

	// Panggil API AList-nya
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Gagal nyambung ke AList. Servernya nyala?"})
	}
	defer resp.Body.Close()

	// Parse struktur balasan AList
	var result struct {
		Code int `json:"code"`
		Data struct {
			Content []struct {
				Name  string `json:"name"`
				IsDir bool   `json:"is_dir"`
			} `json:"content"`
		} `json:"data"`
		Message string `json:"message"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Balasan AList nggak bisa dibaca."})
	}
	if result.Code != 200 {
		return c.Status(400).JSON(fiber.Map{"error": "AList nolak: " + result.Message})
	}

	added := 0
	for _, item := range result.Data.Content {
		if item.IsDir {
			continue // Lewatin kalau bentuknya folder
		}

		ext := strings.ToLower(filepath.Ext(item.Name))
		if ext == ".mp4" || ext == ".mkv" || ext == ".webm" {
			// Kasih tanda "alist://" biar server tahu ini dari cloud
			fullPath := "alist://" + strings.TrimRight(config.AListPath, "/") + "/" + item.Name
			
			var count int64
			database.DB.Model(&models.Song{}).Where("file_path = ?", fullPath).Count(&count)
			
			if count == 0 {
				filename := strings.TrimSuffix(item.Name, ext)
				parts := strings.SplitN(filename, "-", 2)
				artist := "Unknown"
				title := filename
				
				if len(parts) == 2 {
					artist = strings.TrimSpace(parts[0])
					title = strings.TrimSpace(parts[1])
				}
				
				newSong := models.Song{Title: title, Artist: artist, FilePath: fullPath}
				database.DB.Create(&newSong)
				added++
			}
		}
	}

	return c.JSON(fiber.Map{"message": "Scan AList sukses!", "added": added})
}

// ClearQueue: Kosongin antrian doang
func ClearQueue(c *fiber.Ctx) error {
	// Hapus semua data di tabel queues
	database.DB.Exec("DELETE FROM queues")

	// Teriak ke STB buat berhenti muter video
	Broadcast <- map[string]interface{}{
		"action": "STOP",
	}
	// Teriak ke HP buat nge-refresh layar antrian
	Broadcast <- map[string]interface{}{
		"action": "QUEUE_UPDATED",
	}

	return c.JSON(fiber.Map{"message": "Antrian berhasil dikosongkan!"})
}

// ClearSongs: Kosongin semua lagu hasil scan (Lokal + AList)
func ClearSongs(c *fiber.Ctx) error {
	// Hapus antrian dulu biar lagu yang lagi nyangkut di antrian ikut bersih
	database.DB.Exec("DELETE FROM queues")
	// Baru hapus semua lagunya
	database.DB.Exec("DELETE FROM songs")

	Broadcast <- map[string]interface{}{
		"action": "STOP",
	}
	Broadcast <- map[string]interface{}{
		"action": "QUEUE_UPDATED",
	}

	return c.JSON(fiber.Map{"message": "Semua data lagu berhasil dihapus dari database!"})
}

