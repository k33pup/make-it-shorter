package database

import (
	"database/sql"
	"errors"
	"time"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID           int
	Username     string
	PasswordHash string
	Email        string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type UserDB struct {
	db *sql.DB
}

func NewUserDB(connectionString string) (*UserDB, error) {
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, err
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &UserDB{db: db}, nil
}

func (udb *UserDB) Close() error {
	return udb.db.Close()
}

func (udb *UserDB) CreateUser(username, password, email string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	query := `INSERT INTO users (username, password_hash, email) VALUES ($1, $2, $3)`
	_, err = udb.db.Exec(query, username, string(hashedPassword), email)
	if err != nil {
		return err
	}

	return nil
}

func (udb *UserDB) ValidateUser(username, password string) (bool, error) {
	var passwordHash string
	query := `SELECT password_hash FROM users WHERE username = $1`

	err := udb.db.QueryRow(query, username).Scan(&passwordHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password))
	if err != nil {
		return false, nil
	}

	return true, nil
}

func (udb *UserDB) UserExists(username string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)`

	err := udb.db.QueryRow(query, username).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (udb *UserDB) GetUser(username string) (*User, error) {
	user := &User{}
	query := `SELECT id, username, password_hash, email, created_at, updated_at FROM users WHERE username = $1`

	err := udb.db.QueryRow(query, username).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.Email,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	return user, nil
}
