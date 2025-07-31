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

// If the pokemon doesn't exist in the PokeDex already it fetches the data from the PokeAPI
func (cfg *Config) FetchPokemonData(ctx context.Context, identifier string) error {
	// Check if name or ID is provided
	var err error
	if id, parseErr := strconv.Atoi(identifier); parseErr == nil {
		_, err = cfg.DB.FetchPokemonDataById(ctx, int32(id))
	} else {
		_, err = cfg.DB.FetchPokemonDataByName(ctx, strings.ToLower(identifier))
	}
	// Validate that the pokemon does not already exist
	// This should be validated before this function is called
	// If the pokemon already exists we exit early and log a message
	if err == nil {
		log.Printf("pokemon with ID %s already exists in the PokeDex, skipping", identifier)
		return nil
	} else if err != sql.ErrNoRows {
		return fmt.Errorf("error checking for existing pokemon: %w", err)
	}

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
