package main

import (
	"math"
	"os"
	"testing"
)

func TestHaversineDistance(t *testing.T) {
	// Test cases with known distances
	testCases := []struct {
		lat1, lon1, lat2, lon2, expected float64
	}{
		// Same point should be 0 distance
		{52.5200, 13.4050, 52.5200, 13.4050, 0.0},

		// Berlin TV Tower to Brandenburg Gate (approximately 2.2 km)
		{52.5208, 13.4094, 52.5163, 13.3777, 2.2},

		// New York to Los Angeles (approximately 3940 km)
		{40.7128, -74.0060, 34.0522, -118.2437, 3940.0},
	}

	for i, tc := range testCases {
		distance := haversineDistance(tc.lat1, tc.lon1, tc.lat2, tc.lon2)

		// For the same point, expect exactly 0
		if tc.lat1 == tc.lat2 && tc.lon1 == tc.lon2 {
			if distance != 0 {
				t.Errorf("Test case %d: Expected 0 for same point, got %f", i, distance)
			}
			continue
		}

		// For other cases, allow for some margin of error (5%)
		margin := tc.expected * 0.05
		if math.Abs(distance-tc.expected) > margin {
			t.Errorf("Test case %d: Expected distance around %f km, got %f km",
				i, tc.expected, distance)
		}
	}
}

func TestCalculateRouteDistance(t *testing.T) {
	// Test with empty slice
	emptyRoute := []TrackPoint{}
	if distance := calculateRouteDistance(emptyRoute); distance != 0 {
		t.Errorf("Expected 0 distance for empty route, got %f", distance)
	}

	// Test with single point
	singlePoint := []TrackPoint{{Latitude: 52.5200, Longitude: 13.4050}}
	if distance := calculateRouteDistance(singlePoint); distance != 0 {
		t.Errorf("Expected 0 distance for single point route, got %f", distance)
	}

	// Test with multiple points forming a square in Berlin (approximately 4 km perimeter)
	squareRoute := []TrackPoint{
		{Latitude: 52.52, Longitude: 13.40}, // Alexanderplatz area
		{Latitude: 52.52, Longitude: 13.45}, // 3.5 km east
		{Latitude: 52.56, Longitude: 13.45}, // 4.4 km north
		{Latitude: 52.56, Longitude: 13.40}, // 3.5 km west
		{Latitude: 52.52, Longitude: 13.40}, // 4.4 km south (back to start)
	}

	distance := calculateRouteDistance(squareRoute)
	expectedDistance := 15.8         // Approximate perimeter in km
	margin := expectedDistance * 0.1 // Allow 10% margin of error

	if math.Abs(distance-expectedDistance) > margin {
		t.Errorf("Expected distance around %f km for square route, got %f km",
			expectedDistance, distance)
	}
}

func TestAdjustRouteDistance(t *testing.T) {
	// Test scaling a square route
	originalRoute := []TrackPoint{
		{Latitude: 52.52, Longitude: 13.40},
		{Latitude: 52.52, Longitude: 13.45},
		{Latitude: 52.56, Longitude: 13.45},
		{Latitude: 52.56, Longitude: 13.40},
		{Latitude: 52.52, Longitude: 13.40},
	}

	// Calculate original distance
	originalDistance := calculateRouteDistance(originalRoute)

	// Test scaling down by 50%
	scaleFactor := 0.5
	scaledRoute := adjustRouteDistance(originalRoute, scaleFactor)
	scaledDistance := calculateRouteDistance(scaledRoute)

	// The scaled distance should be approximately scaleFactor * originalDistance
	expectedDistance := originalDistance * scaleFactor
	margin := expectedDistance * 0.1 // Allow 10% margin of error

	if math.Abs(scaledDistance-expectedDistance) > margin {
		t.Errorf("Expected scaled distance around %f km, got %f km",
			expectedDistance, scaledDistance)
	}

	// Verify that the route still has the same number of points
	if len(scaledRoute) != len(originalRoute) {
		t.Errorf("Expected scaled route to have %d points, got %d points",
			len(originalRoute), len(scaledRoute))
	}

	// Verify that the first and last points are still the same (closed loop)
	if originalRoute[0].Latitude == originalRoute[len(originalRoute)-1].Latitude &&
		originalRoute[0].Longitude == originalRoute[len(originalRoute)-1].Longitude {
		if scaledRoute[0].Latitude != scaledRoute[len(scaledRoute)-1].Latitude ||
			scaledRoute[0].Longitude != scaledRoute[len(scaledRoute)-1].Longitude {
			t.Errorf("Scaled route should maintain closed loop property")
		}
	}
}

