# Route Suggestion Algorithm Improvements

## Current Algorithm Analysis

### What it does now:
1. Takes all existing user routes
2. Filters out geographic outliers (>50km from median center)
3. Creates a bounding box around remaining routes
4. Adds random variation (±5%) to the bounding box
5. Creates a rectangular perimeter route
6. Optionally asks OSRM to find streets connecting these corners
7. Scales routes to match min/max distance constraints

### Problems with current approach:

1. **Artificial geometric shapes** - Routes are rectangular/polygonal perimeters, not natural walking paths
2. **No learning from user preferences** - Ignores which streets/areas the user actually walks
3. **No exploration** - Just suggests routes within the same area, doesn't discover new places
4. **Poor starting points for OSRM** - Geometric corners rarely align with actual street networks
5. **No variety** - Each suggestion is similar, just a box around existing routes
6. **Ignores route patterns** - Doesn't consider if user prefers loops, out-and-back, or point-to-point routes
7. **No consideration of POIs** - Doesn't suggest routes through parks, waterfronts, landmarks
8. **Distance scaling is imprecise** - Mathematical scaling of lat/lng doesn't account for actual street distances

## Proposed Improvements

### Strategy 1: **Graph-Based Exploration** (Recommended)

Build a graph from user's existing routes and explore nearby unvisited edges.

**Algorithm:**
1. Extract all segments from existing routes (edges in a graph)
2. Build a spatial index of visited segments
3. For each visited segment endpoint, query nearby streets (using OSRM or Overpass API)
4. Find "frontier" streets - adjacent to visited routes but not yet walked
5. Create routes that incorporate some familiar segments + new exploration
6. Score routes based on:
   - % new vs familiar (sweet spot ~60% new)
   - Distance from target
   - Connectivity (prefer loops over out-and-back)
   - POI density (parks, water, etc.)

**Pros:**
- Natural exploration of neighborhood
- Learns from user's actual walking patterns
- Suggests realistic, walkable routes
- Encourages discovery of new areas

**Cons:**
- More complex implementation
- Requires external mapping data
- Higher computational cost

### Strategy 2: **Heatmap-Based Route Generation**

Create a heatmap of areas user has visited, then generate routes through low-heat (unvisited) areas.

**Algorithm:**
1. Create a grid overlay on user's routes (e.g., 100m cells)
2. Count visits to each cell (heatmap)
3. Identify "cold spots" - unvisited cells near visited areas
4. Use OSRM to find routes that:
   - Start/end near high-heat areas (familiar)
   - Pass through cold spots (new exploration)
   - Form natural loops
5. Score by distance match, novelty, and connectivity

**Pros:**
- Balances familiar and new areas
- Relatively simple to implement
- Good for discovering nearby neighborhoods

**Cons:**
- Grid granularity affects quality
- Doesn't directly consider street network

### Strategy 3: **Template-Based Generation**

Use common route patterns as templates and fit them to user's area.

**Algorithm:**
1. Define route templates:
   - "Figure 8" loop
   - "Lollipop" (out-and-back with loop at end)
   - "Double loop" (two connected circles)
   - "Spiral" (gradually expanding)
2. Find a good starting point (from existing route centers)
3. Scale template to match target distance
4. Use OSRM to snap template to actual streets
5. Validate route stays in walkable area

**Pros:**
- Generates aesthetically pleasing routes
- Easy to control route characteristics
- Predictable results

**Cons:**
- Limited variety (only as good as templates)
- May suggest artificial-feeling routes
- Doesn't learn from user behavior

### Strategy 4: **ML-Based Route Prediction** (Advanced)

Train a model on user's route preferences to generate similar routes.

**Algorithm:**
1. Extract features from existing routes:
   - Street types (residential, main road, pedestrian)
   - Elevation changes
   - Turn frequency
   - POI proximity (parks, water, etc.)
   - Time of day patterns
2. Train a model to predict "route quality" score
3. Generate candidate routes using A* or similar
4. Score with trained model
5. Return top-scored routes

**Pros:**
- Highly personalized
- Learns subtle user preferences
- Can improve over time

**Cons:**
- Requires significant training data
- Complex implementation
- May overfit to existing patterns

