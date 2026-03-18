const host = window.location.host;
let ws;

// Inisialisasi awal
document.addEventListener('DOMContentLoaded', () => {
    connectWebSocket();
    fetchSongs();
    fetchQueue();
});

// 1. Setup WebSocket buat interaksi langsung ke Player
function connectWebSocket() {
    // 1. Cek dulu halamannya pakai HTTP atau HTTPS
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    // 2. Ambil host-nya otomatis (biar gak hardcode IP/Domain)
    const host = window.location.host;
    // 3. Gabungin! Kalau ditaruh di belakang Caddy, dia otomatis jadi wss://domain.com/ws
    const wsURL = `${protocol}//${host}/ws?type=remote`;
    // 4. Buka koneksinya
    const ws = new WebSocket(wsURL);

    ws.onopen = () => console.log('✅ Remote terhubung ke Server');

    ws.onmessage = (event) => {
        const msg = JSON.parse(event.data);
        // Kalau ada update antrian dari user lain, refresh list
        if (msg.action === 'QUEUE_UPDATED') {
            fetchQueue();
        }
    };

    ws.onclose = () => {
        console.log('❌ Terputus dari server, mencoba reconnect...');
        setTimeout(connectWebSocket, 3000);
    };
}

// 2. Fungsi Kirim Perintah ke Player (TV)
function sendCommand(action) {
    if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ action: action }));
        console.log(`📡 Perintah dikirim: ${action}`);
    } else {
        alert("Belum terhubung ke server!");
    }
}

// 3. Ambil data lagu dari API
// Variabel global buat nginget posisi halaman dan kata kunci pencarian
let currentLocalPage = 1;
let currentLocalQuery = '';

// Fungsi fetchSongs yang udah di-upgrade
async function fetchSongs(query = '', append = false) {
    // Kalau bukan 'load more' (pencarian baru atau awal buka), reset ke halaman 1
    if (!append) {
        currentLocalPage = 1;
        currentLocalQuery = query;
        document.getElementById('songList').innerHTML = '<div class="text-center py-4"><div class="animate-spin rounded-full h-8 w-8 border-t-2 border-b-2 border-pink-500 mx-auto mb-2"></div><p class="text-pink-400 text-sm">Memuat lagu...</p></div>';
    }

    try {
        // Panggil API dengan parameter halaman
        const res = await fetch(`/api/songs?q=${currentLocalQuery}&page=${currentLocalPage}`);
        const data = await res.json();

        const container = document.getElementById('songList');
        const loadMoreBtn = document.getElementById('loadMoreContainer');

        // Kalau pencarian baru, kosongin list lama
        if (!append) container.innerHTML = '';

        // Kalau datanya kosong
        if (!data.data || data.data.length === 0) {
            if (!append) container.innerHTML = '<p class="text-center text-gray-500 italic py-4">Belum ada lagu. Coba scan dulu Kang!</p>';
            loadMoreBtn.classList.add('hidden');
            return;
        }

        // Render lagunya
        data.data.forEach(song => {
            const div = document.createElement('div');
            div.className = "flex justify-between items-center p-3 bg-gray-800 rounded-xl shadow-sm border border-gray-700";

            // Bikin label pembeda biar tahu ini lagu Lokal atau AList
            const isAList = song.file_path.startsWith('alist://');
            const badge = isAList
                ? '<span class="bg-blue-600 text-[10px] px-1.5 py-0.5 rounded text-white ml-2 align-middle">☁️ AList</span>'
                : '<span class="bg-pink-600 text-[10px] px-1.5 py-0.5 rounded text-white ml-2 align-middle">📁 Lokal</span>';

            div.innerHTML = `
                        <div class="flex-1 min-w-0 pr-4">
                            <h3 class="font-bold text-white truncate text-sm mb-0.5">${escapeHtml(song.title)}</h3>
                            <p class="text-xs text-gray-400 truncate">${escapeHtml(song.artist)} ${badge}</p>
                        </div>
                        <button onclick="addToQueue(${song.ID})" class="bg-pink-600 hover:bg-pink-500 text-white font-bold h-10 px-3 rounded-lg transition-colors text-xs whitespace-nowrap flex-shrink-0 shadow-md">
                            + Antri
                        </button>
                    `;
            container.appendChild(div);
        });

        // Cek apakah masih ada sisa halaman? Kalau ada, munculin tombolnya
        if (currentLocalPage < data.total_pages) {
            loadMoreBtn.classList.remove('hidden');
        } else {
            loadMoreBtn.classList.add('hidden');
        }

    } catch (err) {
        console.error(err);
        if (!append) document.getElementById('songList').innerHTML = '<p class="text-center text-red-500 italic py-4">Gagal mengambil data lagu.</p>';
    }
}

