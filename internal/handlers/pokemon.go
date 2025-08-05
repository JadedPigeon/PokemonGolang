package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/JadedPigeon/pokemongolang/internal/database"
	"github.com/google/uuid"
)

const pokeapi = "https://pokeapi.co/api/v2/pokemon/"

type PokeAPIResponse struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Types []struct {
		Slot int `json:"slot"`
		Type struct {
			Name string `json:"name"`
		} `json:"type"`
	} `json:"types"`
	Stats []struct {
		BaseStat int `json:"base_stat"`
		Stat     struct {
			Name string `json:"name"`
		} `json:"stat"`
	} `json:"stats"`
	Moves []struct {
		Move struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"move"`
	} `json:"moves"`
}

// Check if poksemon exists in db, if not get it, then return pokemon data
func (cfg *Config) GetPokemon(ctx context.Context, identifier string) (*database.Pokedex, error) {
	var (
		pokedexEntry database.Pokedex
		err          error
	)

	if id, parseErr := strconv.Atoi(identifier); parseErr == nil {
		pokedexEntry, err = cfg.DB.FetchPokemonDataById(ctx, int32(id))
	} else {
		pokedexEntry, err = cfg.DB.FetchPokemonDataByName(ctx, strings.ToLower(identifier))
	}

	if err == nil {
		return &pokedexEntry, nil
	} else if err != sql.ErrNoRows {
		return nil, err
	}

	// If not found, fetch from API and insert
	if fetchErr := cfg.FetchPokemonData(ctx, identifier); fetchErr != nil {
		log.Printf("error fetching pokemon data: %s", fetchErr)
		return nil, fetchErr
	}

	// Try fetching again after insert
	if id, parseErr := strconv.Atoi(identifier); parseErr == nil {
		pokedexEntry, err = cfg.DB.FetchPokemonDataById(ctx, int32(id))
	} else {
		pokedexEntry, err = cfg.DB.FetchPokemonDataByName(ctx, strings.ToLower(identifier))
	}
	if err != nil {
		return nil, err
	}
	return &pokedexEntry, nil
}

// Get pokemon from PokeAPI and insert in db
func (cfg *Config) FetchPokemonData(ctx context.Context, identifier string) error {
	resp, err := http.Get(fmt.Sprintf("%s%s", pokeapi, identifier))
	if err != nil {
		return fmt.Errorf("failed to fetch data: %w", err)
	} else if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch data: %s", resp.Status)
	}
	defer resp.Body.Close()

	var data PokeAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Pokemon may have one or two types, handle accordingly
	var type2 sql.NullString
	if len(data.Types) > 1 {
		type2 = sql.NullString{String: strings.ToLower(data.Types[1].Type.Name), Valid: true}
	} else {
		type2 = sql.NullString{Valid: false}
	}

	// Stats may not always be in the same order, so we use the field names
	stats := make(map[string]int32)
	for _, s := range data.Stats {
		stats[s.Stat.Name] = int32(s.BaseStat)
	}
	// Check that api returned all required stats
	requiredStats := []string{"hp", "attack", "defense", "special-attack", "special-defense", "speed"}
	for _, key := range requiredStats {
		if _, ok := stats[key]; !ok {
			return fmt.Errorf("missing stat '%s' for pokemon '%s'", key, data.Name)
		}
	}

	err = cfg.DB.InsertPokedex(ctx, database.InsertPokedexParams{
		ID:             int32(data.ID),
		Name:           strings.ToLower(data.Name),
		Type1:          strings.ToLower(data.Types[0].Type.Name),
		Type2:          type2,
		Hp:             stats["hp"],
		Attack:         stats["attack"],
		Defense:        stats["defense"],
		SpecialAttack:  stats["special-attack"],
		SpecialDefense: stats["special-defense"],
		Speed:          stats["speed"],
	})
	// Select up to 4 moves, prioritizing same-type moves
	pokeTypes := []string{strings.ToLower(data.Types[0].Type.Name)}
	if type2.Valid {
		pokeTypes = append(pokeTypes, type2.String)
	}

	type moveCandidate struct {
		moveID     int
		isSameType bool
	}
	var candidates []moveCandidate
	for _, m := range data.Moves {
		// Parse move ID from URL
		parts := strings.Split(strings.Trim(m.Move.URL, "/"), "/")
		if len(parts) < 1 {
			continue
		}
		moveID, err := strconv.Atoi(parts[len(parts)-1])
		if err != nil {
			continue
		}
		moveDetail, err := cfg.FetchPokemonMoveData(ctx, moveID)
		if err != nil || moveDetail.Power == nil || moveDetail.DamageClass.Name == "status" {
			continue
		}
		isSameType := false
		for _, t := range pokeTypes {
			if strings.ToLower(moveDetail.Type.Name) == t {
				isSameType = true
				break
			}
		}
		candidates = append(candidates, moveCandidate{moveID: moveID, isSameType: isSameType})
	}
	// Prioritize same-type moves
	var selected []int
	for _, c := range candidates {
		if c.isSameType && len(selected) < 4 {
			selected = append(selected, c.moveID)
		}
	}
	for _, c := range candidates {
		if !c.isSameType && len(selected) < 4 {
			selected = append(selected, c.moveID)
		}
	}
	for _, moveID := range selected {
		err := cfg.DB.InsertPokemonMove(ctx, database.InsertPokemonMoveParams{
			PokemonID: sql.NullInt32{Int32: int32(data.ID), Valid: true},
			MoveID:    sql.NullInt32{Int32: int32(moveID), Valid: true},
		})
		if err != nil {
			log.Printf("error linking move %d to pokemon %d: %v", moveID, data.ID, err)
		}
	}
	return err
}

type MoveDetail struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Power       *int   `json:"power"`
	DamageClass struct {
		Name string `json:"name"`
	} `json:"damage_class"`
	Type struct {
		Name string `json:"name"`
	} `json:"type"`
	FlavorTextEntries []struct {
		FlavorText   string                `json:"flavor_text"`
		Language     struct{ Name string } `json:"language"`
		VersionGroup struct{ Name string } `json:"version_group"`
	} `json:"flavor_text_entries"`
}

// helper function to get the latest English description of a move
func getLatestEnglishDescription(entries []struct {
	FlavorText   string                `json:"flavor_text"`
	Language     struct{ Name string } `json:"language"`
	VersionGroup struct{ Name string } `json:"version_group"`
}) string {
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].Language.Name == "en" {
			return entries[i].FlavorText
		}
	}
	return ""
}

// Fetches move data from the PokeAPI and inserts it into the db if it doesn't already exist
func (cfg *Config) FetchPokemonMoveData(ctx context.Context, moveID int) (*MoveDetail, error) {
	moveURL := fmt.Sprintf("https://pokeapi.co/api/v2/move/%d/", moveID)
	resp, err := http.Get(moveURL)
	if err != nil || resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to fetch move from API: %w", err)
	}
	defer resp.Body.Close()

	var move MoveDetail
	if err := json.NewDecoder(resp.Body).Decode(&move); err != nil {
		return nil, fmt.Errorf("failed to decode move: %w", err)
	}

	// Check if move already exists
	_, err = cfg.DB.GetMoveByID(ctx, int32(move.ID))
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("error checking move in db: %w", err)
	} else if err == sql.ErrNoRows {
		description := getLatestEnglishDescription(move.FlavorTextEntries)
		power := int32(0)
		if move.Power != nil {
			power = int32(*move.Power)
		}
		err = cfg.DB.InsertMove(ctx, database.InsertMoveParams{
			MoveID:      int32(move.ID),
			Name:        move.Name,
			Power:       power,
			Type:        move.Type.Name,
			Description: sql.NullString{String: description, Valid: description != ""},
		})
		if err != nil {
			return nil, fmt.Errorf("error inserting move: %w", err)
		}
	}

	return &move, nil
}

// Catch pokemon
func (cfg *Config) CatchPokemonHandler(w http.ResponseWriter, r *http.Request) {
	// Only allow POST requests
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Invalid method"})
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Bad form data"})
		return
	}

	// pokemone_identifier can be either name of ID
	pokemon := r.PostForm.Get("pokemon_identifier")
	if pokemon == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "pokemon_identifier is required"})
		return
	}

	ctx := r.Context()

	// Validate user context
	user, ok := ctx.Value(userContextKey).(*database.User)
	if !ok || user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	// Get pokemon ID
	pokemonEntry, err := cfg.GetPokemon(ctx, pokemon)
	if err != nil {
		log.Printf("error checking for existing pokemon: %s", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
		return
	}

	// Add pokemon to the user's collection
	partysize, err := cfg.DB.CountUserPokemon(ctx, user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
		return
	}
	if partysize >= 6 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "You can only have at most six pokemon in your party"})
		return
	}

	err = cfg.DB.InsertUserPokemon(ctx, database.InsertUserPokemonParams{
		ID:        uuid.New(),
		UserID:    user.ID,
		PokemonID: sql.NullInt32{Valid: true, Int32: int32(pokemonEntry.ID)},
		Nickname:  sql.NullString{Valid: false},
		CurrentHp: int32(pokemonEntry.Hp),
		IsActive:  false,
	})
	if err != nil {
		log.Printf("error inserting user pokemon: %s", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
		return
	}

	// Set the new pokemon as active
	err = cfg.DB.DeactivateAllUserPokemon(ctx, user.ID)
	if err != nil {
		log.Printf("error deactivating user's pokemon to set new active: %s", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
		return
	}

	err = cfg.DB.ActivateUserPokemon(ctx, database.ActivateUserPokemonParams{
		UserID:    user.ID,
		PokemonID: sql.NullInt32{Valid: true, Int32: int32(pokemonEntry.ID)},
	})
	if err != nil {
		log.Printf("error activating user's new pokemon: %s", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
		return
	}

	// Return success response
	response := map[string]interface{}{
		"message":       "Pokemon caught successfully",
		"pokemon_id":    pokemonEntry.ID,
		"pokemon_name":  pokemonEntry.Name,
		"user_username": user.Username,
	}
	writeJSON(w, http.StatusOK, response)
}

// Challenge pokemon
func (cfg *Config) ChooseChallengePokemonHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad form data", http.StatusBadRequest)
		return
	}
	// pokemone_identifier can be either name of ID
	pokemon := r.PostForm.Get("pokemon_identifier")
	if pokemon == "" {
		http.Error(w, "pokemon_identifier is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	user, ok := r.Context().Value(userContextKey).(*database.User)
	if !ok || user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	// Get pokemon ID
	pokemonEntry, err := cfg.GetPokemon(ctx, pokemon)
	if err != nil {
		log.Printf("error checking for existing pokemon: %s", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Remove previous challenge pokemon if exists
	if user.ChallengePokemonID.Valid {
		err := cfg.DB.DeleteChallengePokemon(ctx, user.ChallengePokemonID.UUID)
		if err != nil {
			log.Printf("Failed to delete previous challenge: %v", err)
			return
		}
	}

	challengePokemonId := uuid.New()
	err = cfg.DB.InsertChallengePokemon(ctx, database.InsertChallengePokemonParams{
		ID:        challengePokemonId,
		PokemonID: sql.NullInt32{Valid: true, Int32: int32(pokemonEntry.ID)},
		CurrentHp: int32(pokemonEntry.Hp),
	})
	if err != nil {
		log.Printf("error inserting challenge pokemon: %s", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	err = cfg.DB.SetUserChallengePokemon(ctx, database.SetUserChallengePokemonParams{
		ChallengePokemonID: uuid.NullUUID{UUID: challengePokemonId, Valid: true},
		ID:                 user.ID,
	})
	if err != nil {
		log.Printf("Could not set challenge pokemon for user: %s", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"message":       "Challenge initiated successfully",
		"pokemon_id":    pokemonEntry.ID,
		"pokemon_name":  pokemonEntry.Name,
		"user_username": user.Username,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Needed Responst struct for cleaner JSON response, ie issues with displaying type 2 since they are sql.NullString
type PokedexResponse struct {
	ID             int32  `json:"ID"`
	Name           string `json:"Name"`
	Type1          string `json:"Type1"`
	Type2          string `json:"Type2"`
	Hp             int32  `json:"Hp"`
	Attack         int32  `json:"Attack"`
	Defense        int32  `json:"Defense"`
	SpecialAttack  int32  `json:"SpecialAttack"`
	SpecialDefense int32  `json:"SpecialDefense"`
	Speed          int32  `json:"Speed"`
	Active         bool   `json:"Active"`
}

func (cfg *Config) GetUserPokemonHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	user, ok := ctx.Value(userContextKey).(*database.User)
	if !ok || user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	pokemonList, err := cfg.DB.GetUserPokemon(r.Context(), user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to retrieve Pokémon"})
		return
	}

	var response []PokedexResponse
	for _, p := range pokemonList {
		type2 := ""
		if p.Type2.Valid {
			type2 = p.Type2.String
		}
		response = append(response, PokedexResponse{
			ID:             p.ID,
			Name:           p.Name,
			Type1:          p.Type1,
			Type2:          type2,
			Hp:             p.Hp,
			Attack:         p.Attack,
			Defense:        p.Defense,
			SpecialAttack:  p.SpecialAttack,
			SpecialDefense: p.SpecialDefense,
			Speed:          p.Speed,
			Active:         p.IsActive,
		})
	}

	writeJSON(w, http.StatusOK, response)
}
