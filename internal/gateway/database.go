package gateway

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// Database models
type User struct {
	ID        int       `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	APIKey    string    `json:"api_key"`
	CreatedAt time.Time `json:"created_at"`
}

type Hub struct {
	ID            int       `json:"id"`
	UserID        int       `json:"user_id"`
	HubID         string    `json:"hub_id"`
	Name          string    `json:"name"`
	PublicKey     string    `json:"public_key"`
	ProductKey    string    `json:"product_key"`
	Endpoint      string    `json:"endpoint"`
	Status        string    `json:"status"`
	AutoRegistered bool     `json:"auto_registered"`
	LastSeen      time.Time `json:"last_seen"`
	CreatedAt     time.Time `json:"created_at"`
}

type Device struct {
	ID           int      `json:"id"`
	HubID        int      `json:"hub_id"`
	DeviceID     string   `json:"device_id"`
	DeviceType   string   `json:"device_type"`
	Name         string   `json:"name"`
	Model        string   `json:"model"`
	Address      string   `json:"address"`
	Capabilities []string `json:"capabilities"`
	Status       string   `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

// Database handles SQLite database operations
type Database struct {
	db *sql.DB
}

// NewDatabase creates a new database connection
func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	database := &Database{db: db}
	
	// Initialize database schema
	if err := database.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return database, nil
}

// Close closes the database connection
func (d *Database) Close() error {
	return d.db.Close()
}