### Strategy 5: **POI-Anchored Routes** (Quick Win)

Generate routes that visit interesting points of interest.

**Algorithm:**
1. Query nearby POIs from OpenStreetMap:
   - Parks, viewpoints, monuments
   - Cafes, restaurants (for longer routes)
   - Water features, architecture
2. Find POIs user hasn't visited yet (not in existing routes)
3. Use OSRM to create a route visiting 2-4 POIs
4. Prefer routes that form loops
5. Filter by distance constraints

**Pros:**
- Routes have clear "destination" feeling
- Easy to implement with Overpass API
- Encourages exploration
- Routes feel purposeful, not arbitrary

**Cons:**
- Requires external API (OpenStreetMap)
- Quality depends on POI data availability
- May not work well in residential areas

## Recommended Implementation Plan

### Phase 1: Quick Improvements (1-2 days)
1. **Better starting points for OSRM**
   - Instead of bounding box corners, use actual waypoints from existing routes
   - Sample points from less-traveled areas
   - This alone will dramatically improve route quality

2. **Add variety**
   - Generate multiple candidate routes (3-5)
   - Use different strategies (loop, out-and-back, figure-8)
   - Let user pick their favorite

3. **Basic POI integration**
   - Query OpenStreetMap for nearby parks/landmarks
   - Bias route generation toward unvisited POIs

### Phase 2: Graph-Based Exploration (1 week)
1. Build route segment database
2. Implement spatial indexing
3. Find "frontier" streets adjacent to known routes
4. Generate routes that explore new edges

### Phase 3: ML Enhancement (2-3 weeks, optional)
1. Feature extraction from existing routes
2. Route quality scoring model
3. Continuous learning from user feedback

## Sample Code Sketch: POI-Anchored Routes

```go
func generatePOIAnchoredRoute(userRoutes []*storage.RouteData, minDistance, maxDistance float64) ([]SuggestedRoute, error) {
    // 1. Find center of user's routes
    centerLat, centerLng := calculateRouteCenter(userRoutes)
    
    // 2. Query nearby POIs from OpenStreetMap
    pois, err := queryNearbyPOIs(centerLat, centerLng, 5.0) // 5km radius
    if err != nil {
        return nil, err
    }
    
    // 3. Filter out POIs user has already visited
    unvisitedPOIs := filterUnvisitedPOIs(pois, userRoutes)
    
    // 4. Find 2-3 POIs that would create a good route
    targetDistance := (minDistance + maxDistance) / 2
    poiCombinations := findBestPOICombination(unvisitedPOIs, centerLat, centerLng, targetDistance)
    
    // 5. Generate routes through these POIs using OSRM
    var suggestions []SuggestedRoute
    for _, combo := range poiCombinations {
        waypoints := []storage.TrackPoint{
            {Latitude: centerLat, Longitude: centerLng}, // Start
        }
        for _, poi := range combo {
            waypoints = append(waypoints, poi.Location)
        }
        waypoints = append(waypoints, waypoints[0]) // End at start (loop)
        
        route, err := getRouteFollowingStreets(waypoints)
        if err == nil && route.Distance >= minDistance && route.Distance <= maxDistance {
            suggestions = append(suggestions, route)
        }
    }
    
    return suggestions, nil
}
```

## Questions for User

1. **What's most important?**
   - Discovering new neighborhoods/streets?
   - Finding specific destinations (parks, cafes)?
   - Getting precise distance matches?
   - Variety in route shapes?

2. **How much external dependency is acceptable?**
   - OpenStreetMap queries (free, public API)
   - Additional OSRM requests (may need rate limiting)
   - Local database for route history

3. **Performance vs Quality trade-off?**
   - Fast suggestions (< 1 second) with simpler algorithm
   - Slower (2-5 seconds) with better route quality

4. **User feedback mechanism?**
   - Should users rate suggested routes?
   - Track which suggestions get walked?
   - Use this data to improve future suggestions?

## Immediate Action Items

What would you like to prioritize? I recommend starting with **Phase 1** improvements:
- Better waypoint selection (use actual route points, not geometric corners)
- Generate 3-5 route options instead of 1
- Add basic POI discovery

This would take a few hours to implement and significantly improve the perceived quality of suggestions.
