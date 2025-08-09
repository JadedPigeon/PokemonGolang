package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand/v2"
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
	Sprites struct {
		Other struct {
			OfficialArtwork struct {
				FrontDefault string `json:"front_default"`
			} `json:"official-artwork"`
		} `json:"other"`
	} `json:"sprites"`
}

// Check if pokemon exists in db, if not get it, then return pokemon data
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
	var data PokeAPIResponse
	if err := getJSON(ctx, fmt.Sprintf("%s%s", pokeapi, identifier), &data); err != nil {
		return fmt.Errorf("failed to fetch data: %w", err)
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

	err := cfg.DB.InsertPokedex(ctx, database.InsertPokedexParams{
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
		ImageUrl:       sql.NullString{String: data.Sprites.Other.OfficialArtwork.FrontDefault, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("error inserting pokemon into db: %w", err)
	}

	// Select up to 4 moves, prioritizing same-type moves
	pokeTypes := map[string]struct{}{
		strings.ToLower(data.Types[0].Type.Name): {},
	}
	if type2.Valid {
		pokeTypes[type2.String] = struct{}{}
	}

	// Shuffle to avoid always picking the same early-list moves
	rand.Shuffle(len(data.Moves), func(i, j int) { data.Moves[i], data.Moves[j] = data.Moves[j], data.Moves[i] })

	selected := make([]int, 0, 4)
	sameType := make([]int, 0, 4)
	others := make([]int, 0, 4)

	const maxAPICalls = 8 // safety valve for slow networks / rate limits
	apiCalls := 0

	for _, m := range data.Moves {
		// Stop once we know we can fill 4 (best case)
		if len(sameType) == 4 {
			break
		}
		// Parse move ID from URL
		parts := strings.Split(strings.Trim(m.Move.URL, "/"), "/")
		if len(parts) == 0 {
			continue
		}
		moveID, err := strconv.Atoi(parts[len(parts)-1])
		if err != nil {
			continue
		}

		// 1) Try DB first (zero HTTP). Your moves table has Power and Type.
		if dbMove, err := cfg.DB.GetMoveByID(ctx, int32(moveID)); err == nil {
			if dbMove.Power > 0 { // power>0 implies non-status
				if _, ok := pokeTypes[strings.ToLower(dbMove.Type)]; ok {
					if len(sameType) < 4 {
						sameType = append(sameType, moveID)
					}
				} else if len(others) < 4 {
					others = append(others, moveID)
				}
			}
			// Already decided from DB; continue to next move.
			continue
		} else if err != sql.ErrNoRows {
			// Unexpected DB error; skip this move gracefully
			continue
		}

		// 2) Not in DB: fall back to API, but respect a hard cap to avoid N calls.
		if apiCalls >= maxAPICalls {
			continue
		}
		md, err := cfg.FetchPokemonMoveData(ctx, moveID)
		apiCalls++
		if err != nil || md == nil || md.Power == nil || md.DamageClass.Name == "status" {
			continue
		}

		moveType := strings.ToLower(md.Type.Name)
		if _, ok := pokeTypes[moveType]; ok {
			if len(sameType) < 4 {
				sameType = append(sameType, moveID)
			}
		} else if len(others) < 4 {
			others = append(others, moveID)
		}
	}

	// Merge preference buckets, cap at 4
	selected = append(selected, sameType...)
	if len(selected) < 4 {
		selected = append(selected, others...)
	}
	if len(selected) > 4 {
		selected = selected[:4]
	}

	// Link moves (use ON CONFLICT DO NOTHING in SQL to avoid dup errors)
	for _, moveID := range selected {
		if err := cfg.DB.InsertPokemonMove(ctx, database.InsertPokemonMoveParams{
			PokemonID: int32(data.ID),
			MoveID:    int32(moveID),
		}); err != nil {
			log.Printf("link move %d -> pokemon %d: %v", moveID, data.ID, err)
		}
	}
	return nil
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
	var move MoveDetail
	if err := getJSON(ctx, moveURL, &move); err != nil {
		return nil, fmt.Errorf("fetch move: %w", err)
	}

	// Check if move already exists
	_, err := cfg.DB.GetMoveByID(ctx, int32(move.ID))
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

	// pokemon_identifier can be either name of ID
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

	newUPID := uuid.New()
	err = cfg.DB.InsertUserPokemon(ctx, database.InsertUserPokemonParams{
		ID:        newUPID,
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

	_, err = cfg.DB.ActivateUserPokemon(ctx, database.ActivateUserPokemonParams{
		UserID: user.ID,
		ID:     newUPID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "pokemon not owned by user"})
			return
		}
		log.Printf("activate failed: %v", err)
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
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Invalid method"})
		return
	}

	if err := r.ParseForm(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Bad form data"})
		return
	}

	// pokemon_identifier can be either name or ID
	pokemon := r.PostForm.Get("pokemon_identifier")
	if pokemon == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "pokemon_identifier is required"})
		return
	}

	ctx := r.Context()
	user, ok := ctx.Value(userContextKey).(*database.User)
	if !ok || user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	// Get pokemon entry
	pokemonEntry, err := cfg.GetPokemon(ctx, pokemon)
	if err != nil {
		log.Printf("error checking for existing pokemon: %s", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
		return
	}

	// Remove previous challenge pokemon if exists
	if user.ChallengePokemonID.Valid {
		if err := cfg.DB.DeleteChallengePokemon(ctx, user.ChallengePokemonID.UUID); err != nil {
			log.Printf("Failed to delete previous challenge: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
			return
		}
	}

	// Insert new challenge pokemon
	challengePokemonID := uuid.New()
	if err := cfg.DB.InsertChallengePokemon(ctx, database.InsertChallengePokemonParams{
		ID:        challengePokemonID,
		PokemonID: sql.NullInt32{Valid: true, Int32: int32(pokemonEntry.ID)},
		CurrentHp: int32(pokemonEntry.Hp),
	}); err != nil {
		log.Printf("error inserting challenge pokemon: %s", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
		return
	}

	// Link challenge pokemon to user
	if err := cfg.DB.SetUserChallengePokemon(ctx, database.SetUserChallengePokemonParams{
		ChallengePokemonID: uuid.NullUUID{UUID: challengePokemonID, Valid: true},
		ID:                 user.ID,
	}); err != nil {
		log.Printf("Could not set challenge pokemon for user: %s", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
		return
	}

	// Success response
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":       "Challenge initiated successfully",
		"pokemon_id":    pokemonEntry.ID,
		"pokemon_name":  pokemonEntry.Name,
		"user_username": user.Username,
	})
}

// Needed Response struct for cleaner JSON response, ie issues with displaying type 2 since they are sql.NullString
type PokedexResponse struct {
	ID             int32  `json:"id"`
	Name           string `json:"name"`
	Type1          string `json:"type1"`
	Type2          string `json:"type2,omitempty"`
	Hp             int32  `json:"hp"`
	Attack         int32  `json:"attack"`
	Defense        int32  `json:"defense"`
	SpecialAttack  int32  `json:"special_attack"`
	SpecialDefense int32  `json:"special_defense"`
	Speed          int32  `json:"speed"`
	Active         bool   `json:"active"`
	ImageUrl       string `json:"image_url,omitempty"`
}

func (cfg *Config) GetUserPokemonHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Invalid method"})
		return
	}

	ctx := r.Context()
	user, ok := ctx.Value(userContextKey).(*database.User)
	if !ok || user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	pokemonList, err := cfg.DB.GetAllUserPokemon(r.Context(), user.ID)
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
		img := ""
		if p.ImageUrl.Valid {
			img = p.ImageUrl.String
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
			ImageUrl:       img,
		})
	}

	writeJSON(w, http.StatusOK, response)
}