// initSchema creates the database tables
func (d *Database) initSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			email TEXT,
			api_key TEXT UNIQUE NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS hubs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
			hub_id TEXT UNIQUE NOT NULL,
			name TEXT NOT NULL,
			public_key TEXT NOT NULL,
			product_key TEXT,
			endpoint TEXT,
			status TEXT DEFAULT 'offline',
			auto_registered BOOLEAN DEFAULT FALSE,
			last_seen DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS devices (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			hub_id INTEGER REFERENCES hubs(id) ON DELETE CASCADE,
			device_id TEXT NOT NULL,
			device_type TEXT NOT NULL,
			name TEXT NOT NULL,
			model TEXT,
			address TEXT,
			capabilities TEXT, -- JSON array as TEXT
			status TEXT DEFAULT 'unknown',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(hub_id, device_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_hubs_user_id ON hubs(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_devices_hub_id ON devices(hub_id)`,
		`CREATE INDEX IF NOT EXISTS idx_users_api_key ON users(api_key)`,
		`CREATE INDEX IF NOT EXISTS idx_hubs_hub_id ON hubs(hub_id)`,
	}

	for _, query := range queries {
		if _, err := d.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query: %w", err)
		}
	}

	return nil
}

// User operations
func (d *Database) CreateUser(username, email string) (*User, error) {
	apiKey := uuid.New().String()
	
	query := `INSERT INTO users (username, email, api_key) VALUES (?, ?, ?)`
	result, err := d.db.Exec(query, username, email, apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get user ID: %w", err)
	}

	return d.GetUser(int(id))
}

func (d *Database) GetUser(id int) (*User, error) {
	query := `SELECT id, username, email, api_key, created_at FROM users WHERE id = ?`
	
	var user User
	err := d.db.QueryRow(query, id).Scan(
		&user.ID, &user.Username, &user.Email, &user.APIKey, &user.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

func (d *Database) GetUserByAPIKey(apiKey string) (*User, error) {
	query := `SELECT id, username, email, api_key, created_at FROM users WHERE api_key = ?`
	
	var user User
	err := d.db.QueryRow(query, apiKey).Scan(
		&user.ID, &user.Username, &user.Email, &user.APIKey, &user.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get user by API key: %w", err)
	}

	return &user, nil
}

// Hub operations
func (d *Database) CreateHub(userID int, hubID, name, publicKey, endpoint string) (*Hub, error) {
	query := `INSERT INTO hubs (user_id, hub_id, name, public_key, endpoint, status, auto_registered, last_seen) 
			  VALUES (?, ?, ?, ?, ?, 'online', FALSE, CURRENT_TIMESTAMP)`
	
	result, err := d.db.Exec(query, userID, hubID, name, publicKey, endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create hub: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get hub ID: %w", err)
	}

	return d.GetHub(int(id))
}

func (d *Database) GetHub(id int) (*Hub, error) {
	query := `SELECT id, user_id, hub_id, name, public_key, product_key, endpoint, status, auto_registered, last_seen, created_at 
			  FROM hubs WHERE id = ?`
	
	var hub Hub
	var userID sql.NullInt64
	var productKey sql.NullString
	err := d.db.QueryRow(query, id).Scan(
		&hub.ID, &userID, &hub.HubID, &hub.Name, &hub.PublicKey, 
		&productKey, &hub.Endpoint, &hub.Status, &hub.AutoRegistered, &hub.LastSeen, &hub.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get hub: %w", err)
	}
	
	// Handle nullable fields
	hub.UserID = int(userID.Int64)
	hub.ProductKey = productKey.String

	return &hub, nil
}

func (d *Database) GetHubByHubID(hubID string) (*Hub, error) {
	query := `SELECT id, user_id, hub_id, name, public_key, endpoint, status, last_seen, created_at 
			  FROM hubs WHERE hub_id = ?`
	
	var hub Hub
	err := d.db.QueryRow(query, hubID).Scan(
		&hub.ID, &hub.UserID, &hub.HubID, &hub.Name, &hub.PublicKey, 
		&hub.Endpoint, &hub.Status, &hub.LastSeen, &hub.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get hub by hub_id: %w", err)
	}

	return &hub, nil
}

// RegisterHub registers a new hub without requiring a user (for initial registration)
func (d *Database) RegisterHub(hubID, publicKey, name, productKey string) (*Hub, error) {
	// Check if hub already exists
	_, err := d.GetHubByHubID(hubID)
	if err == nil {
		// Hub exists, update the public key and name
		query := `UPDATE hubs SET public_key = ?, name = ?, status = 'offline', last_seen = CURRENT_TIMESTAMP 
				  WHERE hub_id = ?`
		_, err := d.db.Exec(query, publicKey, name, hubID)
		if err != nil {
			return nil, fmt.Errorf("failed to update existing hub: %w", err)
		}
		// Return updated hub
		return d.GetHubByHubID(hubID)
	}

	// Hub doesn't exist, create new one with NULL user_id
	query := `INSERT INTO hubs (user_id, hub_id, name, public_key, product_key, endpoint, status, auto_registered, last_seen) 
			  VALUES (NULL, ?, ?, ?, ?, '', 'offline', TRUE, CURRENT_TIMESTAMP)`
	
	result, err := d.db.Exec(query, hubID, name, publicKey, productKey)
	if err != nil {
		return nil, fmt.Errorf("failed to register hub: %w", err)
	}
	
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get hub ID: %w", err)
	}
	
	return d.GetHub(int(id))
}

func (d *Database) GetHubByProductKey(productKey string) (*Hub, error) {
	query := `SELECT id, user_id, hub_id, name, public_key, product_key, endpoint, status, auto_registered, last_seen, created_at 
			  FROM hubs WHERE product_key = ?`
	
	var hub Hub
	var userID sql.NullInt64
	var productKeyDB sql.NullString
	err := d.db.QueryRow(query, productKey).Scan(
		&hub.ID, &userID, &hub.HubID, &hub.Name, &hub.PublicKey,
		&productKeyDB, &hub.Endpoint, &hub.Status, &hub.AutoRegistered, &hub.LastSeen, &hub.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get hub by product key: %w", err)
	}
	
	// Handle nullable fields
	hub.UserID = int(userID.Int64)
	hub.ProductKey = productKeyDB.String

	return &hub, nil
}

func (d *Database) ClaimHub(hubID string, userID int) error {
	query := `UPDATE hubs SET user_id = ?, auto_registered = FALSE WHERE hub_id = ?`
	_, err := d.db.Exec(query, userID, hubID)
	if err != nil {
		return fmt.Errorf("failed to claim hub: %w", err)
	}
	return nil
}

func (d *Database) UpdateDevicesUserID(hubID, userID int) error {
	// Update devices to inherit user_id from the claimed hub
	query := `UPDATE devices SET status = 'claimed' WHERE hub_id = ?`
	_, err := d.db.Exec(query, hubID)
	if err != nil {
		return fmt.Errorf("failed to update device ownership: %w", err)
	}
	return nil
}

func (d *Database) GetUserHubs(userID int) ([]*Hub, error) {
	query := `SELECT id, user_id, hub_id, name, public_key, endpoint, status, last_seen, created_at 
			  FROM hubs WHERE user_id = ? ORDER BY created_at DESC`
	
	rows, err := d.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user hubs: %w", err)
	}
	defer rows.Close()

	var hubs []*Hub
	for rows.Next() {
		var hub Hub
		err := rows.Scan(
			&hub.ID, &hub.UserID, &hub.HubID, &hub.Name, &hub.PublicKey,
			&hub.Endpoint, &hub.Status, &hub.LastSeen, &hub.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan hub: %w", err)
		}
		hubs = append(hubs, &hub)
	}

	return hubs, nil
}

func (d *Database) UpdateHubStatus(hubID, status string) error {
	query := `UPDATE hubs SET status = ?, last_seen = CURRENT_TIMESTAMP WHERE hub_id = ?`
	_, err := d.db.Exec(query, status, hubID)
	if err != nil {
		return fmt.Errorf("failed to update hub status: %w", err)
	}
	return nil
}

// Device operations
func (d *Database) CreateDevice(hubID int, deviceID, deviceType, name, model, address string, capabilities []string) (*Device, error) {
	capabilitiesJSON, err := json.Marshal(capabilities)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal capabilities: %w", err)
	}

	query := `INSERT INTO devices (hub_id, device_id, device_type, name, model, address, capabilities) 
			  VALUES (?, ?, ?, ?, ?, ?, ?)`
	
	result, err := d.db.Exec(query, hubID, deviceID, deviceType, name, model, address, string(capabilitiesJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create device: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get device ID: %w", err)
	}

	return d.GetDevice(int(id))
}

func (d *Database) GetDevice(id int) (*Device, error) {
	query := `SELECT id, hub_id, device_id, device_type, name, model, address, capabilities, status, created_at 
			  FROM devices WHERE id = ?`
	
	var device Device
	var capabilitiesJSON string
	err := d.db.QueryRow(query, id).Scan(
		&device.ID, &device.HubID, &device.DeviceID, &device.DeviceType,
		&device.Name, &device.Model, &device.Address, &capabilitiesJSON,
		&device.Status, &device.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get device: %w", err)
	}

	if err := json.Unmarshal([]byte(capabilitiesJSON), &device.Capabilities); err != nil {
		return nil, fmt.Errorf("failed to unmarshal capabilities: %w", err)
	}

	return &device, nil
}

func (d *Database) GetHubDevices(hubID int) ([]*Device, error) {
	query := `SELECT id, hub_id, device_id, device_type, name, model, address, capabilities, status, created_at 
			  FROM devices WHERE hub_id = ? ORDER BY created_at DESC`
	
	rows, err := d.db.Query(query, hubID)
	if err != nil {
		return nil, fmt.Errorf("failed to query hub devices: %w", err)
	}
	defer rows.Close()

	var devices []*Device
	for rows.Next() {
		var device Device
		var capabilitiesJSON string
		err := rows.Scan(
			&device.ID, &device.HubID, &device.DeviceID, &device.DeviceType,
			&device.Name, &device.Model, &device.Address, &capabilitiesJSON,
			&device.Status, &device.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan device: %w", err)
		}

		if err := json.Unmarshal([]byte(capabilitiesJSON), &device.Capabilities); err != nil {
			return nil, fmt.Errorf("failed to unmarshal capabilities: %w", err)
		}

		devices = append(devices, &device)
	}

	return devices, nil
}

func (d *Database) GetUserDevices(userID int) ([]*Device, error) {
	query := `SELECT d.id, d.hub_id, d.device_id, d.device_type, d.name, d.model, d.address, d.capabilities, d.status, d.created_at 
			  FROM devices d 
			  JOIN hubs h ON d.hub_id = h.id 
			  WHERE h.user_id = ? 
			  ORDER BY d.created_at DESC`
	
	rows, err := d.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user devices: %w", err)
	}
	defer rows.Close()

	var devices []*Device
	for rows.Next() {
		var device Device
		var capabilitiesJSON string
		err := rows.Scan(
			&device.ID, &device.HubID, &device.DeviceID, &device.DeviceType,
			&device.Name, &device.Model, &device.Address, &capabilitiesJSON,
			&device.Status, &device.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan device: %w", err)
		}

		if err := json.Unmarshal([]byte(capabilitiesJSON), &device.Capabilities); err != nil {
			return nil, fmt.Errorf("failed to unmarshal capabilities: %w", err)
		}

		devices = append(devices, &device)
	}

	return devices, nil
}

func (d *Database) UpdateDeviceStatus(deviceID string, status string) error {
	query := `UPDATE devices SET status = ? WHERE device_id = ?`
	_, err := d.db.Exec(query, status, deviceID)
	if err != nil {
		return fmt.Errorf("failed to update device status: %w", err)
	}
	return nil
}

func (d *Database) DeleteDevice(id int) error {
	query := `DELETE FROM devices WHERE id = ?`
	_, err := d.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete device: %w", err)
	}
	return nil
}

// Find device by device_id (for routing messages)
func (d *Database) FindDeviceByID(deviceID string) (*Device, *Hub, error) {
	query := `SELECT d.id, d.hub_id, d.device_id, d.device_type, d.name, d.model, d.address, d.capabilities, d.status, d.created_at,
					 h.id, h.user_id, h.hub_id, h.name, h.public_key, h.endpoint, h.status, h.last_seen, h.created_at
			  FROM devices d 
			  JOIN hubs h ON d.hub_id = h.id 
			  WHERE d.device_id = ?`
	
	var device Device
	var hub Hub
	var capabilitiesJSON string
	
	err := d.db.QueryRow(query, deviceID).Scan(
		&device.ID, &device.HubID, &device.DeviceID, &device.DeviceType,
		&device.Name, &device.Model, &device.Address, &capabilitiesJSON,
		&device.Status, &device.CreatedAt,
		&hub.ID, &hub.UserID, &hub.HubID, &hub.Name, &hub.PublicKey,
		&hub.Endpoint, &hub.Status, &hub.LastSeen, &hub.CreatedAt,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find device and hub: %w", err)
	}

	if err := json.Unmarshal([]byte(capabilitiesJSON), &device.Capabilities); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal capabilities: %w", err)
	}

	return &device, &hub, nil
}