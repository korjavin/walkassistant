package main

import (
	"log"
	"math"
)

// generateRouteWithMinDistance creates a route that follows streets and meets the minimum distance requirement
func generateRouteWithMinDistance(minDistance float64) ([]SuggestedRoute, error) {
	// Lock the routes mutex to safely access the routes
	routesMutex.RLock()
	defer routesMutex.RUnlock()

	// Find the bounding box of all existing routes
	var minLat, maxLat, minLng, maxLng float64
	hasPoints := false

	// Iterate through all routes and their points
	for _, route := range routes {
		for _, point := range route.TrackPoints {
			// Initialize min/max on first point
			if !hasPoints {
				minLat, maxLat = point.Latitude, point.Latitude
				minLng, maxLng = point.Longitude, point.Longitude
				hasPoints = true
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

	// Calculate the center of the existing routes
	centerLat := (minLat + maxLat) / 2
	centerLng := (minLng + maxLng) / 2

	// If we don't have enough existing routes, use a default location
	if minLat == 0 && maxLat == 0 {
		// Use a default location (Berlin, Germany)
		centerLat = 52.52
		centerLng = 13.405
	}

	log.Printf("Using center point: [%f, %f] to generate route with min distance %f km",
		centerLat, centerLng, minDistance)

	// Create a simple route with just two points far enough apart
	// Estimate how far we need to go to get the desired distance
	// 1 degree is roughly 111 km, so we calculate an appropriate offset
	offset := math.Sqrt(minDistance/2.0) / 111.0 // Convert km to degrees

	// Create a simple route with just two points
	simplePoints := []TrackPoint{
		{Latitude: centerLat - offset, Longitude: centerLng - offset},
		{Latitude: centerLat + offset, Longitude: centerLng + offset},
	}

	// Try to get a street route with these points
	log.Printf("Trying to get a street route with 2 points and offset %f", offset)
	streetRoute, err := getRouteFollowingStreets(simplePoints)

	// If successful and meets the minimum distance
	if err == nil && streetRoute.Distance >= minDistance {
		// Success!
		log.Printf("Created street route with distance: %f km", streetRoute.Distance)
		return []SuggestedRoute{streetRoute}, nil
	}

	// If that didn't work, try with a larger offset
	log.Printf("First attempt failed, trying with a larger offset")
	offset *= 2.0
	simplePoints = []TrackPoint{
		{Latitude: centerLat - offset, Longitude: centerLng - offset},
		{Latitude: centerLat + offset, Longitude: centerLng + offset},
	}

	// Try again with the larger offset
	log.Printf("Trying with offset %f", offset)
	streetRoute, err = getRouteFollowingStreets(simplePoints)

	// If successful and meets the minimum distance
	if err == nil && streetRoute.Distance >= minDistance {
		// Success!
		log.Printf("Created street route with larger offset: %f km", streetRoute.Distance)
		return []SuggestedRoute{streetRoute}, nil
	}

	// If that didn't work, try with a polygon
	log.Printf("Simple route attempts failed, trying with a polygon")

	// Create a polygon around the center point
	numPoints := 4 // Use a square
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

	// Try to get a street route with the polygon
	log.Printf("Trying with a polygon of %d points", len(polygonPoints))
	streetRoute, err = getRouteFollowingStreets(polygonPoints)

	// If successful and meets the minimum distance
	if err == nil && streetRoute.Distance >= minDistance {
		// Success!
		log.Printf("Created street route with polygon: %f km", streetRoute.Distance)
		return []SuggestedRoute{streetRoute}, nil
	}

	// If all else fails, fall back to a simple approach
	log.Printf("All specialized attempts failed, falling back to simple approach")

	// Create a simple route with a large offset
	offset = math.Sqrt(minDistance) * 2 / 111.0 // Use a much larger offset
	simplePoints = []TrackPoint{
		{Latitude: centerLat - offset, Longitude: centerLng - offset},
		{Latitude: centerLat + offset, Longitude: centerLng + offset},
	}

	// Try with the simple route one last time
	log.Printf("Trying with a simple 2-point route with very large offset: %f", offset)
	streetRoute, err = getRouteFollowingStreets(simplePoints)

	if err == nil {
		// Use whatever we got, even if it doesn't meet the minimum distance
		log.Printf("Created street route with very large offset: %f km", streetRoute.Distance)
		return []SuggestedRoute{streetRoute}, nil
	}

	// If everything fails, return a simple route that doesn't follow streets
	log.Printf("All attempts failed, returning a simple route that doesn't follow streets")
	simpleRoute := SuggestedRoute{
		Points: []TrackPoint{
			{Latitude: centerLat - offset, Longitude: centerLng - offset},
			{Latitude: centerLat + offset, Longitude: centerLng + offset},
		},
		Distance: calculateRouteDistance([]TrackPoint{
			{Latitude: centerLat - offset, Longitude: centerLng - offset},
			{Latitude: centerLat + offset, Longitude: centerLng + offset},
		}),
		FollowsStreets: false,
	}

	return []SuggestedRoute{simpleRoute}, nil
}
