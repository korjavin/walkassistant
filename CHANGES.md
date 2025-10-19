# Walk Assistant - Authentication & Privacy Update

## Summary

Walk Assistant has been updated to include **Google OAuth authentication** and **user-specific route privacy**. Each user now has their own isolated storage, and routes are completely private to the authenticated user.

## What Changed

### 🔐 Security & Authentication

1. **Google OAuth Integration**
   - Users must log in with their Google account to use the application
   - Secure session management with encrypted cookies (AES-256)
   - OAuth 2.0 flow with CSRF protection

2. **User Privacy**
   - Each user sees only their own routes
   - GPX files are stored in user-specific directories: `data/users/{user_id}/`
   - Database queries are filtered by user ID
   - Complete data isolation between users

### 💾 Database

1. **SQLite Storage**
   - Migrated from in-memory storage to persistent SQLite database
   - Database file: `data/walkassistant.db`
   - Pure Go SQLite implementation (`modernc.org/sqlite`) - no CGO required

2. **Schema**
   - `users` table: Stores user information (ID and Google ID)
   - `routes` table: Stores route data with user association
   - Foreign key constraints ensure data integrity

### 🎨 Frontend Updates

1. **Authentication UI**
   - Login prompt for unauthenticated users
   - "Login with Google" button
   - Logout button in header
   - User status indicator
   - App content hidden until authenticated

2. **Enhanced UX**
   - Routes load automatically after login
   - Clear feedback on authentication status
   - Smooth redirect flow

### 🏗️ Backend Changes

1. **New Package Structure**
   - `backend/pkg/storage/` - Storage abstraction layer
   - `storage.go` - Interface definition
   - `sqlite.go` - SQLite implementation

2. **Updated Handlers**
   - All endpoints now require authentication
   - User context passed through middleware
   - Routes filtered by authenticated user

3. **Route Generation**
   - All existing algorithms preserved
   - Functions updated to accept user routes as parameters
   - OSRM integration unchanged
   - Polyline decoding unchanged

### 📦 Dependencies Added

```go
github.com/google/uuid v1.6.0
github.com/gorilla/securecookie v1.1.2
golang.org/x/oauth2 v0.30.0
google.golang.org/api v0.248.0
modernc.org/sqlite v1.34.4
```

### 🐳 Deployment Updates

1. **Docker Compose**
   - Added environment variables for OAuth configuration
   - Added cookie encryption keys
   - Updated documentation

2. **Environment Variables Required**
   - `GOOGLE_CLIENT_ID` - OAuth client ID
   - `GOOGLE_CLIENT_SECRET` - OAuth client secret
   - `GOOGLE_REDIRECT_URL` - OAuth callback URL
   - `COOKIE_HASH_KEY` - Cookie encryption key (64 bytes)
   - `COOKIE_BLOCK_KEY` - Cookie encryption key (32 bytes)
   - `GOOGLE_ADMIN_ID` (optional) - Admin user

## Files Modified

### New Files
- ✅ `backend/pkg/storage/storage.go` - Storage interface
- ✅ `backend/pkg/storage/sqlite.go` - SQLite implementation
- ✅ `.env.example` - Environment configuration template
- ✅ `SETUP.md` - Detailed setup guide
- ✅ `CHANGES.md` - This file

### Modified Files
- ✅ `backend/main.go` - Complete rewrite with OAuth integration
- ✅ `backend/main.go.backup` - Backup of original
- ✅ `frontend/index.html` - Added login UI
- ✅ `frontend/js/app.js` - Added authentication logic
- ✅ `deployments/docker-compose.yml` - Added environment variables
- ✅ `go.mod` - Added new dependencies
- ✅ `README.md` - Updated with authentication info

### Unchanged Files
- ✅ `backend/min_distance_route.go` - Preserved (algorithms remain)
- ✅ `frontend/css/style.css` - No changes needed
- ✅ All route generation algorithms preserved

## Migration Guide

### For Existing Installations

If you're upgrading from a previous version:

1. **Backup your data**:
   ```bash
   cp -r data/ data.backup/
   ```

2. **Set up Google OAuth** (see SETUP.md):
   - Create Google Cloud project
   - Configure OAuth credentials
   - Generate cookie encryption keys

3. **Set environment variables**:
   ```bash
   export GOOGLE_CLIENT_ID="your-client-id"
   export GOOGLE_CLIENT_SECRET="your-secret"
   export GOOGLE_REDIRECT_URL="http://localhost:8080/auth/google/callback"
   export COOKIE_HASH_KEY=$(openssl rand -hex 64)
   export COOKIE_BLOCK_KEY=$(openssl rand -hex 32)
   ```

4. **Migrate existing GPX files**:
   ```bash
   # The app will auto-load files from data/*.gpx on startup
   # They will be associated with the first user who logs in
   
   # Or manually organize by user:
   mkdir -p data/users/your-user-id/
   mv data/*.gpx data/users/your-user-id/
   ```

5. **Rebuild and restart**:
   ```bash
   go mod tidy
   go build -o walkassistant ./backend
   ./walkassistant
   ```

### For New Installations

Follow the setup guide in [SETUP.md](SETUP.md)

## Testing Checklist

- [ ] Set up Google OAuth credentials
- [ ] Set all required environment variables
- [ ] Start the application
- [ ] Login with Google account
- [ ] Upload a GPX file
- [ ] Verify route appears on map
- [ ] Generate suggested routes
- [ ] Logout and verify app is hidden
- [ ] Login again and verify routes persist
- [ ] Try with a different Google account - verify route isolation

## Troubleshooting

See [SETUP.md](SETUP.md#troubleshooting) for common issues and solutions.

## Breaking Changes

⚠️ **Authentication Required**: The application now requires Google OAuth authentication. Anonymous usage is no longer supported.

⚠️ **Environment Variables**: You must configure OAuth credentials before running the application.

⚠️ **Data Migration**: Existing GPX files need to be associated with a user. See migration guide above.

## Backward Compatibility

- ✅ All route generation algorithms unchanged
- ✅ GPX file format unchanged
- ✅ Map visualization unchanged
- ✅ Distance filtering unchanged
- ✅ OSRM integration unchanged

## Security Considerations

1. **Session Security**
   - HttpOnly cookies prevent XSS attacks
   - Secure flag ensures HTTPS-only transmission (production)
   - AES-256 encryption protects cookie values
   - HMAC authentication prevents tampering

2. **OAuth Security**
   - State parameter prevents CSRF attacks
   - Client secret never exposed to client
   - Tokens stored server-side only

3. **Data Isolation**
   - Database constraints enforce user ownership
   - File system isolation with user directories
   - No cross-user queries possible

## Performance Impact

- **Minimal overhead**: Database queries are indexed and fast
- **No significant latency**: OAuth flow only on login
- **Efficient storage**: SQLite is lightweight and performant
- **Same route generation**: All algorithms run at the same speed

## Future Enhancements

Potential future improvements:
- [ ] Route sharing between users (optional)
- [ ] Export user data
- [ ] Email notifications for new routes
- [ ] Social features (optional)
- [ ] Additional OAuth providers (GitHub, etc.)

## Support

- 📖 Full setup guide: [SETUP.md](SETUP.md)
- 📝 Updated README: [README.md](README.md)
- 🐛 Issues: https://github.com/korjavin/walkassistant/issues

---

**Version**: 2.0.0 (Authentication Update)  
**Date**: 2025-10-19  
**Author**: Walk Assistant Team