func (cfg *Config) ChangeActivePokemonHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Invalid method"})
		return
	}

	if err := r.ParseForm(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Bad form data"})
		return
	}

	// pokemon_identifier is the ID of the pokemon
	pokemonIDStr := r.PostForm.Get("pokemon_identifier")
	if pokemonIDStr == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "pokemon_identifier is required"})
		return
	}

	pokemonIDInt, err := strconv.Atoi(pokemonIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "pokemon_identifier must be a valid integer"})
		return
	}
	pokemonID := int32(pokemonIDInt)

	ctx := r.Context()
	user, ok := ctx.Value(userContextKey).(*database.User)
	if !ok || user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	// Get user_pokemon id
	userPokemon, err := cfg.DB.GetOneUserPokemon(ctx, database.GetOneUserPokemonParams{
		UserID:    user.ID,
		PokemonID: sql.NullInt32{Valid: true, Int32: pokemonID},
	})
	if err != nil {
		if err == sql.ErrNoRows {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Pokemon not found for user"})
			return
		}
		log.Printf("error getting user pokemon: %s", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
		return
	}

	// deactivate all user pokemon
	err = cfg.DB.DeactivateAllUserPokemon(ctx, user.ID)
	if err != nil {
		log.Printf("error deactivating user's pokemon to set new active: %s", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
		return
	}

	// activate new pokemon
	_, err = cfg.DB.ActivateUserPokemon(ctx, database.ActivateUserPokemonParams{
		UserID: user.ID,
		ID:     userPokemon.ID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "pokemon not owned by user"})
			return
		}
		log.Printf("activate failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
		return
	}

	// success response
	writeJSON(w, http.StatusOK, map[string]string{
		"message":       "Active pokemon changed successfully",
		"pokemon_id":    pokemonIDStr,
		"user_username": user.Username,
	})
}

