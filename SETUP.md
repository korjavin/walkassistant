# Walk Assistant - Google OAuth Setup Guide

This guide will help you set up Google OAuth authentication for Walk Assistant, ensuring that each user can only see and manage their own routes.

## Table of Contents

1. [Overview](#overview)
2. [Prerequisites](#prerequisites)
3. [Google OAuth Setup](#google-oauth-setup)
4. [Environment Configuration](#environment-configuration)
5. [Local Development](#local-development)
6. [Production Deployment](#production-deployment)
7. [Database](#database)
8. [Security Notes](#security-notes)

---

## Overview

Walk Assistant now includes:
- **Google OAuth authentication** - Users must log in with their Google account
- **User-specific route storage** - Each user only sees their own GPX files and routes
- **SQLite database** - Routes are stored in a persistent database
- **Secure sessions** - Cookie-based sessions with encryption

---

## Prerequisites

- Go 1.24 or later
- A Google account
- Access to Google Cloud Console
- (For production) A domain name with HTTPS

---

## Google OAuth Setup

### Step 1: Create a Google Cloud Project

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Click **Select a project** → **New Project**
3. Enter a project name (e.g., "Walk Assistant")
4. Click **Create**

### Step 2: Enable Google+ API

1. In your project, go to **APIs & Services** → **Library**
2. Search for "Google+ API" (or "People API")
3. Click **Enable**

### Step 3: Create OAuth 2.0 Credentials

1. Go to **APIs & Services** → **Credentials**
2. Click **Create Credentials** → **OAuth client ID**
3. If prompted, configure the OAuth consent screen:
   - Choose **External** (unless you have a Google Workspace)
   - Fill in:
     - **App name**: Walk Assistant
     - **User support email**: Your email
     - **Developer contact**: Your email
   - Click **Save and Continue**
   - Skip **Scopes** (click Save and Continue)
   - Add test users if needed (your own email)
   - Click **Save and Continue**

4. Back to **Create OAuth client ID**:
   - **Application type**: Web application
   - **Name**: Walk Assistant Web Client
   - **Authorized JavaScript origins**:
     - For local dev: `http://localhost:8080`
     - For production: `https://yourdomain.com`
   - **Authorized redirect URIs**:
     - For local dev: `http://localhost:8080/auth/google/callback`
     - For production: `https://yourdomain.com/auth/google/callback`
   - Click **Create**

5. **Save your credentials**:
   - Copy the **Client ID**
   - Copy the **Client Secret**
   - You'll need these for the environment variables

---

## Environment Configuration

### Step 1: Generate Cookie Encryption Keys

These keys are used to securely encrypt session cookies:

```bash
# Generate COOKIE_HASH_KEY (64 bytes = 128 hex characters)
openssl rand -hex 64

# Generate COOKIE_BLOCK_KEY (32 bytes = 64 hex characters)
openssl rand -hex 32
```

**Important**: Save these keys! If you regenerate them, all existing user sessions will be invalidated.

### Step 2: Create Environment File

Create a `.env` file in the project root (or set environment variables):

```bash
# Google OAuth Configuration
GOOGLE_CLIENT_ID=your-client-id-here.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=your-client-secret-here
GOOGLE_REDIRECT_URL=http://localhost:8080/auth/google/callback

# Optional: Admin user (use your Google account email)
GOOGLE_ADMIN_ID=your-email@gmail.com

# Cookie encryption keys (generate with openssl as shown above)
COOKIE_HASH_KEY=your-128-character-hex-string-here
COOKIE_BLOCK_KEY=your-64-character-hex-string-here
```

**For production**, update `GOOGLE_REDIRECT_URL` to use HTTPS:
```bash
GOOGLE_REDIRECT_URL=https://yourdomain.com/auth/google/callback
```

---

## Local Development

### Step 1: Install Dependencies

```bash
cd /path/to/walkassistant
go mod download
```

### Step 2: Set Environment Variables

Option A - Using `.env` file:
```bash
# Create .env file as shown above, then:
export $(cat .env | xargs)
```

Option B - Direct export:
```bash
export GOOGLE_CLIENT_ID="your-client-id"
export GOOGLE_CLIENT_SECRET="your-secret"
export GOOGLE_REDIRECT_URL="http://localhost:8080/auth/google/callback"
export COOKIE_HASH_KEY="your-hash-key"
export COOKIE_BLOCK_KEY="your-block-key"
```

### Step 3: Run the Server

```bash
cd backend
go run main.go
```

The server will start on `http://localhost:8080`

### Step 4: Test Authentication

1. Open http://localhost:8080 in your browser
2. You should see a "Please login" message
3. Click "Login with Google"
4. Authorize the application
5. You should be redirected back and see the main app

---

## Production Deployment

### Option 1: Docker Compose (Recommended)

1. **Create `.env` file** in `deployments/` directory:

```bash
# deployments/.env
GOOGLE_CLIENT_ID=your-production-client-id
GOOGLE_CLIENT_SECRET=your-production-secret
GOOGLE_REDIRECT_URL=https://yourdomain.com/auth/google/callback
COOKIE_HASH_KEY=your-production-hash-key
COOKIE_BLOCK_KEY=your-production-block-key

# Optional
GOOGLE_ADMIN_ID=admin@example.com

# Docker/Traefik configuration
DNS_NAME=yourdomain.com
NETWORK_NAME=traefik
```

2. **Deploy**:

```bash
cd deployments
docker-compose up -d
```

### Option 2: Manual Deployment

1. **Build the application**:

```bash
cd backend
go build -o walkassistant main.go
```

2. **Set environment variables** on your server

3. **Run the binary**:

```bash
./walkassistant
```

4. **Set up reverse proxy** (nginx, Caddy, or Traefik) with HTTPS

---

## Database

### Location

- Local development: `backend/data/walkassistant.db`
- Docker deployment: Stored in the `walkassistant-data` volume

### Schema

The SQLite database contains two tables:

**users**:
```sql
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    google_id TEXT UNIQUE NOT NULL
);
```

**routes**:
```sql
CREATE TABLE routes (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    filename TEXT NOT NULL,
    track_points TEXT NOT NULL,  -- JSON array
    distance REAL NOT NULL,
    duration REAL NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
```

### GPX File Storage

- Local development: `backend/data/users/{user_id}/`
- Docker deployment: `/app/data/users/{user_id}/` (inside container)

Each user's GPX files are stored in their own directory, isolated from other users.

### Backup

To backup your data:

```bash
# Backup database
cp backend/data/walkassistant.db walkassistant.db.backup

# Backup GPX files
tar -czf gpx-files-backup.tar.gz backend/data/users/
```

For Docker:
```bash
docker exec walkassistant tar -czf /tmp/backup.tar.gz /app/data
docker cp walkassistant:/tmp/backup.tar.gz ./walkassistant-backup.tar.gz
```

---

## Security Notes

### Cookie Security

- **HttpOnly**: Cookies cannot be accessed via JavaScript
- **Secure flag**: In production with HTTPS, cookies are only sent over encrypted connections
- **AES-256 encryption**: Cookie values are encrypted using the `COOKIE_BLOCK_KEY`
- **HMAC authentication**: Cookie integrity is verified using the `COOKIE_HASH_KEY`

### Best Practices

1. **Never commit** `.env` files or secrets to version control
2. **Use strong keys**: Always generate random keys with `openssl rand`
3. **HTTPS in production**: Always use HTTPS for production deployments
4. **Rotate keys periodically**: Consider rotating cookie keys every 6-12 months
5. **Backup regularly**: Automate database and file backups
6. **Monitor logs**: Check logs for authentication errors or suspicious activity

### Key Rotation

If you need to rotate cookie keys:

1. Generate new keys
2. Update environment variables with new keys
3. Restart the application
4. All users will need to log in again

### User Data Privacy

- Each user can only access their own routes
- GPX files are stored in user-specific directories
- Database queries are filtered by user ID
- No cross-user data leakage

---

## Troubleshooting

### "Google login is not configured"

**Cause**: Environment variables are not set correctly

**Solution**: 
1. Verify `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, and `GOOGLE_REDIRECT_URL` are set
2. Check for typos in environment variable names
3. Restart the application after setting variables

### "Redirect URI mismatch" error from Google

**Cause**: The redirect URL doesn't match what's configured in Google Cloud Console

**Solution**:
1. Check that `GOOGLE_REDIRECT_URL` exactly matches the authorized redirect URI in Google Cloud Console
2. Ensure the protocol (http/https), domain, port, and path all match exactly
3. For local dev, use `http://localhost:8080/auth/google/callback`
4. For production, use `https://yourdomain.com/auth/google/callback`

### "Invalid session cookie" errors

**Cause**: Cookie keys were changed or are incorrect

**Solution**:
1. Verify `COOKIE_HASH_KEY` and `COOKIE_BLOCK_KEY` are set
2. If you rotated keys, all users need to log in again
3. Clear browser cookies and try logging in again

### Routes not showing after login

**Cause**: Database initialization issues or file permissions

**Solution**:
1. Check that the `data/` directory exists and is writable
2. Check application logs for database errors
3. Verify that `data/users/{user_id}/` directories are created
4. Check file permissions

### Cannot upload GPX files

**Cause**: Authentication issues or file permissions

**Solution**:
1. Verify you're logged in (check for "Logged in" in header)
2. Check browser console for errors
3. Verify `data/users/` directory permissions
4. Check server logs for upload errors

---

## Support

For issues or questions:
- GitHub Issues: https://github.com/korjavin/walkassistant/issues
- Check server logs for detailed error messages
- Verify all environment variables are set correctly

---

## Summary Checklist

- [ ] Created Google Cloud project
- [ ] Enabled Google+ API
- [ ] Created OAuth 2.0 credentials
- [ ] Generated cookie encryption keys
- [ ] Created `.env` file with all required variables
- [ ] Updated authorized redirect URIs in Google Cloud Console
- [ ] Tested local authentication flow
- [ ] (Production) Set up HTTPS
- [ ] (Production) Configured production redirect URL
- [ ] (Production) Set up automated backups

---

**Congratulations!** Your Walk Assistant instance is now secured with Google OAuth authentication, and each user's routes are private and isolated.
