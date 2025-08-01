package main

import (
	"database/sql"
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
		log.Fatalf("Error opening DB: %v", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("Error pinging DB: %v", err)
	}

	cfg := &handlers.Config{DB: database.New(db)}

	// testing
	// if err := cfg.FetchPokemonData(context.Background(), "jolteon"); err != nil {
	// 	log.Fatalf("failed to fetch pokemon: %v", err)
	// } else {
	// 	log.Println("Pokemon inserted/skipped successfully")
	// }

	//Four main endpoints to start with:
	http.HandleFunc("/register", cfg.RegisterHandler)
	http.HandleFunc("/login", cfg.LoginHandler)
	http.HandleFunc("/logout", cfg.AuthMiddleware(cfg.LogoutHandler))
	http.HandleFunc("/protected", cfg.AuthMiddleware(cfg.ProtectedHandler))
	http.HandleFunc("/catch", cfg.AuthMiddleware(cfg.CatchPokemonHandler))
	log.Fatal(http.ListenAndServe(":8080", nil))

}