func (cfg *Config) FightHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Invalid method"})
		return
	}
	ctx := r.Context()
	user, ok := ctx.Value(userContextKey).(*database.User)
	if !ok || user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	// Get user's active pokemon
	activePokemon, err := cfg.DB.GetActiveUserPokemon(ctx, user.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "No active pokemon found"})
			return
		}
		log.Printf("error getting active pokemon: %s", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
		return
	}

	// Get challenge pokemon
	challengePokemon, err := cfg.DB.GetUserChallengePokemon(ctx, user.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "No challenge pokemon found"})
			return
		}
		log.Printf("error getting challenge pokemon: %s", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
		return
	}

	// Get user pokemon details
	userPokemon, err := cfg.DB.FetchPokemonDataById(ctx, activePokemon.PokemonID.Int32)
	if err != nil {
		log.Printf("error fetching user pokemon data: %s", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
	}

	// Get user's pokmon moves
	userMoves, err := cfg.DB.GetPokemonMoves(ctx, activePokemon.PokemonID.Int32)
	if err != nil {
		if err == sql.ErrNoRows {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "No moves found for user's pokemon"})
			return
		}
		log.Printf("error getting user pokemon moves: %s", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
		return
	}

	// Get challenge pokemon details
	challengePokemonDetails, err := cfg.DB.FetchPokemonDataById(ctx, challengePokemon.PokemonID.Int32)
	if err != nil {
		log.Printf("error fetching challenge pokemon data: %s", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
	}

	// Get challenge pokemon moves
	challengerMoves, err := cfg.DB.GetPokemonMoves(ctx, challengePokemon.PokemonID.Int32)
	if err != nil {
		if err == sql.ErrNoRows {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "No moves found for challenger's pokemon"})
			return
		}
		log.Printf("error getting challenger pokemon moves: %s", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
		return
	}

	// No game logic implemented yet, just return the data
	type moveDTO struct {
		ID          int32   `json:"id"`
		Name        string  `json:"name"`
		Power       int32   `json:"power"`
		Type        string  `json:"type"`
		Description *string `json:"description,omitempty"`
	}

	toMoves := func(ms []database.Move) []moveDTO {
		out := make([]moveDTO, 0, len(ms))
		for _, m := range ms {
			var desc *string
			if m.Description.Valid {
				desc = &m.Description.String
			}
			out = append(out, moveDTO{
				ID:          m.MoveID,
				Name:        m.Name,
				Power:       m.Power,
				Type:        m.Type,
				Description: desc,
			})
		}
		return out
	}

	type pokemonDTO struct {
		ID    int32    `json:"id"`
		Name  string   `json:"name"`
		Types []string `json:"types"`
		Stats struct {
			HP             int32 `json:"hp"`
			Attack         int32 `json:"attack"`
			Defense        int32 `json:"defense"`
			SpecialAttack  int32 `json:"special_attack"`
			SpecialDefense int32 `json:"special_defense"`
			Speed          int32 `json:"speed"`
		} `json:"stats"`
		ImageURL string    `json:"image_url,omitempty"`
		Moves    []moveDTO `json:"moves"`
	}

	type fightResponse struct {
		User struct {
			Nickname  *string    `json:"nickname,omitempty"`
			CurrentHP int32      `json:"current_hp"`
			IsActive  bool       `json:"is_active"`
			Pokemon   pokemonDTO `json:"pokemon"`
		} `json:"user"`
		Challenger struct {
			CurrentHP int32      `json:"current_hp"`
			Pokemon   pokemonDTO `json:"pokemon"`
		} `json:"challenger"`
	}

	// Build user pokemon payload
	userPoke := pokemonDTO{
		ID:   userPokemon.ID,
		Name: userPokemon.Name,
		Types: func() []string {
			if userPokemon.Type2.Valid {
				return []string{userPokemon.Type1, userPokemon.Type2.String}
			}
			return []string{userPokemon.Type1}
		}(),
		ImageURL: func() string {
			if userPokemon.ImageUrl.Valid {
				return userPokemon.ImageUrl.String
			}
			return ""
		}(),
		Moves: toMoves(userMoves),
	}
	userPoke.Stats.HP = userPokemon.Hp
	userPoke.Stats.Attack = userPokemon.Attack
	userPoke.Stats.Defense = userPokemon.Defense
	userPoke.Stats.SpecialAttack = userPokemon.SpecialAttack
	userPoke.Stats.SpecialDefense = userPokemon.SpecialDefense
	userPoke.Stats.Speed = userPokemon.Speed

	// Build challenger pokemon payload
	challengerPoke := pokemonDTO{
		ID:   challengePokemonDetails.ID,
		Name: challengePokemonDetails.Name,
		Types: func() []string {
			if challengePokemonDetails.Type2.Valid {
				return []string{challengePokemonDetails.Type1, challengePokemonDetails.Type2.String}
			}
			return []string{challengePokemonDetails.Type1}
		}(),
		ImageURL: func() string {
			if challengePokemonDetails.ImageUrl.Valid {
				return challengePokemonDetails.ImageUrl.String
			}
			return ""
		}(),
		Moves: toMoves(challengerMoves),
	}
	challengerPoke.Stats.HP = challengePokemonDetails.Hp
	challengerPoke.Stats.Attack = challengePokemonDetails.Attack
	challengerPoke.Stats.Defense = challengePokemonDetails.Defense
	challengerPoke.Stats.SpecialAttack = challengePokemonDetails.SpecialAttack
	challengerPoke.Stats.SpecialDefense = challengePokemonDetails.SpecialDefense
	challengerPoke.Stats.Speed = challengePokemonDetails.Speed

	resp := fightResponse{}
	if activePokemon.Nickname.Valid {
		resp.User.Nickname = &activePokemon.Nickname.String
	}
	resp.User.CurrentHP = activePokemon.CurrentHp
	resp.User.IsActive = activePokemon.IsActive
	resp.User.Pokemon = userPoke

	resp.Challenger.CurrentHP = challengePokemon.CurrentHp
	resp.Challenger.Pokemon = challengerPoke

	writeJSON(w, http.StatusOK, resp)
}
