package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/JadedPigeon/pokemongolang/internal/database"
	"github.com/JadedPigeon/pokemongolang/internal/handlers"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

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

	cfg := &handlers.Config{DB: database.New(db)}

	// testing
	if err := cfg.FetchPokemonData(context.Background(), "jolteon"); err != nil {
		log.Fatalf("failed to fetch pokemon: %v", err)
	} else {
		log.Println("Pokemon inserted/skipped successfully")
	}

	//Four main endpoints to start with:
	http.HandleFunc("/register", cfg.RegisterHandler)
	http.HandleFunc("/login", cfg.LoginHandler)
	http.HandleFunc("/logout", cfg.LogoutHandler)
	//Test endpoint to see if user is logged in
	http.HandleFunc("/protected", cfg.ProtectedHandler)
	http.ListenAndServe(":8080", nil)

}