func TestDecodePolyline(t *testing.T) {
	// Test with a simple polyline
	// This encodes the points: (38.5, -120.2), (40.7, -120.95), (43.252, -126.453)
	polyline := "_p~iF~ps|U_ulLnnqC_mqNvxq`@"

	points := decodePolyline(polyline)

	// Check that we got the right number of points
	if len(points) != 3 {
		t.Errorf("Expected 3 points, got %d", len(points))
	}

	// Check the decoded points (with some tolerance for floating point precision)
	expectedPoints := [][]float64{
		{38.5, -120.2},
		{40.7, -120.95},
		{43.252, -126.453},
	}

	for i, point := range points {
		if i >= len(expectedPoints) {
			break
		}

		if math.Abs(point[0]-expectedPoints[i][0]) > 0.0001 ||
			math.Abs(point[1]-expectedPoints[i][1]) > 0.0001 {
			t.Errorf("Point %d: Expected %v, got %v", i, expectedPoints[i], point)
		}
	}

	// Test with empty polyline
	emptyPoints := decodePolyline("")
	if len(emptyPoints) != 0 {
		t.Errorf("Expected 0 points for empty polyline, got %d", len(emptyPoints))
	}
}

// Add new tests for route generation and manipulation
func TestGenerateSuggestedRoutes(t *testing.T) {
	// We need to set up some test data first
	// Create a test route to populate the routes slice
	testRoute := RouteData{
		Filename: "test.gpx",
		TrackPoints: []TrackPoint{
			{Latitude: 52.52, Longitude: 13.40},
			{Latitude: 52.53, Longitude: 13.41},
			{Latitude: 52.54, Longitude: 13.42},
			{Latitude: 52.53, Longitude: 13.43},
			{Latitude: 52.52, Longitude: 13.40},
		},
		Distance: 5.0,
	}

	// Add the test route to the routes slice
	routesMutex.Lock()
	// Save the original routes and restore them after the test
	originalRoutes := routes
	routes = []RouteData{testRoute}
	defer func() {
		routesMutex.Lock()
		routes = originalRoutes
		routesMutex.Unlock()
	}()
	routesMutex.Unlock()

	// Test case 1: Generate a route with reasonable constraints
	generatedRoutes, err := generateSuggestedRoutes(1.0, 10.0, false)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if len(generatedRoutes) == 0 {
		t.Errorf("Expected at least one route, got none")
	} else {
		// Check that the route meets the distance constraints with a small margin of error
		route := generatedRoutes[0]
		margin := 10.0 * 0.01 // 1% margin
		if route.Distance < 1.0 || route.Distance > 10.0+margin {
			t.Errorf("Route distance %f is outside range [1.0, 10.0]", route.Distance)
		}
	}

	// Test case 2: Generate a route with very large constraints
	generatedRoutes, err = generateSuggestedRoutes(1.0, 1000.0, false)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if len(generatedRoutes) == 0 {
		t.Errorf("Expected at least one route, got none")
	}

	// Test case 3: Generate a route with impossible constraints
	generatedRoutes, err = generateSuggestedRoutes(1000.0, 2000.0, false)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if len(generatedRoutes) > 0 {
		t.Logf("Got a route with distance %f for impossible constraints [1000.0, 2000.0]",
			generatedRoutes[0].Distance)
	}
}

