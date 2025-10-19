package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// SQLiteStorage implements the Storage interface using SQLite
type SQLiteStorage struct {
	db *sql.DB
}

// NewSQLiteStorage creates a new SQLite storage instance
func NewSQLiteStorage(dbPath string) (*SQLiteStorage, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create tables if they don't exist
	if err := createTables(db); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return &SQLiteStorage{db: db}, nil
}

func createTables(db *sql.DB) error {
	// Create users table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			google_id TEXT UNIQUE NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}

	// Create routes table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS routes (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			filename TEXT NOT NULL,
			track_points TEXT NOT NULL,
			distance REAL NOT NULL,
			duration REAL NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create routes table: %w", err)
	}

	// Create index on user_id for faster queries
	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_routes_user_id ON routes(user_id)
	`)
	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}

	return nil
}

// Close closes the database connection
func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

// User management methods

func (s *SQLiteStorage) GetUserByID(userID string) (*User, error) {
	row := s.db.QueryRow("SELECT id, google_id FROM users WHERE id = ?", userID)
	var user User
	err := row.Scan(&user.ID, &user.GoogleID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *SQLiteStorage) GetUserByGoogleID(googleID string) (*User, error) {
	row := s.db.QueryRow("SELECT id, google_id FROM users WHERE google_id = ?", googleID)
	var user User
	err := row.Scan(&user.ID, &user.GoogleID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *SQLiteStorage) CreateUser(googleID string) (*User, error) {
	user := &User{
		ID:       uuid.NewString(),
		GoogleID: googleID,
	}
	stmt, err := s.db.Prepare("INSERT INTO users(id, google_id) VALUES(?, ?)")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	_, err = stmt.Exec(user.ID, user.GoogleID)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *SQLiteStorage) GetAllUsers() ([]*User, error) {
	rows, err := s.db.Query("SELECT id, google_id FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.GoogleID); err != nil {
			return nil, err
		}
		users = append(users, &user)
	}
	return users, nil
}

// Route management methods

func (s *SQLiteStorage) CreateRoute(route *RouteData) error {
	if route.ID == "" {
		route.ID = uuid.NewString()
	}

	// Serialize track points to JSON
	trackPointsJSON, err := json.Marshal(route.TrackPoints)
	if err != nil {
		return fmt.Errorf("failed to marshal track points: %w", err)
	}

	stmt, err := s.db.Prepare(`
		INSERT INTO routes(id, user_id, filename, track_points, distance, duration)
		VALUES(?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(
		route.ID,
		route.UserID,
		route.Filename,
		string(trackPointsJSON),
		route.Distance,
		route.Duration,
	)
	return err
}

func (s *SQLiteStorage) GetRoutesByUserID(userID string) ([]*RouteData, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, filename, track_points, distance, duration
		FROM routes
		WHERE user_id = ?
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var routes []*RouteData
	for rows.Next() {
		var route RouteData
		var trackPointsJSON string

		if err := rows.Scan(
			&route.ID,
			&route.UserID,
			&route.Filename,
			&trackPointsJSON,
			&route.Distance,
			&route.Duration,
		); err != nil {
			return nil, err
		}

		// Deserialize track points
		if err := json.Unmarshal([]byte(trackPointsJSON), &route.TrackPoints); err != nil {
			return nil, fmt.Errorf("failed to unmarshal track points: %w", err)
		}

		routes = append(routes, &route)
	}
	return routes, nil
}

func (s *SQLiteStorage) GetRouteByID(routeID string) (*RouteData, error) {
	row := s.db.QueryRow(`
		SELECT id, user_id, filename, track_points, distance, duration
		FROM routes
		WHERE id = ?
	`, routeID)

	var route RouteData
	var trackPointsJSON string

	err := row.Scan(
		&route.ID,
		&route.UserID,
		&route.Filename,
		&trackPointsJSON,
		&route.Distance,
		&route.Duration,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Deserialize track points
	if err := json.Unmarshal([]byte(trackPointsJSON), &route.TrackPoints); err != nil {
		return nil, fmt.Errorf("failed to unmarshal track points: %w", err)
	}

	return &route, nil
}

func (s *SQLiteStorage) DeleteRoute(routeID string) error {
	stmt, err := s.db.Prepare("DELETE FROM routes WHERE id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(routeID)
	return err
}

func (s *SQLiteStorage) GetAllRoutes() ([]*RouteData, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, filename, track_points, distance, duration
		FROM routes
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var routes []*RouteData
	for rows.Next() {
		var route RouteData
		var trackPointsJSON string

		if err := rows.Scan(
			&route.ID,
			&route.UserID,
			&route.Filename,
			&trackPointsJSON,
			&route.Distance,
			&route.Duration,
		); err != nil {
			return nil, err
		}

		// Deserialize track points
		if err := json.Unmarshal([]byte(trackPointsJSON), &route.TrackPoints); err != nil {
			return nil, fmt.Errorf("failed to unmarshal track points: %w", err)
		}

		routes = append(routes, &route)
	}
	return routes, nil
}