// Fungsi pemicu saat tombol "Muat Lebih Banyak" dipencet
function loadMoreSongs() {
    currentLocalPage++;
    fetchSongs(currentLocalQuery, true); // true = mode 'append' (tambahin ke bawah list)
}

// 4. Render list lagu ke HTML
function renderSongs(songs) {
    const container = document.getElementById('songList');
    container.innerHTML = '';

    if (!songs || songs.length === 0) {
        container.innerHTML = '<p class="text-center text-gray-500 italic">Lagu tidak ditemukan. Udah di-scan belum?</p>';
        return;
    }

    songs.forEach(song => {
        const div = document.createElement('div');
        div.className = "flex justify-between items-center p-4 bg-gray-800 rounded-xl shadow-sm border border-gray-700";
        div.innerHTML = `
                    <div class="flex-1 truncate pr-4">
                        <h3 class="font-bold text-white truncate">${song.title}</h3>
                        <p class="text-sm text-gray-400 truncate">${song.artist}</p>
                    </div>
                    <button onclick="addToQueue(${song.ID})" class="bg-pink-600 hover:bg-pink-500 text-white font-bold py-2 px-4 rounded-lg transition-colors text-sm">
                        + Antri
                    </button>
                `;
        container.appendChild(div);
    });
}

// 5. Tambah ke Antrian via API
async function addToQueue(songId) {
    try {
        const res = await fetch('/api/queue', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ song_id: songId })
        });

        if (res.ok) {
            // Berhasil masuk antrian!
            // Notifikasi visual (opsional) atau langsung refresh queue
            fetchQueue();
        } else {
            alert("Gagal menambahkan ke antrian");
        }
    } catch (err) {
        console.error("Error queue:", err);
    }
}

// 6. Ambil status antrian saat ini
async function fetchQueue() {
    try {
        const res = await fetch('/api/queue');
        const queues = await res.json();
        renderQueue(queues);
    } catch (err) {
        console.error("Gagal ambil antrian", err);
    }
}

// 7. Render list antrian ke HTML
function renderQueue(queues) {
    const container = document.getElementById('queueList');
    container.innerHTML = '';

    if (!queues || queues.length === 0) {
        container.innerHTML = '<p class="text-center text-gray-500 text-sm italic py-2">Antrian kosong. Ayo pilih lagu!</p>';
        return;
    }

    queues.forEach((q, index) => {
        const div = document.createElement('div');
        // Kasih warna beda buat lagu yang lagi "playing" (index 0 / status tertentu)
        const isPlaying = q.status === 'playing' || index === 0;
        div.className = `flex justify-between items-center p-3 rounded-lg border ${isPlaying ? 'bg-pink-900 bg-opacity-30 border-pink-700' : 'bg-gray-800 border-gray-700'}`;
        div.innerHTML = `
                    <div class="flex items-center space-x-3 truncate">
                        <span class="${isPlaying ? 'text-pink-400' : 'text-gray-500'} font-bold w-6 text-center">${index + 1}</span>
                        <div class="truncate">
                            <p class="font-semibold text-white truncate text-sm">${q.song.title}</p>
                            <p class="text-xs text-gray-400 truncate">${q.song.artist}</p>
                        </div>
                    </div>
                    ${isPlaying ? '<span class="text-xs bg-pink-600 text-white px-2 py-1 rounded">Main</span>' : ''}
                `;
        container.appendChild(div);
    });
}
// 8. Fungsi buat trigger Scan ke Server Debian
async function scanLibrary() {
    const btn = document.getElementById('btnScan');
    const originalText = btn.innerHTML;

    // Ubah tampilan tombol biar kelihatan lagi loading
    btn.innerHTML = "⏳ Wait...";
    btn.disabled = true;
    btn.classList.add("opacity-50", "cursor-not-allowed");

    try {
        const res = await fetch('/api/scan', { method: 'POST' });
        const data = await res.json();

        if (res.ok) {
            alert(`Scan beres Kang! Nemu ${data.added} lagu baru.`);
            // Langsung refresh list lagu biar yang baru muncul
            fetchSongs(document.getElementById('searchInput').value);
        } else {
            alert("Waduh, gagal scan: " + (data.error || "Error tidak diketahui"));
        }
    } catch (err) {
        console.error("Error pas scan:", err);
        alert("Gagal menghubungi server Debian buat scan.");
    } finally {
        // Balikin tombol ke semula
        btn.innerHTML = originalText;
        btn.disabled = false;
        btn.classList.remove("opacity-50", "cursor-not-allowed");
    }
}

