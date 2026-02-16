# FunBussin

Syncs waiver data from Smartwaiver to Mailchimp.

## Docker

### Build (Linux)
```bash
docker build --platform linux/amd64 -t funbussin .
```

### Build and push to registry
```bash
docker build --platform linux/amd64 -t 192.168.10.50:5000/local/funbussin:latest .
docker push 192.168.10.50:5000/local/funbussin:latest
```

### Pull and run from registry
```bash
docker pull 192.168.10.50:5000/local/funbussin:latest
docker run -d --restart on-failure --env-file .env 192.168.10.50:5000/local/funbussin:latest
```

### Run (sync every 15 min until stopped)
```bash
docker run -d --restart on-failure --env-file .env funbussin
```
Default: `-sync -interval 15m`

Or with env vars:
```bash
docker run -d --restart on-failure \
  -e SMARTWAIVER_API_KEY=your_smartwaiver_api_key \
  -e MAILCHIMP_API_KEY=your_mailchimp_api_key \
  -e MAILCHIMP_SERVER=us19 \
  -e MAILCHIMP_LIST_ID=your_list_id \
  -e TZ=America/New_York \
  funbussin
```

### Options
```bash
# One-time sync (no loop)
docker run --rm --env-file .env funbussin -sync

# Sync every 30 minutes
docker run -d --restart on-failure --env-file .env funbussin -sync -interval 30m

# Dry run (fetch only, no import)
docker run --rm --env-file .env funbussin -sync -dry-run

# Fetch all waivers (no 24h filter)
docker run --rm --env-file .env funbussin -sync -all

# Import from JSON file (mount file first)
docker run --rm --env-file .env -v $(pwd)/data:/data funbussin -import -file /data/contacts.json
```

## Production

Pull from registry and run (sync every 15 min, restarts on crash):
```bash
docker pull 192.168.10.50:5000/local/funbussin:latest
docker run -d --restart on-failure --env-file /path/to/.env 192.168.10.50:5000/local/funbussin:latest
```
Stop with `docker stop <container_id>`
