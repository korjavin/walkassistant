package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
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

	// Log the parameters for debugging
	log.Printf("Suggesting routes with parameters: minDistance=%f, maxDistance=%f, followStreets=%t",
		minDistance, maxDistance, followStreets)

	// Generate suggested routes
	var suggested []SuggestedRoute
	var err error

	// If we need a route with a minimum distance and following streets, use a specialized function
	if minDistance > 0 && followStreets {
		log.Printf("Using specialized function to generate a route with minimum distance %f km that follows streets", minDistance)
		suggested, err = generateRouteWithMinDistance(minDistance)
	} else {
		suggested, err = generateSuggestedRoutes(minDistance, maxDistance, followStreets)
	}

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

	// Add some randomization to the perimeter points to generate different routes each time
	// We don't need to seed the random generator as it's already initialized

	// Add some random variation to the bounding box (up to 10% of the size)
	latRange := maxLat - minLat
	lngRange := maxLng - minLng

	// Random variation between -5% and +5%
	minLatVar := minLat + (rand.Float64()*0.1-0.05)*latRange
	minLngVar := minLng + (rand.Float64()*0.1-0.05)*lngRange
	maxLatVar := maxLat + (rand.Float64()*0.1-0.05)*latRange
	maxLngVar := maxLng + (rand.Float64()*0.1-0.05)*lngRange

	// Create a perimeter with the randomized points
	perimeter := []TrackPoint{
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
		// If the route is too long, try to create a shorter route
		// For simplicity, we'll just use a portion of the perimeter
		log.Printf("Route exceeds max distance, scaling down from %f km to %f km", distance, maxDistance)
		scaleFactor := maxDistance / distance
		log.Printf("Using scale factor: %f for perimeter route", scaleFactor)
		perimeter = adjustRouteDistance(perimeter, scaleFactor)
		distance = calculateRouteDistance(perimeter)
		log.Printf("After scaling, perimeter route distance is now: %f km", distance)
	} else if minDistance > 0 && distance < minDistance {
		// If the route is too short, try to create a longer route
		// For simplicity, we'll add some zigzags to make it longer
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

					// Try a completely different approach - use the original perimeter points
					// but create a smaller perimeter that's approximately the right size
					percentage := maxDistance / streetDistance
					log.Printf("Need to keep approximately %.2f%% of the route", percentage*100)

					// Get the original perimeter points (the ones we used to create the street route)
					originalPoints := perimeter   // Use the perimeter points defined above
					if len(originalPoints) >= 4 { // Need at least 4 points for a rectangle
						// Calculate the center of the perimeter
						var centerLat, centerLng float64
						for _, p := range originalPoints {
							centerLat += p.Latitude
							centerLng += p.Longitude
						}
						centerLat /= float64(len(originalPoints))
						centerLng /= float64(len(originalPoints))

						// Create a smaller perimeter by scaling points toward the center
						// Use a slightly smaller scale factor to account for street routing variations
						scaleFactor := percentage * 0.8
						log.Printf("Using scale factor %.4f to create smaller perimeter", scaleFactor)

						var scaledPoints []TrackPoint
						for _, p := range originalPoints {
							// Scale the point toward the center
							newLat := centerLat + (p.Latitude-centerLat)*scaleFactor
							newLng := centerLng + (p.Longitude-centerLng)*scaleFactor
							scaledPoints = append(scaledPoints, TrackPoint{Latitude: newLat, Longitude: newLng})
						}

						// Now get a new street route based on these scaled perimeter points
						log.Printf("Getting new street route based on scaled perimeter points")
						newStreetRoute, err := getRouteFollowingStreets(scaledPoints)

						if err == nil {
							newDistance := newStreetRoute.Distance
							log.Printf("New street route created with distance: %f km", newDistance)

							if newDistance <= maxDistance*1.1 { // Allow a small margin over max distance
								// Success! Use the new route
								streetRoute = newStreetRoute
								log.Printf("Successfully created a street route within max distance")
							} else {
								// Try with an even smaller perimeter
								log.Printf("New route still exceeds max distance (%f km), trying with smaller perimeter", newDistance)

								// Use an even smaller scale factor
								scaleFactor = percentage * 0.5
								log.Printf("Using smaller scale factor %.4f", scaleFactor)

								scaledPoints = []TrackPoint{}
								for _, p := range originalPoints {
									// Scale the point toward the center
									newLat := centerLat + (p.Latitude-centerLat)*scaleFactor
									newLng := centerLng + (p.Longitude-centerLng)*scaleFactor
									scaledPoints = append(scaledPoints, TrackPoint{Latitude: newLat, Longitude: newLng})
								}

								// Try again with the smaller perimeter
								newStreetRoute, err = getRouteFollowingStreets(scaledPoints)
								if err == nil && newStreetRoute.Distance <= maxDistance*1.1 {
									streetRoute = newStreetRoute
									log.Printf("Created street route with smaller perimeter: %f km", newStreetRoute.Distance)
								} else {
									// Try with just a simple rectangle
									log.Printf("Trying with a simple rectangle around the center")

									// Calculate a small rectangle around the center
									// Estimate how big it should be based on the max distance
									// For a 5km max distance, a 0.5km x 0.5km rectangle would give roughly 2km perimeter
									offset := maxDistance / 10.0 / 111.0 // Convert km to degrees (roughly)

									rectPoints := []TrackPoint{
										{Latitude: centerLat - offset, Longitude: centerLng - offset},
										{Latitude: centerLat - offset, Longitude: centerLng + offset},
										{Latitude: centerLat + offset, Longitude: centerLng + offset},
										{Latitude: centerLat + offset, Longitude: centerLng - offset},
										{Latitude: centerLat - offset, Longitude: centerLng - offset}, // Close the loop
									}

									simpleRoute, err := getRouteFollowingStreets(rectPoints)
									if err == nil && simpleRoute.Distance <= maxDistance*1.1 {
										streetRoute = simpleRoute
										log.Printf("Created simple rectangular street route: %f km", simpleRoute.Distance)
									} else {
										// All attempts failed, fall back to mathematical scaling
										log.Printf("All street routing attempts exceeded max distance, falling back to scaled route")
										scaleFactor := maxDistance / streetDistance
										log.Printf("Using scale factor: %f for street route", scaleFactor)
										streetRoute.Points = adjustRouteDistance(streetRoute.Points, scaleFactor)
										streetRoute.Distance = calculateRouteDistance(streetRoute.Points)
										log.Printf("After scaling, street route distance is now: %f km", streetRoute.Distance)
									}
								}
							}
						} else {
							log.Printf("Error getting new street route: %v, falling back to scaled route", err)
							// Fall back to mathematical scaling if the street routing fails
							scaleFactor := maxDistance / streetDistance
							log.Printf("Using scale factor: %f for street route", scaleFactor)
							streetRoute.Points = adjustRouteDistance(streetRoute.Points, scaleFactor)
							streetRoute.Distance = calculateRouteDistance(streetRoute.Points)
							log.Printf("After scaling, street route distance is now: %f km", streetRoute.Distance)
						}
					} else {
						// Not enough points in the original perimeter, fall back to scaling
						log.Printf("Not enough points in original perimeter, falling back to scaled route")
						scaleFactor := maxDistance / streetDistance
						log.Printf("Using scale factor: %f for street route", scaleFactor)
						streetRoute.Points = adjustRouteDistance(streetRoute.Points, scaleFactor)
						streetRoute.Distance = calculateRouteDistance(streetRoute.Points)
						log.Printf("After scaling, street route distance is now: %f km", streetRoute.Distance)
					}
				} else if minDistance > 0 && streetDistance < minDistance {
					log.Printf("Street route is shorter than min distance (%f km), extending to %f km", streetDistance, minDistance)

					// Instead of using zigzags which break the street following,
					// try to get a new street route with a larger perimeter

					// Calculate the center of the existing routes
					var centerLat, centerLng float64
					totalPoints := 0

					// First try to use existing routes for the center
					routesMutex.RLock()
					for _, route := range routes {
						for _, point := range route.TrackPoints {
							centerLat += point.Latitude
							centerLng += point.Longitude
							totalPoints++
						}
					}
					routesMutex.RUnlock()

					// If no existing routes, use the perimeter
					if totalPoints == 0 {
						for _, p := range perimeter {
							centerLat += p.Latitude
							centerLng += p.Longitude
						}
						centerLat /= float64(len(perimeter))
						centerLng /= float64(len(perimeter))
					} else {
						centerLat /= float64(totalPoints)
						centerLng /= float64(totalPoints)
					}

					// Create a polygon around the center point
					// Estimate how far we need to go to get the desired distance
					// 1 degree is roughly 111 km, so we calculate an appropriate offset
					offset := math.Sqrt(minDistance/10.0) / 111.0 // Convert km to degrees

					// Create a polygon with a small number of points (to avoid OSRM API limits)
					numPoints := 5 // Use a pentagon
					var polygonPoints []TrackPoint

					// Create the polygon
					for i := 0; i < numPoints; i++ {
						angle := 2.0 * math.Pi * float64(i) / float64(numPoints)
						polygonPoints = append(polygonPoints, TrackPoint{
							Latitude:  centerLat + offset*math.Sin(angle),
							Longitude: centerLng + offset*math.Cos(angle),
						})
					}

					// Close the loop
					polygonPoints = append(polygonPoints, polygonPoints[0])

					// Try to get a street route with these polygon points
					log.Printf("Trying to get a longer street route with %d polygon points", len(polygonPoints))
					// Force the route to be near existing routes
					newStreetRoute, err := getRouteFollowingStreets(polygonPoints)
					// Skip the check for isRouteNearExistingRoutes since we're deliberately creating a route
					// that might be outside the existing area

					// If successful and meets the minimum distance
					if err == nil && newStreetRoute.Distance >= minDistance {
						// Success!
						streetRoute = newStreetRoute
						log.Printf("Created longer street route with polygon: %f km", newStreetRoute.Distance)
					} else {
						// If that didn't work, try with a larger polygon
						log.Printf("First attempt failed, trying with a larger polygon")

						// Double the offset for a larger polygon
						offset *= 2.0
						polygonPoints = []TrackPoint{}

						// Create the larger polygon
						for i := 0; i < numPoints; i++ {
							angle := 2.0 * math.Pi * float64(i) / float64(numPoints)
							polygonPoints = append(polygonPoints, TrackPoint{
								Latitude:  centerLat + offset*math.Sin(angle),
								Longitude: centerLng + offset*math.Cos(angle),
							})
						}

						// Close the loop
						polygonPoints = append(polygonPoints, polygonPoints[0])

						// Try again with the larger polygon
						log.Printf("Trying with a larger polygon of %d points", len(polygonPoints))
						// Force the route to be near existing routes
						newStreetRoute, err = getRouteFollowingStreets(polygonPoints)
						// Skip the check for isRouteNearExistingRoutes since we're deliberately creating a route
						// that might be outside the existing area

						if err == nil && newStreetRoute.Distance >= minDistance {
							// Success!
							streetRoute = newStreetRoute
							log.Printf("Created longer street route with larger polygon: %f km", newStreetRoute.Distance)
						} else {
							// If all else fails, create a simple route with just a few points
							log.Printf("Polygon attempts failed, trying with a simple route")

							// Create a simple route with just two points far enough apart
							offset = math.Sqrt(minDistance/2.0) / 111.0
							simplePoints := []TrackPoint{
								{Latitude: centerLat - offset, Longitude: centerLng - offset},
								{Latitude: centerLat + offset, Longitude: centerLng + offset},
							}

							// Try with the simple route
							log.Printf("Trying with a simple 2-point route")
							// Force the route to be near existing routes
							newStreetRoute, err = getRouteFollowingStreets(simplePoints)
							// Skip the check for isRouteNearExistingRoutes since we're deliberately creating a route
							// that might be outside the existing area

							if err == nil && newStreetRoute.Distance >= minDistance {
								// Success!
								streetRoute = newStreetRoute
								log.Printf("Created longer street route with simple points: %f km", newStreetRoute.Distance)
							} else {
								// If all attempts fail, try one more time with a larger area
								log.Printf("All street routing attempts failed, trying with a much larger area")

								// Create a simple route with just two points far enough apart
								offset = math.Sqrt(minDistance) / 111.0 // Use a larger offset
								simplePoints := []TrackPoint{
									{Latitude: centerLat - offset, Longitude: centerLng - offset},
									{Latitude: centerLat + offset, Longitude: centerLng + offset},
								}

								// Try with the simple route
								log.Printf("Trying with a simple 2-point route with large offset: %f", offset)
								newStreetRoute, err = getRouteFollowingStreets(simplePoints)

								if err == nil && newStreetRoute.Distance >= minDistance {
									// Success!
									streetRoute = newStreetRoute
									log.Printf("Created longer street route with large offset: %f km", newStreetRoute.Distance)
								} else {
									// If all attempts fail, fall back to the zigzag method
									log.Printf("All street routing attempts failed, falling back to zigzag extension")
									streetRoute.Points = extendRoute(streetRoute.Points, minDistance/streetDistance)
									streetRoute.Distance = calculateRouteDistance(streetRoute.Points)
									log.Printf("After extending with zigzags, street route distance is now: %f km", streetRoute.Distance)
									// Note that this will lose the street-following property
									streetRoute.FollowsStreets = false
								}
							}
						}
					}

				}

				// If we're extending to meet minimum distance, always use the street route
				if minDistance > 0 && streetDistance < minDistance {
					log.Printf("Using street route even though it's outside existing area because we're extending to meet minimum distance")
					suggestedRoute.Points = streetRoute.Points
					suggestedRoute.Distance = streetRoute.Distance
					suggestedRoute.FollowsStreets = true
				} else if isRouteNearExistingRoutes(streetRoute.Points, minLat, maxLat, minLng, maxLng) {
					suggestedRoute.Points = streetRoute.Points
					suggestedRoute.Distance = streetRoute.Distance
					suggestedRoute.FollowsStreets = true
				} else {
					log.Printf("Street route is too far from existing routes, using perimeter route instead")
				}
			}
		} else {
			log.Printf("Error getting street route: %v", err)
		}
	}

	// Log the final route that will be returned
	log.Printf("FINAL ROUTE: Distance=%f km, FollowsStreets=%t, MaxDistance=%f km",
		suggestedRoute.Distance, suggestedRoute.FollowsStreets, maxDistance)

	// Verify that the route respects the max distance constraint
	if maxDistance > 0 && suggestedRoute.Distance > maxDistance {
		log.Printf("WARNING: Final route distance (%f km) still exceeds max distance (%f km)",
			suggestedRoute.Distance, maxDistance)
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
	// If the points are the same, return 0
	if lat1 == lat2 && lon1 == lon2 {
		return 0
	}

	// Earth's radius in kilometers
	const R = 6371.0

	// Convert degrees to radians
	const PI = math.Pi
	lat1Rad := lat1 * (PI / 180)
	lat2Rad := lat2 * (PI / 180)
	lonDiff := (lon2 - lon1) * (PI / 180)
	latDiff := (lat2 - lat1) * (PI / 180)

	// Haversine formula
	a := math.Sin(latDiff/2)*math.Sin(latDiff/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(lonDiff/2)*math.Sin(lonDiff/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	distance := R * c

	return distance
}

// adjustRouteDistance scales a route to match a target distance
func adjustRouteDistance(points []TrackPoint, scaleFactor float64) []TrackPoint {
	// Create a new slice to hold the adjusted points
	adjustedPoints := make([]TrackPoint, len(points))

	// Calculate the centroid of the route
	centroid := TrackPoint{Latitude: 0, Longitude: 0}
	for _, p := range points {
		centroid.Latitude += p.Latitude
		centroid.Longitude += p.Longitude
	}
	centroid.Latitude /= float64(len(points))
	centroid.Longitude /= float64(len(points))

	// Scale each point relative to the centroid
	for i, p := range points {
		adjustedPoints[i] = TrackPoint{
			Latitude:  centroid.Latitude + (p.Latitude-centroid.Latitude)*scaleFactor,
			Longitude: centroid.Longitude + (p.Longitude-centroid.Longitude)*scaleFactor,
		}
	}

	return adjustedPoints
}

// getRouteFollowingStreets uses the OSRM API to get a route that follows streets
func getRouteFollowingStreets(points []TrackPoint) (SuggestedRoute, error) {
	// Use the OSRM API to get a route that follows streets
	// We'll use the public OSRM demo server for this example
	// In a production environment, you would want to host your own OSRM server
	osrmServer := "https://router.project-osrm.org"

	// OSRM API has a limit of 500 waypoints
	// If we have more than 100 points, sample them to reduce the number
	if len(points) > 100 {
		log.Printf("Too many points (%d), sampling to reduce", len(points))
		// Sample the points to reduce the number
		sampledPoints := []TrackPoint{}
		step := len(points) / 100
		if step < 1 {
			step = 1
		}

		for i := 0; i < len(points); i += step {
			sampledPoints = append(sampledPoints, points[i])
		}

		// Make sure we include the last point
		if len(sampledPoints) > 0 && sampledPoints[len(sampledPoints)-1] != points[len(points)-1] {
			sampledPoints = append(sampledPoints, points[len(points)-1])
		}

		points = sampledPoints
		log.Printf("Reduced to %d points", len(points))
	}

	// Log the input points for debugging
	log.Printf("Input points for street routing: %+v", points)

	// Build the coordinates string for the OSRM API
	// Format: lon1,lat1;lon2,lat2;...
	// OSRM API expects coordinates in [longitude, latitude] order
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

	// Log the URL for debugging
	log.Printf("OSRM API URL: %s", url)

	// Make the request to the OSRM API
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Error making OSRM API request: %v", err)
		return SuggestedRoute{}, err
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading OSRM API response: %v", err)
		return SuggestedRoute{}, err
	}

	// Log the response for debugging
	log.Printf("OSRM API response: %s", string(body))

	// Log the distance from OSRM directly
	var osrmDistance float64
	if resp.StatusCode == http.StatusOK {
		var respMap map[string]interface{}
		if err := json.Unmarshal(body, &respMap); err == nil {
			if routes, ok := respMap["routes"].([]interface{}); ok && len(routes) > 0 {
				if route, ok := routes[0].(map[string]interface{}); ok {
					if dist, ok := route["distance"].(float64); ok {
						osrmDistance = dist / 1000.0 // Convert from meters to kilometers
						log.Printf("OSRM reported distance: %f km", osrmDistance)
					}
				}
			}
		}
	}

	// Parse the response
	var osrmResp OSRMResponse
	if err := json.Unmarshal(body, &osrmResp); err != nil {
		log.Printf("Error parsing OSRM API response: %v", err)
		return SuggestedRoute{}, err
	}

	// Check if the OSRM API returned a route
	if osrmResp.Code != "Ok" || len(osrmResp.Routes) == 0 {
		log.Printf("OSRM API did not return a valid route: %s", osrmResp.Code)
		return SuggestedRoute{}, fmt.Errorf("OSRM API did not return a valid route")
	}

	// Decode the polyline geometry
	decodedPoints := decodePolyline(osrmResp.Routes[0].Geometry)

	// Log the decoded points for debugging
	log.Printf("Decoded %d points from polyline", len(decodedPoints))
	if len(decodedPoints) > 0 {
		log.Printf("First point: %v, Last point: %v", decodedPoints[0], decodedPoints[len(decodedPoints)-1])
	}

	// Convert the decoded points to TrackPoints
	// Note: OSRM returns points in [longitude, latitude] format in the API response
	// but our polyline decoder returns them in [latitude, longitude] format
	var trackPoints []TrackPoint
	for _, point := range decodedPoints {
		// Create a new TrackPoint with the correct coordinates
		trackPoint := TrackPoint{
			Latitude:  point[0],
			Longitude: point[1],
		}

		// Log each track point for debugging
		log.Printf("Adding track point: %+v", trackPoint)

		trackPoints = append(trackPoints, trackPoint)
	}

	// Calculate the actual distance using our haversine function to ensure consistency
	actualDistance := 0.0
	if len(trackPoints) >= 2 {
		actualDistance = calculateRouteDistance(trackPoints)
		log.Printf("Calculated street route distance: %f km with %d points", actualDistance, len(trackPoints))
	} else {
		log.Printf("WARNING: Not enough points to calculate distance. Only %d points available.", len(trackPoints))
	}

	// Use the OSRM distance as a fallback if our calculation is zero or very small
	if actualDistance < 0.1 && len(osrmResp.Routes) > 0 {
		// Get the distance directly from the OSRM response (already in meters)
		actualDistance = osrmResp.Routes[0].Distance / 1000.0
		log.Printf("Using OSRM distance as fallback: %f km", actualDistance)

		// If the distance is still too small, use a reasonable default based on the perimeter
		if actualDistance < 0.1 {
			// Calculate the bounding box of the points to estimate a reasonable distance
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

			// Estimate the perimeter of the bounding box
			width := haversineDistance(minLat, minLng, minLat, maxLng)
			height := haversineDistance(minLat, minLng, maxLat, minLng)
			estimatedDistance := 2 * (width + height)

			actualDistance = estimatedDistance
			log.Printf("Using estimated distance based on bounding box: %f km", actualDistance)
		}
	}

	return SuggestedRoute{
		Points:         trackPoints,
		Distance:       actualDistance, // Use our calculated distance instead of OSRM's
		FollowsStreets: true,
	}, nil
}

// decodePolyline decodes a polyline string into a slice of [lat, lng] coordinates
func decodePolyline(polyline string) [][]float64 {
	// Implementation of the Google polyline algorithm
	// See: https://developers.google.com/maps/documentation/utilities/polylinealgorithm
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

		// Convert to floating point and add to coordinates
		lat_f := float64(lat) / 1e5
		lng_f := float64(lng) / 1e5

		// No need to fix negative coordinates anymore - our decoder is working correctly now

		// Log each coordinate for debugging
		log.Printf("Decoded coordinate: [%f, %f]", lat_f, lng_f)

		// OSRM returns coordinates in [longitude, latitude] order, but we need [latitude, longitude]
		coordinates = append(coordinates, []float64{lat_f, lng_f})
	}

	return coordinates
}

