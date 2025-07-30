package main

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
)

var ErrUnauthorized = errors.New("Unauthorized")

func (cfg *config) Authorize(r *http.Request) error {
	username := r.FormValue("username")
	// update to check db later
	user_db, err := cfg.DB.GetUserByUsername(r.Context(), username)
	if err != nil {
		return ErrUnauthorized
	}
	if user_db.ID == uuid.Nil {
		return ErrUnauthorized
	}

	// Get session token from cookie
	st, err := r.Cookie("session_token")
	if err != nil || st.Value == "" || !user_db.SessionToken.Valid || user_db.SessionToken.String != st.Value {
		return ErrUnauthorized
	}

	csrf := r.Header.Get("X-CSRF-Token")
	if csrf != user_db.CsrfToken.String || csrf == "" {
		return ErrUnauthorized
	}

	return nil
}
