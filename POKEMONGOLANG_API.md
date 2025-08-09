# PokémonGolang REST API Documentation (v0.1)

## Overview
Backend service for a Pokémon-like game. Users register, log in, catch Pokémon, choose a challenger Pokémon, and initiate lightweight battles. Sessions are cookie-based; CSRF protection is enforced on authenticated endpoints.

**Base URL (local dev):** `http://localhost:8080`

## Environment
- `DB_URL` – Postgres connection string (required)
- `BATTLE_AI` – `on` to enable AI-generated battle descriptions; anything else uses plain text
- `BATTLE_AI_MODEL` – OpenAI model name (default: `gpt-4o-mini`)

## Auth & Session
- On successful login, server sets two cookies:
  - `session_token` (HTTP-only): identifies the session (expires ~24h)
  - `csrf_token` (readable by JS): value must be echoed in header `X-CSRF-Token` for **all authenticated requests**
- Authenticated routes also require the `session_token` cookie to be present and valid.

## Common Errors
- `400 Bad Request` – Missing/invalid form fields
- `401 Unauthorized` – Missing/invalid session or CSRF token
- `404 Not Found` – Resource not found (e.g., no active Pokémon or moves)
- `405 Method Not Allowed` – Incorrect HTTP method for the route
- `409 Conflict` – Resource already exists (e.g., username taken)
- `500 Internal Server Error` – Server/DB error

## Conventions
- **Form bodies**: `application/x-www-form-urlencoded` (not multipart) for POSTs
- **Headers for authed routes**: `X-CSRF-Token: <csrf_token_cookie_value>`
- **Cookies**: Include both `session_token` and (client reads) `csrf_token`
- **IDs**: Pokémon identifier may be numeric ID or name where noted

---

## Endpoints

### POST /register
Create a new user.

**Body (form):**
- `username` (string, required)
- `password` (string, required)

**Responses:**
- `200` `{ "message": "User registered successfully" }`
- `409` `{ "error": "User already exists" }`

**cURL:**
```bash
curl -X POST http://localhost:8080/register   -d "username=ash" -d "password=pika123"
```

---

### POST /login
Authenticate user and establish session/CSRF cookies.

**Body (form):**
- `username` (string, required)
- `password` (string, required)

**Sets Cookies:** `session_token` (HttpOnly), `csrf_token`

**Responses:**
- `200` `{ "message": "Login successful" }`
- `401` `{ "error": "Invalid login" }`

**cURL (show cookies):**
```bash
curl -i -X POST http://localhost:8080/login   -d "username=ash" -d "password=pika123"
```

> Extract `csrf_token` from `Set-Cookie` and send it back as `X-CSRF-Token` on authed calls.

---

### POST /logout  (Authenticated)
Invalidate session and clear cookies.

**Headers:** `X-CSRF-Token: <csrf_token>`

**Responses:**
- `200` `{ "message": "Logged out successfully" }`
- `401` `{ "error": "Unauthorized" }`

**cURL:**
```bash
curl -X POST http://localhost:8080/logout   -H "X-CSRF-Token: $CSRF"   --cookie "session_token=$SESSION" --cookie "csrf_token=$CSRF"
```

---

### POST /protected  (Authenticated)
Test endpoint to verify auth.

**Headers:** `X-CSRF-Token: <csrf_token>`

**Responses:**
- `200` `{ "message": "Hello <username>, you are making a protected call!" }`
- `401` Unauthorized

---

### POST /catch  (Authenticated)
Catch a Pokémon and set it **active** (also deactivates others). Party size capped at 6.

**Headers:** `X-CSRF-Token: <csrf_token>`

**Body (form):**
- `pokemon_identifier` (string, required) — numeric ID or name (e.g., `6` or `charizard`)

**Responses:**
- `200` `{ "message": "Pokemon caught successfully", "pokemon_id": <int>, "pokemon_name": "<name>", "user_username": "<user>" }`
- `400` `{ "error": "pokemon_identifier is required" }`
- `400` `{ "error": "You can only have at most six pokemon in your party" }`
- `401`, `500` on failures

**Notes:**
- If Pokémon isn’t in local DB, service fetches from PokéAPI and inserts (`pokedex` table). Also selects up to 4 **damaging** moves (prefers same-type), storing them and linking via join table.

**cURL:**
```bash
curl -X POST http://localhost:8080/catch   -H "X-CSRF-Token: $CSRF"   --cookie "session_token=$SESSION" --cookie "csrf_token=$CSRF"   -d "pokemon_identifier=charizard"
```

---

### POST /challenge  (Authenticated)
Choose (or replace) the challenger Pokémon for the user.

**Headers:** `X-CSRF-Token: <csrf_token>`

**Body (form):**
- `pokemon_identifier` (string, required) — numeric ID or name

**Responses:**
- `200` `{ "message": "Challenge initiated successfully", "pokemon_id": <int>, "pokemon_name": "<name>", "user_username": "<user>" }`
- `401`, `500`

