package server

import (
	"database/sql"
	"fmt"
	"ontree-node/internal/database"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// hashPassword hashes a plain text password
func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// checkPassword checks if password matches hash
func checkPassword(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// authenticateUser verifies username and password
func (s *Server) authenticateUser(username, password string) (*database.User, error) {
	db := database.GetDB()
	
	user := &database.User{}
	err := db.QueryRow(`
		SELECT id, username, password, email, first_name, last_name, 
		       is_staff, is_superuser, is_active, date_joined, last_login
		FROM users WHERE username = ? AND is_active = 1
	`, username).Scan(
		&user.ID, &user.Username, &user.Password, &user.Email,
		&user.FirstName, &user.LastName, &user.IsStaff, &user.IsSuperuser,
		&user.IsActive, &user.DateJoined, &user.LastLogin,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("invalid username or password")
		}
		return nil, err
	}
	
	// Check password
	if err := checkPassword(password, user.Password); err != nil {
		return nil, fmt.Errorf("invalid username or password")
	}
	
	// Update last login
	now := time.Now()
	_, err = db.Exec("UPDATE users SET last_login = ? WHERE id = ?", now, user.ID)
	if err != nil {
		// Log error but don't fail authentication
		fmt.Printf("Failed to update last_login: %v\n", err)
	}
	user.LastLogin = sql.NullTime{Time: now, Valid: true}
	
	return user, nil
}

// getUserByID retrieves a user by ID
func (s *Server) getUserByID(id int) (*database.User, error) {
	db := database.GetDB()
	
	user := &database.User{}
	err := db.QueryRow(`
		SELECT id, username, password, email, first_name, last_name, 
		       is_staff, is_superuser, is_active, date_joined, last_login
		FROM users WHERE id = ? AND is_active = 1
	`, id).Scan(
		&user.ID, &user.Username, &user.Password, &user.Email,
		&user.FirstName, &user.LastName, &user.IsStaff, &user.IsSuperuser,
		&user.IsActive, &user.DateJoined, &user.LastLogin,
	)
	
	if err != nil {
		return nil, err
	}
	
	return user, nil
}

// createUser creates a new user
func (s *Server) createUser(username, password, email string, isStaff, isSuperuser bool) (*database.User, error) {
	hashedPassword, err := hashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}
	
	db := database.GetDB()
	now := time.Now()
	
	result, err := db.Exec(`
		INSERT INTO users (username, password, email, is_staff, is_superuser, is_active, date_joined)
		VALUES (?, ?, ?, ?, ?, 1, ?)
	`, username, hashedPassword, email, isStaff, isSuperuser, now)
	
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get user ID: %w", err)
	}
	
	return &database.User{
		ID:          int(id),
		Username:    username,
		Password:    hashedPassword,
		Email:       sql.NullString{String: email, Valid: email != ""},
		IsStaff:     isStaff,
		IsSuperuser: isSuperuser,
		IsActive:    true,
		DateJoined:  now,
	}, nil
}