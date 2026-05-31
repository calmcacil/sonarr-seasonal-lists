# Self-hosting anilistgen

Run your own instance — either locally or on your own GitHub repo.

---

## Local setup

### Prerequisites

- Go 1.24+
- Internet access (AniList API + mapping file downloads)

### Install

```bash
# Clone
git clone https://github.com/calmcacil/anilistgen.git
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

---

## Fork on GitHub

Run the exact same setup as the hosted repo on your own GitHub account.

### 1. Fork the repo

Click Fork on [github.com/calmcacil/anilistgen](https://github.com/calmcacil/anilistgen).

### 2. Enable GitHub Pages

Settings → Pages → Source: **Deploy from a branch** → Branch: `gh-pages`, path: `/`.

### 3. (Optional) Customize the config

Edit `anilistgen.yaml` in your fork to change years, filters, blacklist, etc.
The workflow reads this file from the repo.

### 4. Run the action

Actions → **Weekly anime list sync** → Run workflow (or wait for the Sunday schedule).

### 5. Use your lists

```
https://{your-username}.github.io/anilistgen/winter-2026.json
```

---

## Hosting options

### GitHub Pages (free)

Works out of the box with the included workflow. Files are public.

### Cloudflare Pages / Netlify

Point to the `./out` directory. Use the workflow to build, then deploy
via their GitHub integration.

### S3 / Cloudflare R2

Modify the workflow to upload to S3 instead of gh-pages:

```yaml
- run: ./anilistgen -output ./out
- uses: jakejarvis/s3-sync-action@v0.5.1
  with:
    args: --acl public-read --delete
  env:
    AWS_S3_BUCKET: my-lists
    AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
    AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
```

### Local web server

nginx, Caddy, or any static file server pointed at the output directory.
Useful for LAN-only setups.
