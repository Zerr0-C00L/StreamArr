Thanks for following up, @kadeschs! You're absolutely right - those features were added later on Dec 18 after your initial build.

## ✅ Features Now Available

The latest version includes:
- **HLS.js player** - In-browser playback for Live TV channels
- **Stream proxy endpoint** - CORS bypass for seamless playback
- **Direct stream playback** - Click Play → stream opens in embedded player

These were added in commits after your Dec 18 morning snapshot, which is why you didn't see them.

## 🔄 Updating Your Build

Since you're on Synology/Portainer with a custom-built image, I've just added:

1. **Tagged Releases** - See the [v1.1.1 release](https://github.com/Zerr0-C00L/StreamArr_Pro/releases/tag/v1.1.1) for pre-built binaries and Docker images
2. **Comprehensive Update Guide** - Check out [docs/UPDATE-GUIDE.md](https://github.com/Zerr0-C00L/StreamArr_Pro/blob/main/docs/UPDATE-GUIDE.md) for detailed instructions

### Quick Update for Synology/Portainer:

**Option 1 - Rebuild from Latest:**
```bash
wget https://github.com/Zerr0-C00L/StreamArr_Pro/archive/refs/heads/main.zip
unzip main.zip
cd StreamArr_Pro-main
docker build -t streamarr_pro:latest .
```

Then in Portainer:
- Go to Stacks → Your StreamArr stack
- Click "Update the stack"
- Select "Re-pull image and redeploy"

**Option 2 - Use Tagged Release (when available):**
```yaml
services:
  streamarr:
    image: ghcr.io/zerr0-c00l/streamarr-pro:v1.1.1
```

Your PostgreSQL volume will be preserved, so all your data stays intact.

## 🎬 What You'll Get

After updating, you'll have:
- ✅ In-browser HLS player for Live TV
- ✅ Remove/Blacklist buttons in media detail modals
- ✅ CORS-free stream playback
- ✅ All the latest bug fixes and features

Closing this issue as resolved. Feel free to reopen if you encounter any problems after updating!
