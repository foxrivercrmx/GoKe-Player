#!/bin/bash
# Installer Otomatis GoKe Player (Malas & Hemat Edition)

set -e

echo "===================================================="
echo "  Mulai Instalasi GoKe Player & Dependencies..."
echo "===================================================="

# 1. Pastikan jalan pakai root/sudo
if [ "$EUID" -ne 0 ]; then
  echo "❌ Tolong jalankan pakai sudo ya Kang! (sudo bash install.sh)"
  exit
fi

# 2. Install dependencies dasar & FFmpeg
echo "📦 Menginstall FFmpeg & alat tukang..."
apt-get update -y
apt-get install -y curl wget ffmpeg build-essential git

# 3. Rakit QuickJS (JS Runtime Hemat RAM)
if ! command -v qjs &> /dev/null; then
    echo "🪶 Merakit QuickJS dari source..."
    rm -rf /tmp/quickjs
    git clone https://github.com/bellard/quickjs.git /tmp/quickjs
    cd /tmp/quickjs
    make
    make install
    # Bikin symlink biar yt-dlp gampang nyarinya
    ln -sf /usr/local/bin/qjs /usr/bin/quickjs
    ln -sf /usr/local/bin/qjs /usr/local/bin/quickjs
    cd ~
else
    echo "✅ QuickJS sudah terinstall!"
fi

# 4. Install yt-dlp versi terbaru
if ! command -v qjs &> /dev/null; then
    echo "📥 Menginstall yt-dlp..."
    wget https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -O /usr/local/bin/yt-dlp
    chmod a+rx /usr/local/bin/yt-dlp
else
    echo "✅ YT-dlp sudah terinstall!"
fi

# 5. Siapkan folder dan Download Binary GoKe
echo "🚀 Mengambil binary GoKe Player..."
mkdir -p /opt/goke-player
cd /opt/goke-player
mkdir -p /opt/goke-player/data

# GANTI URL INI DENGAN LINK DOWNLOAD BINARY GOKE DARI GITLAB/GITHUB AKANG
GOKE_URL="https://gitlab.com/denx.bluemonday/goke-player/-/jobs/artifacts/main/raw/goke-player?job=build_goke" 
wget -q --show-progress "$GOKE_URL" -O goke-player
chmod +x goke-player
mv goke-player /usr/local/bin/

# Bikin file cookies kosong biar yt-dlp gak error nyari file-nya
touch /opt/goke-player/cookies.txt
touch /opt/goke-player/karaoke.db

# 6. Bikin Systemd Service
echo "⚙️ Membuat Systemd Service..."
cat <<EOF > /etc/systemd/system/goke.service
[Unit]
Description=GoKe Karaoke Player Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/goke-player
# Jalankan dengan flag quickjs yang udah kita buat tadi
ExecStart=/usr/local/bin/goke-player -storage=/opt/goke-player/data -db=/opt/goke-player/karaoke.db -port=5700 -js-runtimes=quickjs -cookies=/opt/goke-player/cookies.txt
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF

# 7. Nyalakan Service
echo "🔥 Menyalakan GoKe Player..."
systemctl daemon-reload
systemctl enable goke
systemctl restart goke

echo "===================================================="
echo "  WES BERES KANG! GoKe sudah jalan di background."
echo "  Cek status: systemctl status goke"
echo "  Cek log: journalctl -fu goke"
echo "===================================================="