# Walk Assistant - Quick Start Guide

Get up and running with Walk Assistant in 5 minutes!

## Step 1: Get Google OAuth Credentials (5 minutes)

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select existing
3. Navigate to **APIs & Services** → **Credentials**
4. Click **Create Credentials** → **OAuth client ID**
5. Configure consent screen if needed (select External)
6. Choose **Web application**
7. Add authorized redirect URI:
   - Local dev: `http://localhost:8080/auth/google/callback`
   - Production: `https://yourdomain.com/auth/google/callback`
8. Click **Create**
9. **Copy your Client ID and Client Secret**

## Step 2: Generate Cookie Keys

```bash
# Generate hash key (64 bytes)
openssl rand -hex 64

# Generate block key (32 bytes)
openssl rand -hex 32
```

**Save these keys!** You'll need them in the next step.

## Step 3: Set Environment Variables

Create a `.env` file or export directly:

```bash
export GOOGLE_CLIENT_ID="paste-your-client-id-here.apps.googleusercontent.com"
export GOOGLE_CLIENT_SECRET="paste-your-client-secret-here"
export GOOGLE_REDIRECT_URL="http://localhost:8080/auth/google/callback"
export COOKIE_HASH_KEY="paste-your-128-char-hash-key-here"
export COOKIE_BLOCK_KEY="paste-your-64-char-block-key-here"
```

## Step 4: Run the Application

### Option A: Using Go (Development)

```bash
# Clone and enter directory
git clone https://github.com/korjavin/walkassistant.git
cd walkassistant

# Install dependencies
go mod download

# Run the server
cd backend
go run main.go
```

### Option B: Using Docker

```bash
docker run -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  -e GOOGLE_CLIENT_ID="$GOOGLE_CLIENT_ID" \
  -e GOOGLE_CLIENT_SECRET="$GOOGLE_CLIENT_SECRET" \
  -e GOOGLE_REDIRECT_URL="$GOOGLE_REDIRECT_URL" \
  -e COOKIE_HASH_KEY="$COOKIE_HASH_KEY" \
  -e COOKIE_BLOCK_KEY="$COOKIE_BLOCK_KEY" \
  ghcr.io/korjavin/walkassistant:latest
```

### Option C: Using Docker Compose

1. Create `deployments/.env` file with all environment variables
2. Run:
```bash
cd deployments
docker-compose up -d
```

## Step 5: Use the Application

1. Open http://localhost:8080
2. Click **Login with Google**
3. Authorize the application
4. Upload your GPX files
5. Explore suggested routes!

## Troubleshooting

### "Google login is not configured"
❌ Environment variables not set  
✅ Double-check `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, and `GOOGLE_REDIRECT_URL`

### "Redirect URI mismatch"
❌ Redirect URL doesn't match Google Cloud Console  
✅ Ensure exact match: `http://localhost:8080/auth/google/callback`

### "Invalid session cookie"
❌ Cookie keys are incorrect or changed  
✅ Verify `COOKIE_HASH_KEY` and `COOKIE_BLOCK_KEY` are set correctly

### Can't see routes after login
❌ Database or file permission issues  
✅ Check `data/` directory exists and is writable

## Next Steps

- 📖 Read [SETUP.md](SETUP.md) for detailed configuration
- 🔧 Check [CHANGES.md](CHANGES.md) for what's new
- 🐛 Report issues at https://github.com/korjavin/walkassistant/issues

## Pro Tips

💡 **Multi-user**: Each Google account gets isolated storage  
💡 **Backup**: Copy `data/walkassistant.db` and `data/users/` regularly  
💡 **Production**: Always use HTTPS and update redirect URL  
💡 **Privacy**: Your routes never leave your server

---

**Questions?** See the full documentation in [SETUP.md](SETUP.md)
