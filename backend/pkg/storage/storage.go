package storage

// User represents a user in the system
type User struct {
	ID       string `json:"id"`
	GoogleID string `json:"google_id"`
}

// RouteData represents a processed GPX track with additional metadata
type RouteData struct {
	ID          string       `json:"id"`      // Unique ID for the route
	UserID      string       `json:"user_id"` // Owner of the route
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

// Storage defines the interface for data persistence
type Storage interface {
	// User management
	GetUserByID(userID string) (*User, error)
	GetUserByGoogleID(googleID string) (*User, error)
	CreateUser(googleID string) (*User, error)
	GetAllUsers() ([]*User, error)

	// Route management
	CreateRoute(route *RouteData) error
	GetRoutesByUserID(userID string) ([]*RouteData, error)
	GetRouteByID(routeID string) (*RouteData, error)
	DeleteRoute(routeID string) error
	GetAllRoutes() ([]*RouteData, error)
}
