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
}

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
	return err
}
