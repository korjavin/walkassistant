package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tkrajina/gpxgo/gpx"
)

// RouteData represents a processed GPX track with additional metadata
type RouteData struct {
	Filename    string       `json:"filename"`
	TrackPoints []TrackPoint `json:"trackPoints"`
	Distance    float64      `json:"distance"`
	Duration    float64      `json:"duration"`
}

// TrackPoint represents a single point in a GPX track
type TrackPoint struct {
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lng"`
}

// SuggestedRoute represents a suggested new route
type SuggestedRoute struct {
	Points   []TrackPoint `json:"points"`
	Distance float64      `json:"distance"`
}

// Global storage for processed routes
var (
	routes      []RouteData
	routesMutex sync.RWMutex
)

func main() {
	// Create data directory if it doesn't exist
	os.MkdirAll("data", os.ModePerm)

	// Load existing GPX files
	loadExistingGPXFiles()

	// Set up HTTP handlers
	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/routes", routesHandler)
	http.HandleFunc("/suggest", suggestHandler)

	// Serve static files
	fs := http.FileServer(http.Dir("./frontend"))
	http.Handle("/", fs)

	fmt.Println("Starting server at port 8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
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

	// Save the file to the data directory
	err = saveFile(file, handler.Filename)
	if err != nil {
		http.Error(w, "Unable to save file", http.StatusInternalServerError)
		return
	}

	// Parse the GPX file
	gpxData, err := parseGPX(handler.Filename)
	if err != nil {
		http.Error(w, "Unable to parse GPX file", http.StatusInternalServerError)
		return
	}

	// Process and store the route data
	route, err := processGPXData(handler.Filename, gpxData)
	if err != nil {
		http.Error(w, "Unable to process GPX data", http.StatusInternalServerError)
		return
	}

	// Add the route to our collection
	routesMutex.Lock()
	routes = append(routes, route)
	routesMutex.Unlock()

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": fmt.Sprintf("File uploaded and processed successfully: %s", handler.Filename),
	})
}

func saveFile(file multipart.File, filename string) error {
	// Create the data directory if it doesn't exist
	err := os.MkdirAll("data", os.ModePerm)
	if err != nil {
		return err
	}

	// Create the file in the data directory
	dst, err := os.Create(fmt.Sprintf("data/%s", filename))
	if err != nil {
		return err
	}
	defer dst.Close()

	// Copy the uploaded file to the destination file
	_, err = io.Copy(dst, file)
	if err != nil {
		return err
	}

	return nil
}

func parseGPX(filename string) (*gpx.GPX, error) {
	filePath := fmt.Sprintf("data/%s", filename)
	gpxFile, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer gpxFile.Close()

	gpxData, err := gpx.Parse(gpxFile)
	if err != nil {
		return nil, err
	}

	return gpxData, nil
}

func processGPXData(filename string, gpxData *gpx.GPX) (RouteData, error) {
	var route RouteData
	route.Filename = filename

	// Process all tracks in the GPX file
	for _, track := range gpxData.Tracks {
		for _, segment := range track.Segments {
			for _, point := range segment.Points {
				route.TrackPoints = append(route.TrackPoints, TrackPoint{
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

func loadExistingGPXFiles() {
	// Get all GPX files from the data directory
	files, err := filepath.Glob("data/*.gpx")
	if err != nil {
		log.Printf("Error loading existing GPX files: %v", err)
		return
	}

	// Process each file
	for _, file := range files {
		filename := filepath.Base(file)
		gpxData, err := parseGPX(filename)
		if err != nil {
			log.Printf("Error parsing GPX file %s: %v", filename, err)
			continue
		}

		route, err := processGPXData(filename, gpxData)
		if err != nil {
			log.Printf("Error processing GPX file %s: %v", filename, err)
			continue
		}

		routesMutex.Lock()
		routes = append(routes, route)
		routesMutex.Unlock()
	}

	log.Printf("Loaded %d existing GPX files", len(routes))
}

func routesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	routesMutex.RLock()
	defer routesMutex.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(routes)
}

func suggestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get query parameters for filtering
	minDistance := 0.0
	maxDistance := 0.0
	if r.URL.Query().Get("minDistance") != "" {
		fmt.Sscanf(r.URL.Query().Get("minDistance"), "%f", &minDistance)
	}
	if r.URL.Query().Get("maxDistance") != "" {
		fmt.Sscanf(r.URL.Query().Get("maxDistance"), "%f", &maxDistance)
	}

	// Generate suggested routes
	suggested, err := generateSuggestedRoutes(minDistance, maxDistance)
	if err != nil {
		http.Error(w, "Unable to generate suggested routes", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(suggested)
}

func generateSuggestedRoutes(minDistance, maxDistance float64) ([]SuggestedRoute, error) {
	routesMutex.RLock()
	defer routesMutex.RUnlock()

	// If no existing routes, return empty suggestions
	if len(routes) == 0 {
		return []SuggestedRoute{}, nil
	}

	// For now, implement a simple algorithm that suggests routes
	// by finding areas that haven't been explored yet

	// Create a grid of the area covered by existing routes
	var minLat, maxLat, minLng, maxLng float64
	var allPoints []TrackPoint

	// Find the bounding box of all existing routes
	for i, route := range routes {
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

	// Create a simple suggested route by finding unexplored areas
	// This is a placeholder algorithm - in a real implementation, you would use
	// more sophisticated techniques to find unexplored areas

	// For now, we'll just create a route that goes around the perimeter of the explored area
	perimeter := []TrackPoint{
		{Latitude: minLat, Longitude: minLng},
		{Latitude: minLat, Longitude: maxLng},
		{Latitude: maxLat, Longitude: maxLng},
		{Latitude: maxLat, Longitude: minLng},
		{Latitude: minLat, Longitude: minLng},
	}

	// Calculate approximate distance of the suggested route
	distance := calculateRouteDistance(perimeter)

	// Apply distance filters if specified
	if (maxDistance > 0 && distance > maxDistance) || (minDistance > 0 && distance < minDistance) {
		// If the route doesn't meet the distance criteria, return empty
		return []SuggestedRoute{}, nil
	}

	return []SuggestedRoute{
		{
			Points:   perimeter,
			Distance: distance,
		},
	}, nil
}

func calculateRouteDistance(points []TrackPoint) float64 {
	if len(points) < 2 {
		return 0
	}

	var distance float64
	for i := 0; i < len(points)-1; i++ {
		// Use Haversine formula to calculate distance between points
		distance += haversineDistance(
			points[i].Latitude, points[i].Longitude,
			points[i+1].Latitude, points[i+1].Longitude,
		)
	}

	return distance
}

func haversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	// Earth's radius in kilometers
	const R = 6371.0

	// Convert degrees to radians
	lat1Rad := lat1 * (3.14159265359 / 180)
	lat2Rad := lat2 * (3.14159265359 / 180)
	lonDiff := (lon2 - lon1) * (3.14159265359 / 180)

	// Haversine formula
	a := (1-cos(lat2Rad-lat1Rad))/2 + cos(lat1Rad)*cos(lat2Rad)*(1-cos(lonDiff))/2
	distance := 2 * R * asin(sqrt(a))

	return distance
}

// Simple math helpers
func cos(x float64) float64 {
	return float64(int(1000000*float64(int(1000000*x))/1000000) / 1000000)
}

func asin(x float64) float64 {
	return float64(int(1000000*float64(int(1000000*x))/1000000) / 1000000)
}

func sqrt(x float64) float64 {
	return float64(int(1000000*float64(int(1000000*x))/1000000) / 1000000)
}
