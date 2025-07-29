package main

import (
	"errors"
	"fmt"
	"net/http"
)

var ErrUnauthorized = errors.New("Unauthorized")

func Authorize(r *http.Request) error {
	username := r.FormValue("username")
	// update to check db later
	user, ok := users[username]
	if !ok {
		return ErrUnauthorized
	}

	// Get session token from cookie
	st, err := r.Cookie("session_token")
	if err != nil || st.Value == "" || user.SessionToken != st.Value {
		return ErrUnauthorized
	}

	csrf := r.Header.Get("X-CSRF-Token")
	// print csrf to console for debugging
	fmt.Println("CSRF Token:", csrf)
	fmt.Println("User CSRF Token:", user.CSRFToken)
	if csrf != user.CSRFToken || csrf == "" {
		return ErrUnauthorized
	}

	return nil
}
