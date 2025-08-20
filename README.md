# PokemonGolang

## Description
PokemonGolang is a backend capstone project written in **Go (Golang)** that recreates a Pokémon-like experience, reminscent of the Gameboy games, via a REST API.  
Users can register, log in, catch Pokémon, and sample battle with them in turn-based encounters. The system integrates with the public [PokéAPI](https://pokeapi.co/) to fetch Pokémon data and caches it in a PostgreSQL database for faster gameplay.  

The backend also supports **AI-powered battle descriptions** using OpenAI models, bringing battles to life with dynamic narrations.

---

## Why?
Modern backend engineering requires more than just CRUD operations. This project demonstrates:
- Secure authentication with sessions and CSRF protection.  
- Real-world API integration with caching and data validation.  
- Database-backed state management (users, Pokémon, moves, battles).  
- Extendable design for future features (PvP battles, leveling, inventory systems).  
- Optional AI integration for natural-language game narration.  

It’s built to showcase backend engineering skills in **Go, PostgreSQL, REST APIs, and AI integration**

---

## Quick Start

### Requirements
- Go 1.22+
- PostgreSQL 15+
- [Goose](https://github.com/pressly/goose) for migrations
- [sqlc](https://sqlc.dev/) for type-safe database queries

### Setup
1. Clone the repo:
   ```bash
   git clone https://github.com/JadedPigeon/pokemongolang.git
   cd pokemongolang
   ```

2. Install dependencies:
   ```bash
   go mod tidy
   ```

3. Create and migrate the database:
   ```bash
   createdb pokemongolang
   goose up
   ```

4. Add environment variables in `.env`:
   ```env
   DB_URL=postgres://user:password@localhost:5432/pokemongolang?sslmode=disable
   BATTLE_AI=on
   BATTLE_AI_MODEL=gpt-4o-mini
   OPENAI_API_KEY=your_api_key_here
   ```

5. Run the server:
   ```bash
   go run .
   ```
   The API will be available at: [http://localhost:8080](http://localhost:8080)

---

## Usage

### Auth & Session
> **Cookies & CSRF**: `POST /login` sets two cookies: `session_token` (HttpOnly) and `csrf_token`.  
> All **protected** endpoints require the header `X-CSRF-Token` equal to the `csrf_token` cookie value.

- `POST /register` – Create a new user (`username`, `password`)  
- `POST /login` – Log in as user (sets cookies)  
- `POST /logout` – **Protected**; clears session & CSRF cookies  
- `POST /protected` – **Protected**; simple sanity-check endpoint

### Pokémon
- `POST /catch` – **Protected**; catch Pokemon by name or ID and sets as user's current Pokemon (`pokemon_identifier`)  
- `POST /challenge` – **Protected**; choose a challenger Pokémon (`pokemon_identifier`)  
- `GET /GetUserPokemon` – **Protected**; list the user's current Pokémon including stats and moves.
- `POST /ChangeActivePokemon` – **Protected**; set the user's active Pokémon (need's to have been caught previously) **ID** (`pokemon_identifier`)  

### Battles
- `GET /StartBattle` – **Protected**; Returns the Pokemon stats and moves of the user's and challenger's Pokemon. Note: Four moves are assigned randomly, based on power and type when initially caught, and the user must use one of these four moves when they use the "Fight" api call.
- `POST /Fight` – **Protected**; takes `move_id` and returns a narrated turn (AI if enabled)  

> **Case-sensitive routes**: Note the capitalized paths for `GetUserPokemon`, `ChangeActivePokemon`, `StartBattle`, and `Fight`.

---

## Postman Collection (Quick Testing)

A ready-to-use Postman collection is included: **`pokemongolang.postman_collection.json`**.  
It automatically sets `X-CSRF-Token` from the `csrf_token` cookie for protected requests.

Typical flow:
1. `POST /register`  
2. `POST /login`  
3. `POST /catch` with `pokemon_identifier=charmander`  
4. `GET /GetUserPokemon` (optional)
5. `POST /ChangeActivePokemon` (option - requires another Pokemon to have been previously caught)
6. `POST /challenge` with `pokemon_identifier=bulbasaur`  
7. `GET /StartBattle`  
8. `POST /Fight` with a valid `move_id` from your active Pokémon (`GET /StartBattle` shows available moves)

> A user can catch or change their Pokemon, or change their challenger at any time but must use `GET /StartBattle` again before using `POST /Fight`

---

## Contributing
Contributions are welcome!  
Some ideas for extensions:
- Add PvP battles between authenticated users  
- Implement Pokémon leveling and type advantages  
- Build a lightweight frontend for easier interaction  
- Add Docker support for easier deployment  

Fork the repo, create a feature branch, and submit a PR.  

---

## License
MIT License. Free to use and extend.  