**Behavior:** Removes previous challenge (if any) and links the new challenger to the user with full stats and current HP.

---

### GET /GetUserPokemon  (Authenticated)
List the user’s party with stats and active flag.

**Headers:** `X-CSRF-Token: <csrf_token>`

**Responses:** `200` JSON array of:
```json
{
  "id": 6,
  "name": "charizard",
  "type1": "fire",
  "type2": "flying",
  "hp": 78,
  "attack": 84,
  "defense": 78,
  "special_attack": 109,
  "special_defense": 85,
  "speed": 100,
  "active": true,
  "image_url": "https://.../official-artwork/6.png"
}
```
Errors: `401`, `500`.

**cURL:**
```bash
curl -X GET http://localhost:8080/GetUserPokemon   -H "X-CSRF-Token: $CSRF"   --cookie "session_token=$SESSION" --cookie "csrf_token=$CSRF"
```

---

### POST /ChangeActivePokemon  (Authenticated)
Set an owned Pokémon as the active one.

**Headers:** `X-CSRF-Token: <csrf_token>`

**Body (form):**
- `pokemon_identifier` (int, required) — **Pokédex ID** of an owned Pokémon

**Responses:**
- `200` `{ "message": "Active pokemon changed successfully", "pokemon_id": "<id>", "user_username": "<user>" }`
- `400` on missing/invalid ID, `404` if not owned, `401`, `500`

**Behavior:** Deactivates all, then activates the specified one.

---

### GET /StartBattle  (Authenticated)
Returns battle context (user’s active Pokémon + challenger), with their stats, images, and move lists. No damage is applied.

**Headers:** `X-CSRF-Token: <csrf_token>`

**Responses:** `200`:
```json
{
  "user": {
    "nickname": "Sparky",
    "current_hp": 78,
    "is_active": true,
    "pokemon": {
      "id": 6,
      "name": "charizard",
      "types": ["fire", "flying"],
      "stats": {
        "hp": 78, "attack": 84, "defense": 78,
        "special_attack": 109, "special_defense": 85, "speed": 100
      },
      "image_url": "https://...",
      "moves": [
        {"id": 488, "name": "flame-charge", "power": 50, "type": "fire", "description": "..."}
      ]
    }
  },
  "challenger": {
    "current_hp": 80,
    "pokemon": { /* same shape as above */ }
  }
}
```
Errors: `404` if no active/challenger or no moves; `401`, `500`.

**cURL:**
```bash
curl -X GET http://localhost:8080/StartBattle   -H "X-CSRF-Token: $CSRF"   --cookie "session_token=$SESSION" --cookie "csrf_token=$CSRF"
```

---

### POST /Fight  (Authenticated)
Simulates one turn of flavor text only (no HP change). Returns narrated actions for user and challenger.

**Headers:** `X-CSRF-Token: <csrf_token>`

**Body (form):**
- `move_id` (int as string, required) — one of the user Pokémon’s move IDs

**Responses:** `200`:
```json
{
  "user": {
    "name": "charizard",
    "move_used": {"id": 488, "name": "flame-charge", "type": "fire", "power": 50, "description": "..."},
    "action_description": "charizard used Flame Charge! ..."
  },
  "challenger": {
    "name": "venusaur",
    "move_used": {"id": 80, "name": "vine-whip", "type": "grass", "power": 45, "description": "..."},
    "action_description": "venusaur lashes out with Vine Whip! ..."
  }
}
```
Errors: `400` invalid `move_id`; `404` if no active/challenger/moves; `401`, `500`.

**Notes:** If AI is enabled, descriptions are generated via the configured model with a small timeout and fallback to plain text if AI fails.

**cURL:**
```bash
curl -X POST http://localhost:8080/Fight   -H "X-CSRF-Token: $CSRF"   --cookie "session_token=$SESSION" --cookie "csrf_token=$CSRF"   -d "move_id=488"
```

---

## Data Notes & Selection Rules
- Pokémon data fetched from PokéAPI: base stats, types, and official artwork URL (sprites.other.official-artwork.front_default) cached in `pokedex`.
- Move selection on first fetch:
  - Prefer **damaging** moves (power > 0; exclude damage_class `status`).
  - Prefer moves that **match Pokémon’s types**.
  - Skip moves whose latest English description contains the “This move can’t be used…recommended that this move is forgotten…” blurb.
  - Up to 4 moves added; DB is checked before calling PokéAPI. API calls capped defensively.

## Testing Tips
1. `POST /register` → `POST /login` (capture cookies) → authenticated calls with `X-CSRF-Token` set to the `csrf_token` cookie value.
2. Typical flow:
   - `/catch?pokemon_identifier=charizard`
   - `/challenge?pokemon_identifier=venusaur`
   - `/StartBattle`
   - `/Fight?move_id=<one of user move ids>`

## Versioning
This is an early MVP (v0.1). Routes may change; consider prefixing future APIs with `/api/v1/`.