// 9. Logika UI Pengaturan (Settings Modal)
const settingsModal = document.getElementById('settingsModal');

// Pas modal dibuka, langsung narik data konfigurasi terbaru dari Debian
async function openSettings() {
    settingsModal.classList.remove('hidden');
    try {
        const res = await fetch('/api/config');
        const config = await res.json();
        if (config) {
            document.getElementById('configRes').value = config.resolution || '720';
            document.getElementById('configCodec').value = config.codec || 'h264';
            document.getElementById('configAListURL').value = config.alist_url || 'http://127.0.0.1:5244';
            document.getElementById('configAListPath').value = config.alist_path || '/';
        }
    } catch (err) {
        console.error("Gagal memuat pengaturan:", err);
    }
}

// Nutup modal
function closeSettings() {
    settingsModal.classList.add('hidden');
}

// Pas tombol Simpan dipencet, kirim update ke backend
async function saveSettings() {
    const resVal = document.getElementById('configRes').value;
    const codecVal = document.getElementById('configCodec').value;
    const alistUrlVal = document.getElementById('configAListURL').value;
    const alistPathVal = document.getElementById('configAListPath').value;

    try {
        const res = await fetch('/api/config', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                resolution: resVal,
                codec: codecVal,
                alist_url: alistUrlVal,
                alist_path: alistPathVal
            })
        });

        if (res.ok) {
            alert("Mantap! Pengaturan berhasil disimpan.");
            closeSettings();
        } else {
            alert("Waduh, gagal menyimpan pengaturan.");
        }
    } catch (err) {
        console.error("Error save config:", err);
        alert("Terjadi kesalahan saat menghubungi server.");
    }
}

let currentMode = 'local';

// Fungsi buat gonta-ganti tampilan tab
function switchMode(mode) {
    currentMode = mode;
    const tabLocal = document.getElementById('tabLocal');
    const tabYT = document.getElementById('tabYT');
    const input = document.getElementById('searchInput');
    const btnScan = document.getElementById('btnScan');
    const btnAlist = document.getElementById('btnScanAList');

    if (mode === 'local') {
        tabLocal.className = "flex-1 py-2 rounded-lg font-bold bg-pink-600 text-white shadow-md transition";
        tabYT.className = "flex-1 py-2 rounded-lg font-bold bg-gray-800 text-gray-400 border border-gray-700 transition";
        input.placeholder = "Cari penyanyi atau judul lagu lokal...";
        btnScan.classList.remove('hidden'); // Tunjukin tombol scan
        btnAlist.classList.remove('hidden');
        fetchSongs(); // Reset list ke lokal
    } else {
        tabYT.className = "flex-1 py-2 rounded-lg font-bold bg-red-600 text-white shadow-md transition";
        tabLocal.className = "flex-1 py-2 rounded-lg font-bold bg-gray-800 text-gray-400 border border-gray-700 transition";
        input.placeholder = "Ketik pencarian di YouTube...";
        btnScan.classList.add('hidden'); // Sembunyiin scan kalau lagi mode YT
        btnAlist.classList.add('hidden');
        document.getElementById('songList').innerHTML = '<p class="text-center text-gray-500 italic">Ketik judul lalu tekan 🔍</p>';
    }
}

// Fungsi jembatan pas tombol 🔍 dipencet atau enter
function executeSearch() {
    const query = document.getElementById('searchInput').value;
    if (currentMode === 'local') {
        fetchSongs(query);
    } else {
        if (!query) return alert("Ketik dulu lagunya Kang!");
        searchYouTube(query);
    }
}

// Ganti event listener enter yang lama jadi ini
document.getElementById('searchInput').addEventListener('keyup', (e) => {
    if (e.key === 'Enter') executeSearch();
});

