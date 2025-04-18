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
	Points         []TrackPoint `json:"points"`
	Distance       float64      `json:"distance"`
	FollowsStreets bool         `json:"followsStreets"`
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
	followStreets := true // Default to following streets

	if r.URL.Query().Get("minDistance") != "" {
		fmt.Sscanf(r.URL.Query().Get("minDistance"), "%f", &minDistance)
	}
	if r.URL.Query().Get("maxDistance") != "" {
		fmt.Sscanf(r.URL.Query().Get("maxDistance"), "%f", &maxDistance)
	}
	if r.URL.Query().Get("followStreets") == "false" {
		followStreets = false
	}

	// Generate suggested routes
	suggested, err := generateSuggestedRoutes(minDistance, maxDistance, followStreets)
	if err != nil {
		http.Error(w, "Unable to generate suggested routes", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(suggested)
}

func generateSuggestedRoutes(minDistance, maxDistance float64, followStreets bool) ([]SuggestedRoute, error) {
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
	if maxDistance > 0 && distance > maxDistance {
		// If the route is too long, try to create a shorter route
		// For simplicity, we'll just use a portion of the perimeter
		scaleFactor := maxDistance / distance
		perimeter = adjustRouteDistance(perimeter, scaleFactor)
		distance = calculateRouteDistance(perimeter)
	} else if minDistance > 0 && distance < minDistance {
		// If the route is too short, try to create a longer route
		// For simplicity, we'll add some zigzags to make it longer
		perimeter = extendRoute(perimeter, minDistance/distance)
		distance = calculateRouteDistance(perimeter)
	}

	// Create the suggested route
	suggestedRoute := SuggestedRoute{
		Points:         perimeter,
		Distance:       distance,
		FollowsStreets: false,
	}

	// If followStreets is true, try to get a route that follows streets
	if followStreets {
		streetRoute, err := getRouteFollowingStreets(perimeter)
		if err == nil {
			suggestedRoute.Points = streetRoute.Points
			suggestedRoute.Distance = streetRoute.Distance
			suggestedRoute.FollowsStreets = true
		} else {
			log.Printf("Error getting street route: %v", err)
		}
	}

	return []SuggestedRoute{suggestedRoute}, nil
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

// adjustRouteDistance scales a route to match a target distance
func adjustRouteDistance(points []TrackPoint, scaleFactor float64) []TrackPoint {
	// For simplicity, we'll just take a portion of the route
	// In a real implementation, you would use more sophisticated techniques
	if len(points) <= 2 {
		return points
	}

	// If scale factor is close to 1, return the original route
	if scaleFactor > 0.9 {
		return points
	}

	// Calculate how many points to keep
	numPoints := int(float64(len(points)-1) * scaleFactor)
	if numPoints < 2 {
		numPoints = 2
	}

	// Create a new route with fewer points
	newPoints := make([]TrackPoint, numPoints)
	for i := 0; i < numPoints-1; i++ {
		index := int(float64(i) * float64(len(points)-1) / float64(numPoints-1))
		newPoints[i] = points[index]
	}

	// Always include the last point to close the loop
	newPoints[numPoints-1] = points[len(points)-1]

	return newPoints
}

// getRouteFollowingStreets uses the OSRM API to get a route that follows streets
func getRouteFollowingStreets(points []TrackPoint) (SuggestedRoute, error) {
	// Use the OSRM API to get a route that follows streets
	// We'll use the public OSRM demo server for this example
	// In a production environment, you would want to host your own OSRM server
	osrmServer := "https://router.project-osrm.org"

	// Build the coordinates string for the OSRM API
	// Format: lon1,lat1;lon2,lat2;...
	var coordsBuilder strings.Builder
	for i, point := range points {
		if i > 0 {
			coordsBuilder.WriteString(";")
		}
		coordsBuilder.WriteString(fmt.Sprintf("%f,%f", point.Longitude, point.Latitude))
	}

	// Build the OSRM API URL
	// We're using the "route" service with the "walking" profile
	url := fmt.Sprintf("%s/route/v1/walking/%s?overview=full&geometries=polyline",
		osrmServer, coordsBuilder.String())

	// Make the request to the OSRM API
	resp, err := http.Get(url)
	if err != nil {
		return SuggestedRoute{}, err
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return SuggestedRoute{}, err
	}

	// Parse the response
	var osrmResp OSRMResponse
	if err := json.Unmarshal(body, &osrmResp); err != nil {
		return SuggestedRoute{}, err
	}

	// Check if the OSRM API returned a route
	if osrmResp.Code != "Ok" || len(osrmResp.Routes) == 0 {
		return SuggestedRoute{}, fmt.Errorf("OSRM API did not return a valid route")
	}

	// Decode the polyline geometry
	decodedPoints := decodePolyline(osrmResp.Routes[0].Geometry)

	// Convert the decoded points to TrackPoints
	var trackPoints []TrackPoint
	for _, point := range decodedPoints {
		trackPoints = append(trackPoints, TrackPoint{
			Latitude:  point[0],
			Longitude: point[1],
		})
	}

	return SuggestedRoute{
		Points:         trackPoints,
		Distance:       osrmResp.Routes[0].Distance / 1000, // Convert from meters to kilometers
		FollowsStreets: true,
	}, nil
}

// decodePolyline decodes a polyline string into a slice of [lat, lng] coordinates
func decodePolyline(polyline string) [][]float64 {
	var coordinates [][]float64
	index, lat, lng := 0, 0, 0

	for index < len(polyline) {
		result, shift := 1, 0
		var b int

		// Decode latitude
		for {
			b = int(polyline[index]) - 63
			index++
			result += (b & 0x1f) << shift
			shift += 5
			if b < 0x20 {
				break
			}
		}

		if (result & 1) != 0 {
			lat += ^(result >> 1)
		} else {
			lat += result >> 1
		}

		result, shift = 1, 0

		// Decode longitude
		for {
			b = int(polyline[index]) - 63
			index++
			result += (b & 0x1f) << shift
			shift += 5
			if b < 0x20 {
				break
			}
		}

		if (result & 1) != 0 {
			lng += ^(result >> 1)
		} else {
			lng += result >> 1
		}

		lat_f := float64(lat) / 1e5
		lng_f := float64(lng) / 1e5
		coordinates = append(coordinates, []float64{lat_f, lng_f})
	}

	return coordinates
}

// extendRoute makes a route longer by adding zigzags
func extendRoute(points []TrackPoint, extensionFactor float64) []TrackPoint {
	// For simplicity, we'll add zigzags to the route
	// In a real implementation, you would use more sophisticated techniques
	if len(points) <= 2 || extensionFactor <= 1.0 {
		return points
	}

	// Calculate how many zigzags to add
	numZigzags := int(extensionFactor) - 1
	if numZigzags < 1 {
		numZigzags = 1
	}

	// Create a new route with zigzags
	var newPoints []TrackPoint

	// Add zigzags between each pair of points
	for i := 0; i < len(points)-1; i++ {
		p1 := points[i]
		p2 := points[i+1]

		// Add the first point
		newPoints = append(newPoints, p1)

		// Calculate the midpoint
		midLat := (p1.Latitude + p2.Latitude) / 2
		midLng := (p1.Longitude + p2.Longitude) / 2

		// Calculate perpendicular direction
		dLat := p2.Latitude - p1.Latitude
		dLng := p2.Longitude - p1.Longitude

		// Normalize and rotate 90 degrees
		length := sqrt(dLat*dLat + dLng*dLng)
		if length > 0 {
			perpLat := -dLng / length * 0.01 // Scale factor for zigzag size
			perpLng := dLat / length * 0.01  // Scale factor for zigzag size

			// Add zigzags
			for j := 0; j < numZigzags; j++ {
				// Alternate zigzag direction
				direction := 1.0
				if j%2 == 1 {
					direction = -1.0
				}

				// Add a point in the zigzag
				zigzagPoint := TrackPoint{
					Latitude:  midLat + perpLat*direction,
					Longitude: midLng + perpLng*direction,
				}
				newPoints = append(newPoints, zigzagPoint)
			}
		}
	}

	// Add the last point
	newPoints = append(newPoints, points[len(points)-1])

	return newPoints
}
