# PokemonGolang

## Description
PokemonGolang is a backend capstone project written in **Go (Golang)** that recreates a Pokémon-like experience, reminscent of the Gameboy games, via a REST API.  
Users can register, log in, catch Pokémon, and sample battle with them in turn-based encounters. The system integrates with the public [PokéAPI](https://pokeapi.co/) to fetch Pokémon data and caches it in a PostgreSQL database for faster gameplay.  

The backend also supports **AI-powered battle descriptions** using OpenAI models, bringing battles to life with dynamic narrations.

---

## WHY?

Modern backend engineering requires more than just CRUD operations. I wanted to challenge myself to build something that tackled real-world backend problems in a setting I enjoy—so I recreated the excitement and nostalgia of a Gameboy-style Pokémon game, but as a web service.

This project demonstrates:

- Secure authentication with sessions and CSRF protection
- Real-world API integration with caching and data validation
- Database-backed state management (users, Pokémon, moves, battles)
- Extendable design for future features (PvP battles, leveling, inventory systems)
- Optional AI integration for natural-language game narration

It's built to showcase backend engineering skills in Go, PostgreSQL, REST APIs, and AI integration.

Whether you're reviewing code, testing your API skills, or just curious about blending classic games with modern tech, this repo is meant to teach, demonstrate, and inspire.

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

## Try it out with these examples!

1. **Register your user**
   ```bash
   curl -X POST http://localhost:8080/register \
     -d "username=AshKetchum&password=pokemon123"
   ```

2. **Log in as your user**  
   This returns a session cookie and CSRF token in `cookies.txt` for simpler demonstration purposes.
   ```bash
   curl -i -c cookies.txt -X POST http://localhost:8080/login \
     -d "username=AshKetchum&password=pokemon123"
   ```

3. **Extract the CSRF token (optional)**  
   This makes it easier to use in future calls (you can also copy it directly from the login response).
   ```bash
   CSRF=$(grep csrf_token cookies.txt | awk '{print $7}')
   ```

4. **Catch your first Pokémon!**  
   Remember to use the CSRF token from the login step manually if you don't do step 3. Try other Pokemon(Max of 6)! You can use the Pokemon's name or id.
   ```bash
   curl -b cookies.txt -X POST http://localhost:8080/catch \
     -H "X-CSRF-Token: $CSRF" \
     -d "pokemon_identifier=pikachu"
   ```

5. **Check your Pokémon**
   ```bash
   curl -b cookies.txt http://localhost:8080/GetUserPokemon \
     -H "X-CSRF-Token: $CSRF"
   ```

6. **Choose your challenger**
    Try other challengers if you want! You can use the Pokemon's name or id.
   ```bash
   curl -b cookies.txt -X POST http://localhost:8080/challenge \
     -H "X-CSRF-Token: $CSRF" \
     -d "pokemon_identifier=meowth"
   ```

7. **Initiate the battle**  
   You will need the `move_id` of one of the four moves your Pokémon knows. These moves are randomly assigned when the Pokémon is caught, based on power and type. They are returned in this call.
   ```bash
   curl -b cookies.txt http://localhost:8080/StartBattle \
     -H "X-CSRF-Token: $CSRF"
   ```

8. **Fight!**  
   Note: the `move_id` below may not work for you since moves are randomly assigned. Use a valid `move_id` from the previous `StartBattle` call. If using the default value for step 4 and caught Pikachu I've found move_id 24 to be pretty consistent.
   ```bash
   curl -b cookies.txt -X POST http://localhost:8080/Fight \
     -H "X-CSRF-Token: $CSRF" \
     -d "move_id=24"
   ```

---

✨ If you enabled the AI configuration, enjoy dynamic descriptions of your Pokémon using their moves against each other in the `"action_description"` field of the response.

Example AI description using Pikachu and Meowth:

{
	"user": {
		"name": "pikachu",
		"move_used": {
			"id": 24,
			"name": "double-kick",
			"type": "fighting",
			"power": 30,
			"description": "The user attacks by kicking the target twice in a row using two feet."
		},
		"action_description": "Pikachu lunges forward, its tiny feet moving with surprising agility. In a swift motion, it delivers two powerful kicks to Meowth, each strike sending a jolt through its feline frame. The electric mouse, with its determined gaze, shows no hesitation as it executes the rapid assault, leaving its opponent momentarily reeling from the unexpected barrage."
	},
	"challenger": {
		"name": "meowth",
		"move_used": {
			"id": 343,
			"name": "covet",
			"type": "normal",
			"power": 60,
			"description": "The user endearingly approaches the target, then steals the target's held item."
		},
		"action_description": "Meowth saunters playfully towards Pikachu, its eyes sparkling with mischief. With a charming pounce, it swipes at Pikachu's paws, deftly snatching away the held item. The little cat Pokémon grins, basking in its cleverness as it retreats with the prize, leaving Pikachu momentarily startled."
	}
}

---

###
For detailed API documentation, see the [Pokémon API Guide](./POKEMONGOLANGAPI.md).

---

### SQL Cleanup to repeat tests or demonstrations
delete from challenger_pokemon;
delete from moves;
delete from pokemon_moves;
delete from user_pokemon;
delete from users;
delete from pokedex;

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