// isRouteNearExistingRoutes checks if a route is within a reasonable distance of existing routes
func isRouteNearExistingRoutes(points []TrackPoint, minLat, maxLat, minLng, maxLng float64) bool {
	// Calculate the bounding box of the existing routes with some padding
	latPadding := (maxLat - minLat) * 0.5 // 50% padding
	lngPadding := (maxLng - minLng) * 0.5 // 50% padding

	minLatWithPadding := minLat - latPadding
	maxLatWithPadding := maxLat + latPadding
	minLngWithPadding := minLng - lngPadding
	maxLngWithPadding := maxLng + lngPadding

	// Log the bounding box for debugging
	log.Printf("Existing routes bounding box with padding: [%f,%f,%f,%f]",
		minLatWithPadding, maxLatWithPadding, minLngWithPadding, maxLngWithPadding)

	// Check if at least 50% of the points are within the padded bounding box
	pointsInBounds := 0
	for _, point := range points {
		if point.Latitude >= minLatWithPadding && point.Latitude <= maxLatWithPadding &&
			point.Longitude >= minLngWithPadding && point.Longitude <= maxLngWithPadding {
			pointsInBounds++
		}
	}

	// Calculate the percentage of points in bounds
	percentageInBounds := float64(pointsInBounds) / float64(len(points))
	log.Printf("Percentage of points in bounds: %f%%", percentageInBounds*100)

	// Return true if at least 50% of the points are within the padded bounding box
	return percentageInBounds >= 0.5
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
		length := math.Sqrt(dLat*dLat + dLng*dLng)
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
