# Self-hosting anilistgen

> **Deprecated**: This project is superseded by
> [sonarr-anime-bridge](https://github.com/calmcacil/sonarr-anime-bridge),
> which provides a Docker container with the same functionality plus anime
> movie list support. The instructions below are kept for reference.

Run your own instance locally.

---

## Local setup

### Prerequisites

- Go 1.24+
- Internet access (AniList API + mapping file downloads)

### Install

```bash
# Clone
git clone https://github.com/calmcacil/sonarr-seasonal-lists.git
cd anilistgen

# Build
go build ./cmd/anilistgen

# Generate config
./anilistgen init-config
# Edit anilistgen.yaml to your liking

# Test
./anilistgen -dry-run

# Generate
./anilistgen -output ./sonarr-lists
```

### Run periodically (cron)

Run weekly to keep lists current:

```cron
# Every Sunday at 6 AM
0 6 * * 0 cd /path/to/anilistgen && ./anilistgen -output /var/www/lists
```

### Run periodically (systemd)

**`/etc/systemd/system/anilistgen.service`**:
```ini
[Unit]
Description=anilistgen seasonal anime lists
After=network-online.target

[Service]
Type=oneshot
ExecStart=/usr/local/bin/anilistgen -output /var/www/lists
WorkingDirectory=/etc/anilistgen
```

**`/etc/systemd/system/anilistgen.timer`**:
```ini
[Unit]
Description=Run anilistgen weekly

[Timer]
OnCalendar=Sun 06:00
Persistent=true

[Install]
WantedBy=timers.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now anilistgen.timer
```

### Serve the files

Any static file server works:

```bash
# Python (quick test)
python3 -m http.server 8080 -d ./sonarr-lists

# nginx
server {
    listen 80;
    server_name lists.example.com;
    root /var/www/lists;
}
```

Then point Sonarr at `http://localhost:8080/winter-2026.json`.
