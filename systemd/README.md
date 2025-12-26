# SystemD Service Files

This directory contains systemd service unit files for StreamArr Pro.

## Services

### streamarr.service
The main StreamArr Pro API server service.

### streamarr-worker.service
Background worker service for scheduled tasks:
- MDBList sync (every 6 hours)
- Playlist generation (every 12 hours)
- EPG updates (every 6 hours)
- Cache cleanup (every hour)
- Collection sync (every 24 hours)
- Episode scanning (every 24 hours)
- Stream search (every 6 hours)
- Balkan VOD sync (every 24 hours)

**Note:** The worker service is configured with `BindsTo` and `PartOf` directives to ensure it automatically restarts when the main server restarts.

## Installation

1. Copy service files to systemd directory:
```bash
sudo cp systemd/*.service /etc/systemd/system/
```

2. Reload systemd daemon:
```bash
sudo systemctl daemon-reload
```

3. Enable services to start on boot:
```bash
sudo systemctl enable streamarr.service
sudo systemctl enable streamarr-worker.service
```

4. Start services:
```bash
sudo systemctl start streamarr.service
sudo systemctl start streamarr-worker.service
```

## Configuration

Before starting the services, ensure you update the following in the service files:

- `WorkingDirectory`: Path to your StreamArr Pro installation
- `DATABASE_URL`: Your PostgreSQL connection string
- `JWT_SECRET`: A secure random secret for JWT token generation
- `User`: The user that should run the services (default: root)

## Viewing Logs

```bash
# Server logs
sudo journalctl -u streamarr.service -f

# Worker logs
sudo journalctl -u streamarr-worker.service -f

# Both services
sudo journalctl -u streamarr.service -u streamarr-worker.service -f
```

## Service Management

```bash
# Restart services
sudo systemctl restart streamarr.service

# Stop services
sudo systemctl stop streamarr.service streamarr-worker.service

# Check status
sudo systemctl status streamarr.service
sudo systemctl status streamarr-worker.service
```