// Nge-hit API YouTube yang baru kita bikin di Go
async function searchYouTube(query) {
    const container = document.getElementById('songList');

    // Loading Spinner yang lebih jelas biar gak dikira error
    container.innerHTML = `
                <div class="flex flex-col items-center justify-center py-10">
                    <div class="animate-spin rounded-full h-12 w-12 border-t-2 border-b-2 border-pink-500 mb-4"></div>
                    <p class="text-pink-400 font-bold animate-pulse">Sabar Kang, lagi nyari di YouTube...</p>
                </div>
            `;

    try {
        const res = await fetch(`/api/youtube/search?q=${query}`);

        // Cek kalau server ngasih respon error (misal 404 atau 500)
        if (!res.ok) {
            throw new Error(`Server membalas dengan status: ${res.status}`);
        }

        const songs = await res.json();
        container.innerHTML = '';

        if (!songs || songs.length === 0) {
            container.innerHTML = '<p class="text-center text-gray-500 italic">Waduh, nggak nemu di YouTube.</p>';
            return;
        }

        songs.forEach(song => {
            const div = document.createElement('div');
            // Layout baru: Grid/Flex biar rapi ada gambar, teks, dan tombol
            div.className = "flex items-center gap-4 p-3 bg-gray-800 rounded-xl shadow-sm border border-red-900";

            // Siapin fallback image kalau thumbnail gagal dimuat
            const thumbImg = song.thumbnail ? song.thumbnail : 'https://via.placeholder.com/120x90.png?text=No+Image';
            const safeTitle = escapeHtml(song.title);
            const safeArtist = escapeHtml(song.artist);

            div.innerHTML = `
                        <div class="w-24 h-16 flex-shrink-0 rounded-lg overflow-hidden bg-black relative border border-gray-700">
                            <img src="${thumbImg}" class="w-full h-full object-cover" alt="Thumbnail">
                            <div class="absolute bottom-0 right-0 bg-red-600 text-white text-[10px] px-1 font-bold">YT</div>
                        </div>
                        <div class="flex-1 min-w-0">
                            <h3 class="font-bold text-white truncate text-sm leading-tight mb-1">${safeTitle}</h3>
                            <p class="text-xs text-gray-400 truncate text-red-400">${safeArtist}</p>
                        </div>
                        <button onclick="queueYouTube('${song.id}', '${safeTitle}', '${safeArtist}')" class="bg-red-600 hover:bg-red-500 text-white font-bold h-10 px-3 rounded-lg transition-colors text-xs whitespace-nowrap flex-shrink-0">
                            + Add
                        </button>
                    `;
            container.appendChild(div);
        });
    } catch (err) {
        console.error("Error pas search YT:", err);
        container.innerHTML = '<p class="text-center text-red-500 italic mt-4">Gagal terhubung ke server atau pencarian error.</p>';
    }
}

// Fungsi pembantu biar judul lagu yang ada tanda kutipnya gak bikin error HTML
function escapeHtml(text) {
    return (text || '').toString()
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#039;");
}

// Fungsi buat masukin lagu YT ke database dan antrian
async function queueYouTube(id, title, artist) {
    try {
        const res = await fetch('/api/youtube/queue', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ id: id, title: title, artist: artist })
        });

        if (res.ok) {
            // Refresh list antrian di layar HP
            fetchQueue();
        } else {
            alert("Waduh, gagal masukin lagu YouTube ke antrian.");
        }
    } catch (err) {
        console.error("Error queue YT:", err);
        alert("Gagal terhubung ke server.");
    }
}

async function scanAList() {
    const btn = document.getElementById('btnScanAList');
    const originalText = btn.innerHTML;
    btn.innerHTML = "⏳ Wait...";
    btn.disabled = true;

    try {
        const res = await fetch('/api/alist/scan', { method: 'POST' });
        const data = await res.json();

        if (res.ok) {
            alert(`Mantap! Nemu ${data.added} lagu baru dari AList.`);
            fetchSongs(document.getElementById('searchInput').value);
        } else {
            alert("Gagal scan AList: " + (data.error || "Error tidak diketahui"));
        }
    } catch (err) {
        alert("Gagal menghubungi server Debian.");
    } finally {
        btn.innerHTML = originalText;
        btn.disabled = false;
    }
}

// Fungsi JS buat Hapus Antrian
async function clearQueueBtn() {
    if (!confirm("Yakin mau bersihin semua antrian? STB bakal berhenti muter loh.")) return;
    try {
        const res = await fetch('/api/queue/clear', { method: 'DELETE' });
        if (res.ok) {
            fetchQueue(); // Refresh layar HP
        }
    } catch (err) {
        console.error(err);
        alert("Gagal menghubungi server.");
    }
}

// Fungsi JS buat Hapus Database Lagu
async function clearSongsBtn() {
    if (!confirm("⚠️ YAKIN KANG? Semua database lagu lokal & AList bakal kehapus!")) return;
    try {
        const res = await fetch('/api/songs/clear', { method: 'DELETE' });
        if (res.ok) {
            alert("Database udah bersih dari sisa-sisa kenangan Kang! 🧹");
            fetchSongs(); // Kosongin list di layar
            fetchQueue();
            closeSettings(); // Tutup modalnya
        }
    } catch (err) {
        console.error(err);
        alert("Gagal menghapus database.");
    }
}