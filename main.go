package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/JadedPigeon/pokemongolang/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type config struct {
	DB *database.Queries
}

type Login struct {
	HashedPassword string
	SessionToken   string
	CSRFToken      string
}

// Key in the username to stand in for a db for now
// var users = map[string]Login{}

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Println("Error loading db: ", err)
	}
	err = db.Ping()
	if err != nil {
		fmt.Println("Error connecting to DB: ", err)
		return
	}

	cfg := &config{
		DB: database.New(db),
	}

	//Four main endpoints to start with:
	http.HandleFunc("/register", cfg.RegisterHandler)
	http.HandleFunc("/login", cfg.LoginHandler)
	http.HandleFunc("/logout", cfg.LogoutHandler)
	//Test endpoint to see if user is logged in
	http.HandleFunc("/protected", cfg.ProtectedHandler)
	http.ListenAndServe(":8080", nil)
}

func (cfg *config) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	// check if user already exists
	user, err := cfg.DB.GetUserByUsername(r.Context(), username)
	if err == nil && user.ID != uuid.Nil {
		fmt.Printf("User found: %+v\n", user)
		http.Error(w, "User already exists", http.StatusConflict)
		return
	} else if err != nil && err != sql.ErrNoRows {
		http.Error(w, "Database error", http.StatusInternalServerError)
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
		return
	}

	fmt.Fprintln(w, "User registered successfully")
}

func (cfg *config) LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	// check if user exists and validate password
	user_db, err := cfg.DB.GetUserByUsername(r.Context(), username)
	if err != nil || user_db.ID == uuid.Nil || !checkPasswordHash(password, user_db.PasswordHash) {
		http.Error(w, "Invalid login", http.StatusUnauthorized)
		return
	}

	sessionToken := generateToken(32)
	csrfToken := generateToken(32)

	// set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:    "session_token",
		Value:   sessionToken,
		Expires: time.Now().Add(24 * time.Hour),
		// Makes sure the cookie cannot be access by front end JavaScript
		HttpOnly: true,
	})

	// set CSRF token cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "csrf_token",
		Value:    csrfToken,
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: false, // CSRF token should be accessible by JavaScript
	})

	// Update session in db
	err = cfg.DB.SetUserSession(r.Context(), database.SetUserSessionParams{
		SessionToken: sql.NullString{String: sessionToken, Valid: true},
		CsrfToken:    sql.NullString{String: csrfToken, Valid: true},
		ID:           user_db.ID,
	})
	if err != nil {
		http.Error(w, "Error setting user session", http.StatusInternalServerError)
	}

	fmt.Fprintln(w, "Login successfully")
}

func (cfg *config) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if err := cfg.Authorize(r); err != nil {
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

	// clear tokens from db
	username := r.FormValue("username")
	user_db, err := cfg.DB.GetUserByUsername(r.Context(), username)
	if err != nil {
		http.Error(w, "Error getting user", http.StatusInternalServerError)
		return
	}
	err = cfg.DB.SetUserSession(r.Context(), database.SetUserSessionParams{
		SessionToken: sql.NullString{Valid: false},
		CsrfToken:    sql.NullString{Valid: false},
		ID:           user_db.ID,
	})
	if err != nil {
		http.Error(w, "Error clearing user session", http.StatusInternalServerError)
	}

	fmt.Fprintln(w, "Logged out successfully")
}

func (cfg *config) ProtectedHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	if err := cfg.Authorize(r); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	username := r.FormValue("username")
	fmt.Fprintf(w, "Hello %s, you are making a protected call!", username)
}
