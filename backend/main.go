package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	mrand "math/rand"

	"github.com/gorilla/securecookie"
	"github.com/korjavin/walkassistant/backend/pkg/storage"
	"github.com/tkrajina/gpxgo/gpx"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	oauth2v2 "google.golang.org/api/oauth2/v2"
)

// contextKey defines a type for context keys to avoid collisions
type contextKey string

const (
	userContextKey = contextKey("userID")
	cookieName     = "user-session"
)

// SuggestedRoute represents a suggested new route
type SuggestedRoute struct {
	Points         []storage.TrackPoint `json:"points"`
	Distance       float64              `json:"distance"`
	FollowsStreets bool                 `json:"followsStreets"`
}

// OSRMResponse represents the response from the OSRM API
type OSRMResponse struct {
	Code   string `json:"code"`
	Routes []struct {
		Geometry string  `json:"geometry"`
		Distance float64 `json:"distance"`
		Duration float64 `json:"duration"`
	} `json:"routes"`
	Waypoints []struct {
		Location []float64 `json:"location"`
	} `json:"waypoints"`
}

// Global variables
var (
	db                *storage.SQLiteStorage
	googleOauthConfig *oauth2.Config
	oauthStateString  string
	sc                *securecookie.SecureCookie
)

func main() {
	var err error

	// Initialize database
	os.MkdirAll("data", os.ModePerm)
	db, err = storage.NewSQLiteStorage("data/walkassistant.db")
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer db.Close()

	// Initialize OAuth and secure cookies
	initOAuth()
	initSecureCookie()

	// Set up HTTP handlers
	http.HandleFunc("/auth/google/login", handleGoogleLogin)
	http.HandleFunc("/auth/google/callback", handleGoogleCallback)
	http.HandleFunc("/api/auth/status", handleAuthStatus)
	http.HandleFunc("/auth/logout", handleLogout)

	// Protected endpoints
	http.HandleFunc("/upload", withAuth(uploadHandler))
	http.HandleFunc("/routes", withAuth(routesHandler))
	http.HandleFunc("/routes/delete", withAuth(deleteRouteHandler))
	http.HandleFunc("/suggest", withAuth(suggestHandler))

	// Serve static files
	fs := http.FileServer(http.Dir("./frontend"))
	http.Handle("/", fs)

	fmt.Println("Starting server at port 8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

func initOAuth() {
	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	googleClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	redirectURL := os.Getenv("GOOGLE_REDIRECT_URL")

	if googleClientID == "" || googleClientSecret == "" || redirectURL == "" {
		log.Println("Warning: GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, or GOOGLE_REDIRECT_URL not set. Google login will be disabled.")
		googleOauthConfig = nil
		return
	}

	b := make([]byte, 16)
	rand.Read(b)
	oauthStateString = base64.URLEncoding.EncodeToString(b)

	googleOauthConfig = &oauth2.Config{
		RedirectURL:  redirectURL,
		ClientID:     googleClientID,
		ClientSecret: googleClientSecret,
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/userinfo.profile"},
		Endpoint:     google.Endpoint,
	}
	log.Println("Google OAuth initialized.")
}

func initSecureCookie() {
	hashKey := os.Getenv("COOKIE_HASH_KEY")
	blockKey := os.Getenv("COOKIE_BLOCK_KEY")

	if hashKey == "" || blockKey == "" {
		log.Println("Warning: COOKIE_HASH_KEY or COOKIE_BLOCK_KEY not set. Generating random keys for this session.")
		log.Println("For production, these should be set to persistent, randomly generated values.")
		sc = securecookie.New(securecookie.GenerateRandomKey(64), securecookie.GenerateRandomKey(32))
	} else {
		if len(blockKey) != 32 {
			log.Fatalf("Error: COOKIE_BLOCK_KEY must be 32 bytes long for AES-256. Got %d bytes.", len(blockKey))
		}
		sc = securecookie.New([]byte(hashKey), []byte(blockKey))
	}
}

// withAuth is a middleware that checks for a valid user session cookie
func withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(cookieName)
		if err != nil {
			http.Error(w, "Unauthorized: No session cookie provided.", http.StatusUnauthorized)
			return
		}

		var userID string
		if err = sc.Decode(cookieName, cookie.Value, &userID); err != nil {
			log.Printf("Invalid cookie received: %v", err)
			http.Error(w, "Unauthorized: Invalid session cookie.", http.StatusUnauthorized)
			return
		}

		// Add user ID to the request context
		ctx := context.WithValue(r.Context(), userContextKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

func getUserIDFromRequest(r *http.Request) string {
	if userID, ok := r.Context().Value(userContextKey).(string); ok {
		return userID
	}
	return ""
}

func handleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	if googleOauthConfig == nil {
		http.Error(w, "Google login is not configured", http.StatusInternalServerError)
		return
	}
	url := googleOauthConfig.AuthCodeURL(oauthStateString)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	if googleOauthConfig == nil {
		http.Error(w, "Google login is not configured", http.StatusInternalServerError)
		return
	}

	state := r.FormValue("state")
	if state != oauthStateString {
		log.Printf("Invalid oauth state, expected '%s', got '%s'\n", oauthStateString, state)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	code := r.FormValue("code")
	token, err := googleOauthConfig.Exchange(context.Background(), code)
	if err != nil {
		log.Printf("oauthConf.Exchange() failed with '%s'\n", err)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	oauth2Client := oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(token))
	oauth2Service, err := oauth2v2.New(oauth2Client)
	if err != nil {
		log.Printf("Unable to create oauth2 service: %v", err)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	userinfo, err := oauth2Service.Userinfo.Get().Do()
	if err != nil {
		log.Printf("Unable to get user info: %v", err)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	// Get or create user in database
	user, err := db.GetUserByGoogleID(userinfo.Id)
	if err != nil {
		log.Printf("Unable to get user by google ID: %v", err)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}
	if user == nil {
		user, err = db.CreateUser(userinfo.Id)
		if err != nil {
			log.Printf("Unable to create user: %v", err)
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
			return
		}
	}

	encoded, err := sc.Encode(cookieName, user.ID)
	if err != nil {
		log.Printf("Failed to encode cookie: %v", err)
		http.Error(w, "Failed to set session cookie", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    encoded,
		HttpOnly: true,
		Secure:   r.URL.Scheme == "https",
		Path:     "/",
		Expires:  time.Now().Add(30 * 24 * time.Hour), // 30 days
	})

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{"logged_in": false})
		return
	}

	var userID string
	if err = sc.Decode(cookieName, cookie.Value, &userID); err != nil {
		json.NewEncoder(w).Encode(map[string]any{"logged_in": false})
		return
	}

	json.NewEncoder(w).Encode(map[string]any{"logged_in": true, "user_id": userID})
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		HttpOnly: true,
		Path:     "/",
		Expires:  time.Unix(0, 0), // Expire immediately
	})
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	userID := getUserIDFromRequest(r)

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the multipart form
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	// Get the file from the form
	file, handler, err := r.FormFile("gpxfile")
	if err != nil {
		http.Error(w, "Unable to get file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Check if file is a GPX file
	if !strings.HasSuffix(strings.ToLower(handler.Filename), ".gpx") {
		http.Error(w, "File must be a GPX file", http.StatusBadRequest)
		return
	}

	// Create temporary file for parsing (will be deleted after)
	tempFile, err := os.CreateTemp("", "gpx-*.gpx")
	if err != nil {
		log.Printf("Error creating temp file: %v", err)
		http.Error(w, "Unable to process file", http.StatusInternalServerError)
		return
	}
	tempFilePath := tempFile.Name()
	defer os.Remove(tempFilePath) // Delete temp file when done

	// Copy uploaded file to temp file
	_, err = io.Copy(tempFile, file)
	tempFile.Close()
	if err != nil {
		log.Printf("Error writing temp file: %v", err)
		http.Error(w, "Unable to process file", http.StatusInternalServerError)
		return
	}

	// Parse the GPX file from temp location
	gpxFile, err := os.Open(tempFilePath)
	if err != nil {
		log.Printf("Error opening temp file: %v", err)
		http.Error(w, "Unable to process file", http.StatusInternalServerError)
		return
	}
	defer gpxFile.Close()

	gpxData, err := gpx.Parse(gpxFile)
	if err != nil {
		log.Printf("Error parsing GPX file: %v", err)
		http.Error(w, "Unable to parse GPX file", http.StatusInternalServerError)
		return
	}

	// Process and store the route data
	route, err := processGPXData(handler.Filename, gpxData)
	if err != nil {
		http.Error(w, "Unable to process GPX data", http.StatusInternalServerError)
		return
	}

	// Set user ID and save to database
	route.UserID = userID
	if err := db.CreateRoute(&route); err != nil {
		log.Printf("Error saving route to database: %v", err)
		http.Error(w, "Unable to save route", http.StatusInternalServerError)
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": fmt.Sprintf("File uploaded and processed successfully: %s", handler.Filename),
	})
}

func processGPXData(filename string, gpxData *gpx.GPX) (storage.RouteData, error) {
	var route storage.RouteData
	route.Filename = filename

	// Process all tracks in the GPX file
	for _, track := range gpxData.Tracks {
		for _, segment := range track.Segments {
			for _, point := range segment.Points {
				route.TrackPoints = append(route.TrackPoints, storage.TrackPoint{
					Latitude:  point.Latitude,
					Longitude: point.Longitude,
				})
			}
		}
	}

	// Calculate distance and duration if possible
	if len(gpxData.Tracks) > 0 && len(gpxData.Tracks[0].Segments) > 0 {
		// Calculate distance manually
		for _, track := range gpxData.Tracks {
			for _, segment := range track.Segments {
				for i := 0; i < len(segment.Points)-1; i++ {
					p1 := segment.Points[i]
					p2 := segment.Points[i+1]
					route.Distance += haversineDistance(
						p1.Latitude, p1.Longitude,
						p2.Latitude, p2.Longitude,
					)
				}
			}
		}

		// Calculate duration if timestamps are available
		if len(gpxData.Tracks[0].Segments) > 0 && len(gpxData.Tracks[0].Segments[0].Points) > 1 {
			firstPoint := gpxData.Tracks[0].Segments[0].Points[0]
			lastSegment := gpxData.Tracks[0].Segments[len(gpxData.Tracks[0].Segments)-1]
			lastPoint := lastSegment.Points[len(lastSegment.Points)-1]

			if !firstPoint.Timestamp.IsZero() && !lastPoint.Timestamp.IsZero() {
				route.Duration = lastPoint.Timestamp.Sub(firstPoint.Timestamp).Seconds()
			}
		}
	}

	return route, nil
}

// loadExistingGPXFiles - removed as we no longer store files
// All route data is stored in the database only

func routesHandler(w http.ResponseWriter, r *http.Request) {
	userID := getUserIDFromRequest(r)

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	routes, err := db.GetRoutesByUserID(userID)
	if err != nil {
		log.Printf("Error getting routes for user %s: %v", userID, err)
		http.Error(w, "Unable to get routes", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(routes)
}

func suggestHandler(w http.ResponseWriter, r *http.Request) {
	userID := getUserIDFromRequest(r)

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get query parameters for filtering
	minDistance := 0.0
	maxDistance := 0.0
	followStreets := true // Default to following streets
	usePOIs := false      // New parameter for POI-based suggestions
	var selectedRouteIDs []string

	if r.URL.Query().Get("minDistance") != "" {
		fmt.Sscanf(r.URL.Query().Get("minDistance"), "%f", &minDistance)
	}
	if r.URL.Query().Get("maxDistance") != "" {
		fmt.Sscanf(r.URL.Query().Get("maxDistance"), "%f", &maxDistance)
	}
	if r.URL.Query().Get("followStreets") == "false" {
		followStreets = false
	}
	if r.URL.Query().Get("usePOIs") == "true" {
		usePOIs = true
	}
	if r.URL.Query().Get("routeIds") != "" {
		selectedRouteIDs = strings.Split(r.URL.Query().Get("routeIds"), ",")
	}

	// Log the parameters for debugging
	log.Printf("Suggesting routes with parameters: minDistance=%f, maxDistance=%f, followStreets=%t, usePOIs=%t, routeIds=%v",
		minDistance, maxDistance, followStreets, usePOIs, selectedRouteIDs)

	// Get user's routes
	userRoutes, err := db.GetRoutesByUserID(userID)
	if err != nil {
		log.Printf("Error getting routes for user %s: %v", userID, err)
		http.Error(w, "Unable to get routes", http.StatusInternalServerError)
		return
	}

	// Filter routes by selected IDs if provided
	if len(selectedRouteIDs) > 0 {
		selectedMap := make(map[string]bool)
		for _, id := range selectedRouteIDs {
			selectedMap[id] = true
		}

		filteredRoutes := make([]*storage.RouteData, 0, len(userRoutes))
		for _, route := range userRoutes {
			if selectedMap[route.ID] {
				filteredRoutes = append(filteredRoutes, route)
			}
		}
		userRoutes = filteredRoutes
		log.Printf("Filtered to %d selected routes", len(userRoutes))
	}

	// Generate suggested routes
	var suggested []SuggestedRoute

	// Use POI-based suggestions if requested
	if usePOIs {
		log.Printf("Using POI-based route generation")
		suggested, err = generatePOIAnchoredRoute(userRoutes, minDistance, maxDistance)
		if err != nil {
			log.Printf("POI-based generation failed: %v, falling back to standard generation", err)
			// Fall back to standard generation if POI-based fails
			if minDistance > 0 && followStreets {
				suggested, err = generateRouteWithMinDistance(userRoutes, minDistance)
			} else {
				suggested, err = generateSuggestedRoutes(userRoutes, minDistance, maxDistance, followStreets)
			}
		}
	} else if minDistance > 0 && followStreets {
		// If we need a route with a minimum distance and following streets, use a specialized function
		log.Printf("Using specialized function to generate a route with minimum distance %f km that follows streets", minDistance)
		suggested, err = generateRouteWithMinDistance(userRoutes, minDistance)
	} else {
		suggested, err = generateSuggestedRoutes(userRoutes, minDistance, maxDistance, followStreets)
	}

	if err != nil {
		http.Error(w, "Unable to generate suggested routes", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(suggested)
}

func deleteRouteHandler(w http.ResponseWriter, r *http.Request) {
	userID := getUserIDFromRequest(r)

	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get route ID from query parameter
	routeID := r.URL.Query().Get("id")
	if routeID == "" {
		http.Error(w, "Route ID is required", http.StatusBadRequest)
		return
	}

	// Verify the route belongs to this user
	route, err := db.GetRouteByID(routeID)
	if err != nil {
		log.Printf("Error getting route %s: %v", routeID, err)
		http.Error(w, "Route not found", http.StatusNotFound)
		return
	}

	if route.UserID != userID {
		log.Printf("User %s attempted to delete route %s owned by %s", userID, routeID, route.UserID)
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	// Delete from database
	if err := db.DeleteRoute(routeID); err != nil {
		log.Printf("Error deleting route %s: %v", routeID, err)
		http.Error(w, "Unable to delete route", http.StatusInternalServerError)
		return
	}

	log.Printf("User %s deleted route %s (%s)", userID, routeID, route.Filename)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Route deleted successfully",
		"id":      routeID,
	})
}

// filterOutlierRoutes removes routes that are geographically distant from the majority
// This prevents issues when users have routes in different cities/countries
func filterOutlierRoutes(routes []*storage.RouteData) []*storage.RouteData {
	if len(routes) <= 1 {
		return routes
	}

	// Calculate the center point of each route
	type routeCenter struct {
		route *storage.RouteData
		lat   float64
		lng   float64
	}

	centers := make([]routeCenter, 0, len(routes))
	for _, route := range routes {
		if len(route.TrackPoints) == 0 {
			continue
		}

		// Calculate average position
		var sumLat, sumLng float64
		for _, point := range route.TrackPoints {
			sumLat += point.Latitude
			sumLng += point.Longitude
		}
		centers = append(centers, routeCenter{
			route: route,
			lat:   sumLat / float64(len(route.TrackPoints)),
			lng:   sumLng / float64(len(route.TrackPoints)),
		})
	}

	if len(centers) == 0 {
		return routes
	}

	// Find the main cluster by calculating median center
	// Then filter out routes that are more than 50km from the median
	var medianLat, medianLng float64
	for _, c := range centers {
		medianLat += c.lat
		medianLng += c.lng
	}
	medianLat /= float64(len(centers))
	medianLng /= float64(len(centers))

	// Filter routes within 50km of the median center
	const maxDistanceKm = 50.0
	filtered := make([]*storage.RouteData, 0, len(routes))
	for _, c := range centers {
		distance := haversineDistance(c.lat, c.lng, medianLat, medianLng)
		if distance <= maxDistanceKm {
			filtered = append(filtered, c.route)
		} else {
			log.Printf("Filtering out outlier route '%s': %.1f km from center", c.route.Filename, distance)
		}
	}

	return filtered
}

func generateSuggestedRoutes(userRoutes []*storage.RouteData, minDistance, maxDistance float64, followStreets bool) ([]SuggestedRoute, error) {
	// If no existing routes, return empty suggestions
	if len(userRoutes) == 0 {
		return []SuggestedRoute{}, nil
	}

	// Filter out geographic outliers (routes in different cities/countries)
	filteredRoutes := filterOutlierRoutes(userRoutes)
	if len(filteredRoutes) == 0 {
		// If all routes were filtered as outliers, use all routes
		filteredRoutes = userRoutes
	}

	// Create a grid of the area covered by existing routes
	var minLat, maxLat, minLng, maxLng float64
	var allPoints []storage.TrackPoint

	// Find the bounding box of all existing routes
	for i, route := range filteredRoutes {
		for j, point := range route.TrackPoints {
			allPoints = append(allPoints, point)

			// Initialize min/max on first point
			if i == 0 && j == 0 {
				minLat, maxLat = point.Latitude, point.Latitude
				minLng, maxLng = point.Longitude, point.Longitude
				continue
			}

			// Update min/max values
			if point.Latitude < minLat {
				minLat = point.Latitude
			} else if point.Latitude > maxLat {
				maxLat = point.Latitude
			}

			if point.Longitude < minLng {
				minLng = point.Longitude
			} else if point.Longitude > maxLng {
				maxLng = point.Longitude
			}
		}
	}

	// Add some random variation to the bounding box (up to 10% of the size)
	latRange := maxLat - minLat
	lngRange := maxLng - minLng

	// Random variation between -5% and +5%
	minLatVar := minLat + (mrand.Float64()*0.1-0.05)*latRange
	minLngVar := minLng + (mrand.Float64()*0.1-0.05)*lngRange
	maxLatVar := maxLat + (mrand.Float64()*0.1-0.05)*latRange
	maxLngVar := maxLng + (mrand.Float64()*0.1-0.05)*lngRange

	// Create a perimeter with the randomized points
	perimeter := []storage.TrackPoint{
		{Latitude: minLatVar, Longitude: minLngVar},
		{Latitude: minLatVar, Longitude: maxLngVar},
		{Latitude: maxLatVar, Longitude: maxLngVar},
		{Latitude: maxLatVar, Longitude: minLngVar},
		{Latitude: minLatVar, Longitude: minLngVar},
	}

	// Calculate approximate distance of the suggested route
	distance := calculateRouteDistance(perimeter)

	// Apply distance filters if specified
	if maxDistance > 0 && distance > maxDistance {
		log.Printf("Route exceeds max distance, scaling down from %f km to %f km", distance, maxDistance)
		scaleFactor := maxDistance / distance
		log.Printf("Using scale factor: %f for perimeter route", scaleFactor)
		perimeter = adjustRouteDistance(perimeter, scaleFactor)
		distance = calculateRouteDistance(perimeter)
		log.Printf("After scaling, perimeter route distance is now: %f km", distance)
	} else if minDistance > 0 && distance < minDistance {
		log.Printf("Route is shorter than min distance, extending from %f km to %f km", distance, minDistance)
		perimeter = extendRoute(perimeter, minDistance/distance)
		distance = calculateRouteDistance(perimeter)
		log.Printf("After extending, route distance is now: %f km", distance)
	}

	// Create the suggested route
	suggestedRoute := SuggestedRoute{
		Points:         perimeter,
		Distance:       distance,
		FollowsStreets: false,
	}

	// Log the initial route distance for debugging
	log.Printf("Initial route distance: %f km, max distance: %f km", distance, maxDistance)

	// If followStreets is true, try to get a route that follows streets
	log.Printf("Attempting to create a route that follows streets (followStreets=%t)", followStreets)
	if followStreets {
		streetRoute, err := getRouteFollowingStreets(perimeter)
		if err == nil {
			// Verify that the street route is within a reasonable distance of the existing routes
			if isRouteNearExistingRoutes(streetRoute.Points, minLat, maxLat, minLng, maxLng) {
				// Check if the street route meets the distance criteria
				streetDistance := streetRoute.Distance
				log.Printf("Street route distance from OSRM: %f km, max distance: %f km", streetDistance, maxDistance)

				// Make sure we have a valid distance
				if streetDistance < 0.1 {
					log.Printf("WARNING: Street route distance is too small (%f km), using estimated distance", streetDistance)

					// Calculate the bounding box of the points to estimate a reasonable distance
					var minLat, maxLat, minLng, maxLng float64
					for i, point := range streetRoute.Points {
						if i == 0 {
							minLat, maxLat = point.Latitude, point.Latitude
							minLng, maxLng = point.Longitude, point.Longitude
							continue
						}
						if point.Latitude < minLat {
							minLat = point.Latitude
						} else if point.Latitude > maxLat {
							maxLat = point.Latitude
						}
						if point.Longitude < minLng {
							minLng = point.Longitude
						} else if point.Longitude > maxLng {
							maxLng = point.Longitude
						}
					}

					// Estimate the perimeter of the bounding box
					width := haversineDistance(minLat, minLng, minLat, maxLng)
					height := haversineDistance(minLat, minLng, maxLat, minLng)
					estimatedDistance := 2 * (width + height)

					streetDistance = estimatedDistance
					streetRoute.Distance = streetDistance
					log.Printf("Using estimated street route distance: %f km", streetDistance)
				}

				if maxDistance > 0 && streetDistance > maxDistance {
					log.Printf("Street route exceeds max distance (%f km), scaling down to %f km", streetDistance, maxDistance)

					// Calculate the center of the perimeter
					var centerLat, centerLng float64
					for _, p := range perimeter {
						centerLat += p.Latitude
						centerLng += p.Longitude
					}
					centerLat /= float64(len(perimeter))
					centerLng /= float64(len(perimeter))

					// Create a smaller perimeter by scaling points toward the center
					percentage := maxDistance / streetDistance
					scaleFactor := percentage * 0.8
					log.Printf("Using scale factor %.4f to create smaller perimeter", scaleFactor)

					var scaledPoints []storage.TrackPoint
					for _, p := range perimeter {
						newLat := centerLat + (p.Latitude-centerLat)*scaleFactor
						newLng := centerLng + (p.Longitude-centerLng)*scaleFactor
						scaledPoints = append(scaledPoints, storage.TrackPoint{Latitude: newLat, Longitude: newLng})
					}

					// Get a new street route based on scaled perimeter
					newStreetRoute, err := getRouteFollowingStreets(scaledPoints)
					if err == nil && newStreetRoute.Distance <= maxDistance*1.1 {
						streetRoute = newStreetRoute
						log.Printf("Successfully created a street route within max distance")
					} else {
						// Fall back to mathematical scaling
						log.Printf("Falling back to scaled route")
						streetRoute.Points = adjustRouteDistance(streetRoute.Points, maxDistance/streetDistance)
						streetRoute.Distance = calculateRouteDistance(streetRoute.Points)
					}
				} else if minDistance > 0 && streetDistance < minDistance {
					log.Printf("Street route is shorter than min distance (%f km), extending to %f km", streetDistance, minDistance)

					// Calculate center of existing routes
					var centerLat, centerLng float64
					totalPoints := 0

					for _, route := range userRoutes {
						for _, point := range route.TrackPoints {
							centerLat += point.Latitude
							centerLng += point.Longitude
							totalPoints++
						}
					}

					if totalPoints > 0 {
						centerLat /= float64(totalPoints)
						centerLng /= float64(totalPoints)
					} else {
						for _, p := range perimeter {
							centerLat += p.Latitude
							centerLng += p.Longitude
						}
						centerLat /= float64(len(perimeter))
						centerLng /= float64(len(perimeter))
					}

					// Create a polygon around the center point
					offset := math.Sqrt(minDistance/10.0) / 111.0
					numPoints := 5
					var polygonPoints []storage.TrackPoint

					for i := 0; i < numPoints; i++ {
						angle := 2.0 * math.Pi * float64(i) / float64(numPoints)
						polygonPoints = append(polygonPoints, storage.TrackPoint{
							Latitude:  centerLat + offset*math.Sin(angle),
							Longitude: centerLng + offset*math.Cos(angle),
						})
					}
					polygonPoints = append(polygonPoints, polygonPoints[0])

					newStreetRoute, err := getRouteFollowingStreets(polygonPoints)
					if err == nil && newStreetRoute.Distance >= minDistance {
						streetRoute = newStreetRoute
						log.Printf("Created longer street route with polygon: %f km", newStreetRoute.Distance)
					}
				}

				suggestedRoute.Points = streetRoute.Points
				suggestedRoute.Distance = streetRoute.Distance
				suggestedRoute.FollowsStreets = true
			} else {
				log.Printf("Street route is too far from existing routes, using perimeter route instead")
			}
		} else {
			log.Printf("Error getting street route: %v", err)
		}
	}

	log.Printf("FINAL ROUTE: Distance=%f km, FollowsStreets=%t, MaxDistance=%f km",
		suggestedRoute.Distance, suggestedRoute.FollowsStreets, maxDistance)

	return []SuggestedRoute{suggestedRoute}, nil
}

func generateRouteWithMinDistance(userRoutes []*storage.RouteData, minDistance float64) ([]SuggestedRoute, error) {
	// Filter out geographic outliers
	filteredRoutes := filterOutlierRoutes(userRoutes)
	if len(filteredRoutes) == 0 {
		filteredRoutes = userRoutes
	}

	// Find the bounding box of all existing routes
	var minLat, maxLat, minLng, maxLng float64
	hasPoints := false

	for _, route := range filteredRoutes {
		for _, point := range route.TrackPoints {
			if !hasPoints {
				minLat, maxLat = point.Latitude, point.Latitude
				minLng, maxLng = point.Longitude, point.Longitude
				hasPoints = true
				continue
			}

			if point.Latitude < minLat {
				minLat = point.Latitude
			} else if point.Latitude > maxLat {
				maxLat = point.Latitude
			}

			if point.Longitude < minLng {
				minLng = point.Longitude
			} else if point.Longitude > maxLng {
				maxLng = point.Longitude
			}
		}
	}

	// Calculate the center
	centerLat := (minLat + maxLat) / 2
	centerLng := (minLng + maxLng) / 2

	if minLat == 0 && maxLat == 0 {
		// Use a default location (Berlin, Germany)
		centerLat = 52.52
		centerLng = 13.405
	}

	log.Printf("Using center point: [%f, %f] to generate route with min distance %f km",
		centerLat, centerLng, minDistance)

	// Create a simple route with two points far enough apart
	offset := math.Sqrt(minDistance/2.0) / 111.0

	simplePoints := []storage.TrackPoint{
		{Latitude: centerLat - offset, Longitude: centerLng - offset},
		{Latitude: centerLat + offset, Longitude: centerLng + offset},
	}

	streetRoute, err := getRouteFollowingStreets(simplePoints)
	if err == nil && streetRoute.Distance >= minDistance {
		log.Printf("Created street route with distance: %f km", streetRoute.Distance)
		return []SuggestedRoute{streetRoute}, nil
	}

	// Try with larger offset
	log.Printf("First attempt failed, trying with larger offset")
	offset *= 2.0
	simplePoints = []storage.TrackPoint{
		{Latitude: centerLat - offset, Longitude: centerLng - offset},
		{Latitude: centerLat + offset, Longitude: centerLng + offset},
	}

	streetRoute, err = getRouteFollowingStreets(simplePoints)
	if err == nil && streetRoute.Distance >= minDistance {
		log.Printf("Created street route with larger offset: %f km", streetRoute.Distance)
		return []SuggestedRoute{streetRoute}, nil
	}

	// Try with polygon
	log.Printf("Simple route attempts failed, trying with polygon")
	numPoints := 4
	var polygonPoints []storage.TrackPoint

	for i := 0; i < numPoints; i++ {
		angle := 2.0 * math.Pi * float64(i) / float64(numPoints)
		polygonPoints = append(polygonPoints, storage.TrackPoint{
			Latitude:  centerLat + offset*math.Sin(angle),
			Longitude: centerLng + offset*math.Cos(angle),
		})
	}
	polygonPoints = append(polygonPoints, polygonPoints[0])

	streetRoute, err = getRouteFollowingStreets(polygonPoints)
	if err == nil && streetRoute.Distance >= minDistance {
		log.Printf("Created street route with polygon: %f km", streetRoute.Distance)
		return []SuggestedRoute{streetRoute}, nil
	}

	// Return simple route as fallback
	log.Printf("All attempts failed, returning simple route that doesn't follow streets")
	simpleRoute := SuggestedRoute{
		Points: []storage.TrackPoint{
			{Latitude: centerLat - offset, Longitude: centerLng - offset},
			{Latitude: centerLat + offset, Longitude: centerLng + offset},
		},
		Distance: calculateRouteDistance([]storage.TrackPoint{
			{Latitude: centerLat - offset, Longitude: centerLng - offset},
			{Latitude: centerLat + offset, Longitude: centerLng + offset},
		}),
		FollowsStreets: false,
	}

	return []SuggestedRoute{simpleRoute}, nil
}

func calculateRouteDistance(points []storage.TrackPoint) float64 {
	if len(points) < 2 {
		return 0
	}

	var distance float64
	for i := 0; i < len(points)-1; i++ {
		distance += haversineDistance(
			points[i].Latitude, points[i].Longitude,
			points[i+1].Latitude, points[i+1].Longitude,
		)
	}

	return distance
}

func haversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	if lat1 == lat2 && lon1 == lon2 {
		return 0
	}

	const R = 6371.0
	const PI = math.Pi
	lat1Rad := lat1 * (PI / 180)
	lat2Rad := lat2 * (PI / 180)
	lonDiff := (lon2 - lon1) * (PI / 180)
	latDiff := (lat2 - lat1) * (PI / 180)

	a := math.Sin(latDiff/2)*math.Sin(latDiff/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(lonDiff/2)*math.Sin(lonDiff/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	distance := R * c

	return distance
}

func adjustRouteDistance(points []storage.TrackPoint, scaleFactor float64) []storage.TrackPoint {
	adjustedPoints := make([]storage.TrackPoint, len(points))

	centroid := storage.TrackPoint{Latitude: 0, Longitude: 0}
	for _, p := range points {
		centroid.Latitude += p.Latitude
		centroid.Longitude += p.Longitude
	}
	centroid.Latitude /= float64(len(points))
	centroid.Longitude /= float64(len(points))

	for i, p := range points {
		adjustedPoints[i] = storage.TrackPoint{
			Latitude:  centroid.Latitude + (p.Latitude-centroid.Latitude)*scaleFactor,
			Longitude: centroid.Longitude + (p.Longitude-centroid.Longitude)*scaleFactor,
		}
	}

	return adjustedPoints
}

func getRouteFollowingStreets(points []storage.TrackPoint) (SuggestedRoute, error) {
	osrmServer := "https://router.project-osrm.org"

	if len(points) > 100 {
		log.Printf("Too many points (%d), sampling to reduce", len(points))
		sampledPoints := []storage.TrackPoint{}
		step := len(points) / 100
		if step < 1 {
			step = 1
		}

		for i := 0; i < len(points); i += step {
			sampledPoints = append(sampledPoints, points[i])
		}

		if len(sampledPoints) > 0 && sampledPoints[len(sampledPoints)-1] != points[len(points)-1] {
			sampledPoints = append(sampledPoints, points[len(points)-1])
		}

		points = sampledPoints
		log.Printf("Reduced to %d points", len(points))
	}

	log.Printf("Input points for street routing: %+v", points)

	var coordsBuilder strings.Builder
	for i, point := range points {
		if i > 0 {
			coordsBuilder.WriteString(";")
		}
		coordsBuilder.WriteString(fmt.Sprintf("%f,%f", point.Longitude, point.Latitude))
	}

	url := fmt.Sprintf("%s/route/v1/walking/%s?overview=full&geometries=polyline",
		osrmServer, coordsBuilder.String())

	log.Printf("OSRM API URL: %s", url)

	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Error making OSRM API request: %v", err)
		return SuggestedRoute{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading OSRM API response: %v", err)
		return SuggestedRoute{}, err
	}

	log.Printf("OSRM API response: %s", string(body))

	var osrmResp OSRMResponse
	if err := json.Unmarshal(body, &osrmResp); err != nil {
		log.Printf("Error parsing OSRM API response: %v", err)
		return SuggestedRoute{}, err
	}

	if osrmResp.Code != "Ok" || len(osrmResp.Routes) == 0 {
		log.Printf("OSRM API did not return a valid route: %s", osrmResp.Code)
		return SuggestedRoute{}, fmt.Errorf("OSRM API did not return a valid route")
	}

	decodedPoints := decodePolyline(osrmResp.Routes[0].Geometry)

	log.Printf("Decoded %d points from polyline", len(decodedPoints))
	if len(decodedPoints) > 0 {
		log.Printf("First point: %v, Last point: %v", decodedPoints[0], decodedPoints[len(decodedPoints)-1])
	}

	var trackPoints []storage.TrackPoint
	for _, point := range decodedPoints {
		trackPoint := storage.TrackPoint{
			Latitude:  point[0],
			Longitude: point[1],
		}
		trackPoints = append(trackPoints, trackPoint)
	}

	actualDistance := 0.0
	if len(trackPoints) >= 2 {
		actualDistance = calculateRouteDistance(trackPoints)
		log.Printf("Calculated street route distance: %f km with %d points", actualDistance, len(trackPoints))
	} else {
		log.Printf("WARNING: Not enough points to calculate distance. Only %d points available.", len(trackPoints))
	}

	if actualDistance < 0.1 && len(osrmResp.Routes) > 0 {
		actualDistance = osrmResp.Routes[0].Distance / 1000.0
		log.Printf("Using OSRM distance as fallback: %f km", actualDistance)

		if actualDistance < 0.1 {
			var minLat, maxLat, minLng, maxLng float64
			for i, point := range trackPoints {
				if i == 0 {
					minLat, maxLat = point.Latitude, point.Latitude
					minLng, maxLng = point.Longitude, point.Longitude
					continue
				}
				if point.Latitude < minLat {
					minLat = point.Latitude
				} else if point.Latitude > maxLat {
					maxLat = point.Latitude
				}
				if point.Longitude < minLng {
					minLng = point.Longitude
				} else if point.Longitude > maxLng {
					maxLng = point.Longitude
				}
			}

			width := haversineDistance(minLat, minLng, minLat, maxLng)
			height := haversineDistance(minLat, minLng, maxLat, minLng)
			estimatedDistance := 2 * (width + height)

			actualDistance = estimatedDistance
			log.Printf("Using estimated distance based on bounding box: %f km", actualDistance)
		}
	}

	return SuggestedRoute{
		Points:         trackPoints,
		Distance:       actualDistance,
		FollowsStreets: true,
	}, nil
}

func decodePolyline(polyline string) [][]float64 {
	var coordinates [][]float64
	index := 0
	lat, lng := 0, 0

	for index < len(polyline) {
		// Decode latitude
		latResult, latShift := 0, 0
		var b int

		for {
			if index >= len(polyline) {
				break
			}
			b = int(polyline[index]) - 63
			index++
			latResult |= (b & 0x1f) << latShift
			latShift += 5
			if b < 0x20 {
				break
			}
		}

		latChange := latResult
		if (latResult & 1) == 1 {
			latChange = ^(latResult >> 1)
		} else {
			latChange = latResult >> 1
		}

		lat += latChange

		// Decode longitude
		lngResult, lngShift := 0, 0

		for {
			if index >= len(polyline) {
				break
			}
			b = int(polyline[index]) - 63
			index++
			lngResult |= (b & 0x1f) << lngShift
			lngShift += 5
			if b < 0x20 {
				break
			}
		}

		lngChange := lngResult
		if (lngResult & 1) == 1 {
			lngChange = ^(lngResult >> 1)
		} else {
			lngChange = lngResult >> 1
		}

		lng += lngChange

		lat_f := float64(lat) / 1e5
		lng_f := float64(lng) / 1e5

		coordinates = append(coordinates, []float64{lat_f, lng_f})
	}

	return coordinates
}

func isRouteNearExistingRoutes(points []storage.TrackPoint, minLat, maxLat, minLng, maxLng float64) bool {
	latPadding := (maxLat - minLat) * 0.5
	lngPadding := (maxLng - minLng) * 0.5

	minLatWithPadding := minLat - latPadding
	maxLatWithPadding := maxLat + latPadding
	minLngWithPadding := minLng - lngPadding
	maxLngWithPadding := maxLng + lngPadding

	log.Printf("Existing routes bounding box with padding: [%f,%f,%f,%f]",
		minLatWithPadding, maxLatWithPadding, minLngWithPadding, maxLngWithPadding)

	pointsInBounds := 0
	for _, point := range points {
		if point.Latitude >= minLatWithPadding && point.Latitude <= maxLatWithPadding &&
			point.Longitude >= minLngWithPadding && point.Longitude <= maxLngWithPadding {
			pointsInBounds++
		}
	}

	percentageInBounds := float64(pointsInBounds) / float64(len(points))
	log.Printf("Percentage of points in bounds: %f%%", percentageInBounds*100)

	return percentageInBounds >= 0.5
}

func extendRoute(points []storage.TrackPoint, extensionFactor float64) []storage.TrackPoint {
	if len(points) <= 2 || extensionFactor <= 1.0 {
		return points
	}

	numZigzags := int(extensionFactor) - 1
	if numZigzags < 1 {
		numZigzags = 1
	}

	var newPoints []storage.TrackPoint

	for i := 0; i < len(points)-1; i++ {
		p1 := points[i]
		p2 := points[i+1]

		newPoints = append(newPoints, p1)

		midLat := (p1.Latitude + p2.Latitude) / 2
		midLng := (p1.Longitude + p2.Longitude) / 2

		dLat := p2.Latitude - p1.Latitude
		dLng := p2.Longitude - p1.Longitude

		length := math.Sqrt(dLat*dLat + dLng*dLng)
		if length > 0 {
			perpLat := -dLng / length * 0.01
			perpLng := dLat / length * 0.01

			for j := 0; j < numZigzags; j++ {
				direction := 1.0
				if j%2 == 1 {
					direction = -1.0
				}

				zigzagPoint := storage.TrackPoint{
					Latitude:  midLat + perpLat*direction,
					Longitude: midLng + perpLng*direction,
				}
				newPoints = append(newPoints, zigzagPoint)
			}
		}
	}

	newPoints = append(newPoints, points[len(points)-1])

	return newPoints
}

// POI represents a Point of Interest from OpenStreetMap
type POI struct {
	ID       int64              `json:"id"`
	Type     string             `json:"type"`
	Name     string             `json:"name"`
	Location storage.TrackPoint `json:"location"`
	Tags     map[string]string  `json:"tags"`
}

// OverpassResponse represents the response from Overpass API
type OverpassResponse struct {
	Elements []struct {
		ID   int64             `json:"id"`
		Type string            `json:"type"`
		Lat  float64           `json:"lat"`
		Lon  float64           `json:"lon"`
		Tags map[string]string `json:"tags"`
	} `json:"elements"`
}

// queryNearbyPOIs queries OpenStreetMap for interesting POIs within radius (in km)
func queryNearbyPOIs(lat, lng, radiusKm float64) ([]POI, error) {
	// Overpass API query for interesting POIs
	// We look for: parks, viewpoints, monuments, water features, gardens
	overpassQuery := fmt.Sprintf(`[out:json][timeout:25];
(
  node["leisure"="park"](around:%d,%f,%f);
  node["tourism"="viewpoint"](around:%d,%f,%f);
  node["tourism"="attraction"](around:%d,%f,%f);
  node["historic"="monument"](around:%d,%f,%f);
  node["natural"="water"](around:%d,%f,%f);
  node["leisure"="garden"](around:%d,%f,%f);
  node["amenity"="fountain"](around:%d,%f,%f);
  way["leisure"="park"](around:%d,%f,%f);
  way["leisure"="garden"](around:%d,%f,%f);
);
out center;`,
		int(radiusKm*1000), lat, lng,
		int(radiusKm*1000), lat, lng,
		int(radiusKm*1000), lat, lng,
		int(radiusKm*1000), lat, lng,
		int(radiusKm*1000), lat, lng,
		int(radiusKm*1000), lat, lng,
		int(radiusKm*1000), lat, lng,
		int(radiusKm*1000), lat, lng,
		int(radiusKm*1000), lat, lng,
	)

	overpassURL := "https://overpass-api.de/api/interpreter"

	resp, err := http.Post(overpassURL, "application/x-www-form-urlencoded", strings.NewReader("data="+overpassQuery))
	if err != nil {
		return nil, fmt.Errorf("failed to query Overpass API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Overpass API returned status %d", resp.StatusCode)
	}

	var overpassResp OverpassResponse
	if err := json.NewDecoder(resp.Body).Decode(&overpassResp); err != nil {
		return nil, fmt.Errorf("failed to decode Overpass response: %w", err)
	}

	// Convert to POI list
	pois := make([]POI, 0, len(overpassResp.Elements))
	for _, elem := range overpassResp.Elements {
		name := elem.Tags["name"]
		if name == "" {
			// Skip POIs without names
			continue
		}

		poiType := "unknown"
		if leisure, ok := elem.Tags["leisure"]; ok {
			poiType = leisure
		} else if tourism, ok := elem.Tags["tourism"]; ok {
			poiType = tourism
		} else if historic, ok := elem.Tags["historic"]; ok {
			poiType = historic
		} else if natural, ok := elem.Tags["natural"]; ok {
			poiType = natural
		}

		pois = append(pois, POI{
			ID:   elem.ID,
			Type: poiType,
			Name: name,
			Location: storage.TrackPoint{
				Latitude:  elem.Lat,
				Longitude: elem.Lon,
			},
			Tags: elem.Tags,
		})
	}

	log.Printf("Found %d POIs near [%f, %f] within %.1f km", len(pois), lat, lng, radiusKm)
	return pois, nil
}

// filterUnvisitedPOIs removes POIs that are close to existing route points
func filterUnvisitedPOIs(pois []POI, userRoutes []*storage.RouteData) []POI {
	const visitedThresholdKm = 0.1 // Consider POI visited if within 100m of any route point

	unvisited := make([]POI, 0)
	for _, poi := range pois {
		isVisited := false

		for _, route := range userRoutes {
			for _, point := range route.TrackPoints {
				dist := haversineDistance(poi.Location.Latitude, poi.Location.Longitude,
					point.Latitude, point.Longitude)
				if dist <= visitedThresholdKm {
					isVisited = true
					break
				}
			}
			if isVisited {
				break
			}
		}

		if !isVisited {
			unvisited = append(unvisited, poi)
		}
	}

	log.Printf("Filtered to %d unvisited POIs (out of %d total)", len(unvisited), len(pois))
	return unvisited
}

// findBestPOICombination finds combinations of POIs that would create a good route
func findBestPOICombination(pois []POI, startLat, startLng, targetDistance float64) [][]POI {
	if len(pois) == 0 {
		return nil
	}

	// Score each POI by distance from start
	type scoredPOI struct {
		poi  POI
		dist float64
	}

	scored := make([]scoredPOI, 0, len(pois))
	for _, poi := range pois {
		dist := haversineDistance(startLat, startLng, poi.Location.Latitude, poi.Location.Longitude)
		scored = append(scored, scoredPOI{poi: poi, dist: dist})
	}

	// Sort by distance
	for i := 0; i < len(scored); i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[i].dist > scored[j].dist {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Create combinations of 2-3 POIs
	combinations := make([][]POI, 0)

	// Single POI routes (out and back)
	for i := 0; i < len(scored) && i < 5; i++ {
		combinations = append(combinations, []POI{scored[i].poi})
	}

	// Two POI routes
	for i := 0; i < len(scored) && i < 4; i++ {
		for j := i + 1; j < len(scored) && j < 5; j++ {
			combinations = append(combinations, []POI{scored[i].poi, scored[j].poi})
		}
	}

	// Three POI routes
	for i := 0; i < len(scored) && i < 3; i++ {
		for j := i + 1; j < len(scored) && j < 4; j++ {
			for k := j + 1; k < len(scored) && k < 5; k++ {
				combinations = append(combinations, []POI{scored[i].poi, scored[j].poi, scored[k].poi})
			}
		}
	}

	log.Printf("Generated %d POI combinations from %d POIs", len(combinations), len(pois))
	return combinations
}

// generatePOIAnchoredRoute generates routes that visit interesting POIs
func generatePOIAnchoredRoute(userRoutes []*storage.RouteData, minDistance, maxDistance float64) ([]SuggestedRoute, error) {
	if len(userRoutes) == 0 {
		return []SuggestedRoute{}, nil
	}

	// Filter out geographic outliers
	filteredRoutes := filterOutlierRoutes(userRoutes)
	if len(filteredRoutes) == 0 {
		filteredRoutes = userRoutes
	}

	// Calculate center of user's routes
	var centerLat, centerLng float64
	totalPoints := 0

	for _, route := range filteredRoutes {
		for _, point := range route.TrackPoints {
			centerLat += point.Latitude
			centerLng += point.Longitude
			totalPoints++
		}
	}

	if totalPoints > 0 {
		centerLat /= float64(totalPoints)
		centerLng /= float64(totalPoints)
	} else {
		return []SuggestedRoute{}, fmt.Errorf("no valid route points found")
	}

	log.Printf("Route center: [%f, %f]", centerLat, centerLng)

	// Query nearby POIs within 5km
	pois, err := queryNearbyPOIs(centerLat, centerLng, 5.0)
	if err != nil {
		log.Printf("Error querying POIs: %v", err)
		return []SuggestedRoute{}, err
	}

	if len(pois) == 0 {
		log.Printf("No POIs found nearby")
		return []SuggestedRoute{}, fmt.Errorf("no POIs found nearby")
	}

	// Filter out already visited POIs
	unvisitedPOIs := filterUnvisitedPOIs(pois, filteredRoutes)
	if len(unvisitedPOIs) == 0 {
		log.Printf("All nearby POIs have been visited")
		unvisitedPOIs = pois // Fall back to all POIs if all have been visited
	}

	// Find best POI combinations
	targetDistance := minDistance
	if maxDistance > 0 {
		targetDistance = (minDistance + maxDistance) / 2
	}
	if targetDistance == 0 {
		targetDistance = 5.0 // Default 5km
	}

	combinations := findBestPOICombination(unvisitedPOIs, centerLat, centerLng, targetDistance)
	if len(combinations) == 0 {
		return []SuggestedRoute{}, fmt.Errorf("could not create POI combinations")
	}

	// Generate routes through POI combinations
	suggestions := make([]SuggestedRoute, 0)

	for _, combo := range combinations {
		// Build waypoints: start -> POIs -> back to start (loop)
		waypoints := []storage.TrackPoint{
			{Latitude: centerLat, Longitude: centerLng},
		}

		for _, poi := range combo {
			waypoints = append(waypoints, poi.Location)
		}

		// Close the loop
		waypoints = append(waypoints, storage.TrackPoint{Latitude: centerLat, Longitude: centerLng})

		// Get route following streets
		route, err := getRouteFollowingStreets(waypoints)
		if err != nil {
			log.Printf("Error getting street route for POI combo: %v", err)
			continue
		}

		// Check if route meets distance criteria
		if minDistance > 0 && route.Distance < minDistance {
			continue
		}
		if maxDistance > 0 && route.Distance > maxDistance {
			continue
		}

		suggestions = append(suggestions, route)

		// Limit to 5 suggestions
		if len(suggestions) >= 5 {
			break
		}
	}

	log.Printf("Generated %d POI-anchored route suggestions", len(suggestions))
	return suggestions, nil
}
