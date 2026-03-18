const video = document.getElementById('video-player');
const idleScreen = document.getElementById('idle-screen');
const startOverlay = document.getElementById('start-overlay');
const nowPlaying = document.getElementById('now-playing');
const currentTitle = document.getElementById('current-title');

let ws;

// Fungsi awal buat bypass blokir autoplay browser
function initPlayer() {
    startOverlay.classList.add('hidden');
    idleScreen.classList.remove('hidden');
    connectWebSocket();
}

function connectWebSocket() {
    // 1. Cek dulu halamannya pakai HTTP atau HTTPS
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    // 2. Ambil host-nya otomatis (biar gak hardcode IP/Domain)
    const host = window.location.host;
    // 3. Gabungin! Kalau ditaruh di belakang Caddy, dia otomatis jadi wss://domain.com/ws
    const wsURL = `${protocol}//${host}/ws?type=player`;
    // 4. Buka koneksinya
    const ws = new WebSocket(wsURL);

    ws.onopen = () => console.log('✅ STB Terhubung ke Server Karaoke');

    ws.onmessage = (event) => {
        const msg = JSON.parse(event.data);
        console.log("📩 Perintah dari Boss:", msg);

        if (msg.action === 'PLAY_SONG') {
            playSong(msg.url, msg.title);
        } else if (msg.action === 'PAUSE') {
            video.pause();
        } else if (msg.action === 'RESUME') {
            video.play();
        } else if (msg.action === 'STOP') {
            stopPlayer();
        } else if (msg.action === 'QUEUE_UPDATED') {
            // Nanti bisa panggil fungsi update UI antrian di sini
            console.log("Antrian ada yang baru!");
        }
    };

    ws.onclose = () => {
        console.log('❌ Koneksi terputus, mencoba reconnect...');
        setTimeout(connectWebSocket, 3000); // Auto reconnect kalau server restart
    };
}

function playSong(url, title) {
    idleScreen.classList.add('hidden');
    video.classList.remove('hidden');

    video.src = url;
    video.play();

    // Tunjukin judul lagu di pojok kiri bawah
    currentTitle.innerText = title;
    nowPlaying.classList.remove('hidden');

    // Sembunyiin info judul setelah 10 detik biar layar bersih buat nyanyi
    setTimeout(() => {
        nowPlaying.classList.add('hidden');
    }, 10000);
}

function stopPlayer() {
    video.pause();
    video.src = "";
    video.classList.add('hidden');
    idleScreen.classList.remove('hidden');
    nowPlaying.classList.add('hidden');
}

// Kalau lagu beres, kasih tau server minta lagu selanjutnya
video.addEventListener('ended', () => {
    stopPlayer();
    if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ action: 'SONG_ENDED' }));
    }
});