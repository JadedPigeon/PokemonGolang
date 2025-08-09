package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"github.com/JadedPigeon/pokemongolang/internal/database"
	"github.com/JadedPigeon/pokemongolang/internal/describe"
	"github.com/JadedPigeon/pokemongolang/internal/handlers"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	godotenv.Load()

	dbURL := os.Getenv("DB_URL")
	useAI := os.Getenv("BATTLE_AI") == "on"
	model := os.Getenv("BATTLE_AI_MODEL")
	if model == "" {
		model = "gpt-4o-mini" // default
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Error opening DB: %v", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("Error pinging DB: %v", err)
	}

	var d describe.Describer = describe.Plain{}
	if useAI {
		d = describe.NewOpenAI(model)
	}

	cfg := &handlers.Config{
		DB:        database.New(db),
		Describer: d,
	}

	// Set up routes
	http.HandleFunc("/register", cfg.RegisterHandler)
	http.HandleFunc("/login", cfg.LoginHandler)
	http.HandleFunc("/logout", cfg.AuthMiddleware(cfg.LogoutHandler))
	http.HandleFunc("/protected", cfg.AuthMiddleware(cfg.ProtectedHandler))
	http.HandleFunc("/catch", cfg.AuthMiddleware(cfg.CatchPokemonHandler))
	http.HandleFunc("/challenge", cfg.AuthMiddleware(cfg.ChooseChallengePokemonHandler))
	http.HandleFunc("/GetUserPokemon", cfg.AuthMiddleware(cfg.GetUserPokemonHandler))
	http.HandleFunc("/ChangeActivePokemon", cfg.AuthMiddleware(cfg.ChangeActivePokemonHandler))
	http.HandleFunc("/StartBattle", cfg.AuthMiddleware(cfg.StartBattleHandler))
	http.HandleFunc("/Fight", cfg.AuthMiddleware(cfg.FightHandler))
	log.Fatal(http.ListenAndServe(":8080", nil))

}
