package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/JadedPigeon/pokemongolang/internal/database"
	"github.com/google/uuid"
)

type Config struct {
	DB *database.Queries
}

type Login struct {
	HashedPassword string
	SessionToken   string
	CSRFToken      string
}

var ErrUnauthorized = errors.New("Unauthorized")

func (cfg *Config) Authorize(r *http.Request) (*database.User, error) {
	// Look up user by cookie
	cookie, err := r.Cookie("session_token")
	if err != nil || cookie.Value == "" {
		return nil, ErrUnauthorized
	}
	// Get user by session
	user, err := cfg.DB.GetUserBySessionToken(r.Context(), sql.NullString{String: cookie.Value, Valid: true})
	if err != nil || !user.SessionToken.Valid || user.SessionToken.String != cookie.Value {
		return nil, ErrUnauthorized
	}

	csrf := r.Header.Get("X-CSRF-Token")
	if !user.CsrfToken.Valid || csrf != user.CsrfToken.String {
		return nil, ErrUnauthorized
	}

	return &user, nil
}

type contextKey string

const userContextKey contextKey = "user"

func (cfg *Config) AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := cfg.Authorize(r)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		// Optionally, set user in context for downstream handlers
		ctx := r.Context()
		ctx = context.WithValue(ctx, userContextKey, user)
		next(w, r.WithContext(ctx))
	}
}

func (cfg *Config) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad form data", http.StatusBadRequest)
		return
	}
	username := r.PostForm.Get("username")
	password := r.PostForm.Get("password")

	// check if user already exists
	user, err := cfg.DB.GetUserByUsername(r.Context(), username)
	if err == nil && user.ID != uuid.Nil {
		fmt.Printf("User found: %+v\n", user)
		http.Error(w, "User already exists", http.StatusConflict)
		return
	} else if err != nil && err != sql.ErrNoRows {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		log.Printf("DB error: %v", err)
		return
	}

	hashedPassword, err := hashPassword(password)
	if err != nil {
		http.Error(w, "Error hashing password", http.StatusInternalServerError)
		return
	}

	if err := cfg.DB.CreateUser(r.Context(), database.CreateUserParams{
		ID:           uuid.New(),
		Username:     username,
		PasswordHash: hashedPassword,
	}); err != nil {
		http.Error(w, "Error creating user", http.StatusInternalServerError)
		log.Printf("DB error: %v", err)
		return
	}

	fmt.Fprintln(w, "User registered successfully")
}

func (cfg *Config) LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad form data", http.StatusBadRequest)
		return
	}
	username := r.PostForm.Get("username")
	password := r.PostForm.Get("password")

	// check if user exists and validate password
	user_db, err := cfg.DB.GetUserByUsername(r.Context(), username)
	if err != nil || user_db.ID == uuid.Nil {
		log.Printf("DB error: %v", err)
		http.Error(w, "Invalid login", http.StatusUnauthorized)
		return
	}
	if !checkPasswordHash(password, user_db.PasswordHash) {
		log.Printf("Invalid password for user: %s", username)
		http.Error(w, "Invalid login", http.StatusUnauthorized)
		return
	}

	sessionToken := generateToken(32)
	csrfToken := generateToken(32)

	// set session cookie
	const sessionDuration = 24 * time.Hour
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    sessionToken,
		Expires:  time.Now().Add(sessionDuration),
		HttpOnly: true,
		// if this was prod ready Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})

	// set CSRF token cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "csrf_token",
		Value:    csrfToken,
		Expires:  time.Now().Add(sessionDuration),
		HttpOnly: false, // CSRF token should be accessible by JavaScript,
		// if this was prod ready Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})

	// Update session in db
	err = cfg.DB.SetUserSession(r.Context(), database.SetUserSessionParams{
		SessionToken: sql.NullString{String: sessionToken, Valid: true},
		CsrfToken:    sql.NullString{String: csrfToken, Valid: true},
		ID:           user_db.ID,
	})
	if err != nil {
		http.Error(w, "Error setting user session", http.StatusInternalServerError)
		log.Printf("DB error: %v", err)
	}

	fmt.Fprintln(w, "Login successfully")
}

func (cfg *Config) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	user, err := cfg.Authorize(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Clear cookies
	// set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Expires:  time.Now().Add(-time.Hour),
		HttpOnly: true,
	})

	// set CSRF token cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "csrf_token",
		Value:    "",
		Expires:  time.Now().Add(-time.Hour),
		HttpOnly: false,
	})

	err = cfg.DB.SetUserSession(r.Context(), database.SetUserSessionParams{
		SessionToken: sql.NullString{Valid: false},
		CsrfToken:    sql.NullString{Valid: false},
		ID:           user.ID,
	})
	if err != nil {
		http.Error(w, "Error clearing user session", http.StatusInternalServerError)
		log.Printf("DB error: %v", err)
	}

	fmt.Fprintln(w, "Logged out successfully")
}

// Use to test protected endpoints
func (cfg *Config) ProtectedHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	user, ok := r.Context().Value(userContextKey).(*database.User)
	if !ok || user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	fmt.Fprintf(w, "Hello %s, you are making a protected call!", user.Username)
}