func TestExtendRoute(t *testing.T) {
	// Create a simple route
	originalRoute := []TrackPoint{
		{Latitude: 52.52, Longitude: 13.40},
		{Latitude: 52.53, Longitude: 13.41},
		{Latitude: 52.54, Longitude: 13.42},
	}

	// Calculate original distance
	originalDistance := calculateRouteDistance(originalRoute)

	// Test extending the route by different factors
	testCases := []struct {
		extensionFactor float64
		expectedRatio   float64 // Expected ratio of new distance to original distance
	}{
		{1.0, 1.0}, // No extension
		{2.0, 1.5}, // Double (should add zigzags)
		{3.0, 2.7}, // Triple (should add more zigzags)
	}

	for i, tc := range testCases {
		extendedRoute := extendRoute(originalRoute, tc.extensionFactor)
		extendedDistance := calculateRouteDistance(extendedRoute)

		// For no extension, the route should be unchanged
		if tc.extensionFactor <= 1.0 {
			if len(extendedRoute) != len(originalRoute) {
				t.Errorf("Test case %d: Expected unchanged route length, got %d points instead of %d",
					i, len(extendedRoute), len(originalRoute))
			}
			continue
		}

		// For extension, the route should have more points
		if len(extendedRoute) <= len(originalRoute) {
			t.Errorf("Test case %d: Expected more points after extension, got %d (not > %d)",
				i, len(extendedRoute), len(originalRoute))
		}

		// The distance should be increased
		actualRatio := extendedDistance / originalDistance
		margin := tc.expectedRatio * 0.2 // Allow 20% margin of error

		if math.Abs(actualRatio-tc.expectedRatio) > margin {
			t.Errorf("Test case %d: Expected distance ratio around %f, got %f",
				i, tc.expectedRatio, actualRatio)
		}
	}
}

func TestGetRouteFollowingStreets(t *testing.T) {
	// Skip this test if we're running in a CI environment or without internet
	if os.Getenv("CI") != "" {
		t.Skip("Skipping test in CI environment")
	}

	// Create a simple route with a few points
	testRoute := []TrackPoint{
		{Latitude: 52.52, Longitude: 13.40}, // Berlin Alexanderplatz
		{Latitude: 52.51, Longitude: 13.38}, // Berlin Potsdamer Platz
	}

	// Get a route that follows streets
	streetRoute, err := getRouteFollowingStreets(testRoute)

	// This test might fail if the OSRM API is down or rate-limited
	// So we'll just log the error and skip the test in that case
	if err != nil {
		t.Logf("Error getting street route: %v", err)
		t.Skip("Skipping test due to OSRM API error")
	}

	// Check that we got a valid route
	if len(streetRoute.Points) < 2 {
		t.Errorf("Expected at least 2 points in street route, got %d", len(streetRoute.Points))
	}

	// Check that the route follows streets (FollowsStreets flag is set)
	if !streetRoute.FollowsStreets {
		t.Errorf("Expected FollowsStreets to be true, got false")
	}

	// Check that the route has a reasonable distance
	if streetRoute.Distance <= 0 {
		t.Errorf("Expected positive distance, got %f", streetRoute.Distance)
	}

	// The distance should be greater than the direct distance (as streets aren't straight lines)
	directDistance := haversineDistance(
		testRoute[0].Latitude, testRoute[0].Longitude,
		testRoute[1].Latitude, testRoute[1].Longitude,
	)
	if streetRoute.Distance <= directDistance {
		t.Logf("Street route distance (%f km) is not greater than direct distance (%f km)",
			streetRoute.Distance, directDistance)
	}
}

func TestIsRouteNearExistingRoutes(t *testing.T) {
	// Define a bounding box for existing routes
	minLat, maxLat := 52.50, 52.55
	minLng, maxLng := 13.35, 13.45

	// Test cases with routes inside and outside the bounding box
	testCases := []struct {
		route    []TrackPoint
		expected bool
	}{
		// Route completely inside the bounding box
		{[]TrackPoint{
			{Latitude: 52.52, Longitude: 13.40},
			{Latitude: 52.53, Longitude: 13.42},
		}, true},

		// Route completely outside the bounding box
		{[]TrackPoint{
			{Latitude: 53.52, Longitude: 14.40}, // Far away
			{Latitude: 53.53, Longitude: 14.42},
		}, false},

		// Route partially inside the bounding box (>50% inside)
		{[]TrackPoint{
			{Latitude: 52.52, Longitude: 13.40}, // Inside
			{Latitude: 52.53, Longitude: 13.42}, // Inside
			{Latitude: 53.00, Longitude: 14.00}, // Outside
		}, true},

		// Route partially inside the bounding box (<50% inside)
		{[]TrackPoint{
			{Latitude: 52.52, Longitude: 13.40}, // Inside
			{Latitude: 53.00, Longitude: 14.00}, // Outside
			{Latitude: 53.10, Longitude: 14.10}, // Outside
			{Latitude: 53.20, Longitude: 14.20}, // Outside
		}, false},
	}

	for i, tc := range testCases {
		result := isRouteNearExistingRoutes(tc.route, minLat, maxLat, minLng, maxLng)

		if result != tc.expected {
			t.Errorf("Test case %d: Expected %v, got %v", i, tc.expected, result)
		}
	}
}
