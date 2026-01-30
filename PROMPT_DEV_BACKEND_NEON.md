# Prompt Dev-Backend - Effet Néon (v2.46)

## Contexte

Vous êtes `dev-backend` pour BuzzControl. Vous implémenterez la partie backend de la feature "Effet Néon Catégorie" (Option B - rotation CSS conic-gradient).

**Branche** : feature/effet-neon-categorie (déjà créée)
**Version** : 2.46.0 (valable, sera incrémentée en z pour les tests)
**Statut du plan** : Validé par l'utilisateur

## Objectives

Implémenter les endpoints et la configuration backend pour l'effet néon :

1. ✅ Ajouter `NeonEffectConfig` struct dans `config.go`
2. ✅ Modifier GET/POST `/config.json` avec validation dans `http.go`
3. ✅ Ajouter `NeonEffect` champ dans `GameState` (models.go)
4. ✅ Implémenter broadcast WebSocket `CONFIG_UPDATE` (main.go)
5. ✅ Synchroniser config ↔ GameState
6. ✅ Tests unitaires basiques
7. ✅ Commit atomique

## Tâches précises

### 1. Struct NeonEffectConfig

**Fichier** : `server-go/internal/config/config.go`

Ajouter au début du fichier (avant Config struct) :

```go
type NeonEffectConfig struct {
	Enabled       bool    `json:"enabled"`              // true/false
	ArcWidth      int     `json:"arc_width"`            // 30-180 degrés
	IntensityGap  int     `json:"intensity_gap"`        // 0-100 %
	RotationSpeed float64 `json:"rotation_speed"`       // 1.0-10.0 secondes
}
```

Ajouter dans Config struct (après Storage) :

```go
NeonEffect NeonEffectConfig `json:"neon_effect"`
```

Dans la fonction `LoadConfig()` (après initialisation des autres champs), ajouter :

```go
// Initialize NeonEffect with defaults if not set
if conf.NeonEffect.ArcWidth == 0 {
	conf.NeonEffect.ArcWidth = 60
}
if conf.NeonEffect.IntensityGap == 0 {
	conf.NeonEffect.IntensityGap = 80
}
if conf.NeonEffect.RotationSpeed == 0 {
	conf.NeonEffect.RotationSpeed = 4.0
}
if !conf.NeonEffect.Enabled {
	conf.NeonEffect.Enabled = true
}
```

### 2. Validation HTTP

**Fichier** : `server-go/internal/server/http.go`

Ajouter fonction de validation (avant handler configHandler) :

```go
func (h *HTTPServer) validateNeonConfig(cfg *NeonEffectConfig) error {
	if cfg == nil {
		return nil
	}
	if cfg.ArcWidth < 30 || cfg.ArcWidth > 180 {
		return fmt.Errorf("arc_width must be between 30 and 180 degrees, got %d", cfg.ArcWidth)
	}
	if cfg.IntensityGap < 0 || cfg.IntensityGap > 100 {
		return fmt.Errorf("intensity_gap must be between 0 and 100, got %d", cfg.IntensityGap)
	}
	if cfg.RotationSpeed < 1.0 || cfg.RotationSpeed > 10.0 {
		return fmt.Errorf("rotation_speed must be between 1 and 10 seconds, got %.1f", cfg.RotationSpeed)
	}
	return nil
}
```

Modifier le handler `configHandler` pour traiter POST avec neon_effect :

```go
func (h *HTTPServer) configHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		var update struct {
			NeonEffect *NeonEffectConfig `json:"neon_effect"`
		}

		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Validate if neon_effect is being updated
		if update.NeonEffect != nil {
			if err := h.validateNeonConfig(update.NeonEffect); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			h.config.NeonEffect = *update.NeonEffect
		}

		// Save to file
		if err := h.config.Save(); err != nil {
			http.Error(w, "Failed to save config: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Trigger callback if set
		if h.OnConfigUpdate != nil {
			h.OnConfigUpdate()
		}

		// Return updated config
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(h.config)
	} else if r.Method == "GET" {
		// Existing GET logic - just ensure it includes neon_effect
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(h.config)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
```

### 3. GameState Integration

**Fichier** : `server-go/internal/game/models.go`

Ajouter dans GameState struct (vers la fin, avant la dernière accolade) :

```go
NeonEffect config.NeonEffectConfig `json:"neon_effect"`
```

N'oubliez pas d'importer le package config si pas déjà fait :

```go
import (
	// ...
	"buzzcontrol/internal/config"
)
```

### 4. WebSocket Broadcast

**Fichier** : `server-go/cmd/server/main.go`

A. Ajouter la fonction de broadcast (dans App struct ou avant) :

```go
func (a *App) broadcastConfigUpdate() {
	msg := map[string]interface{}{
		"ACTION": "CONFIG_UPDATE",
		"MSG": map[string]interface{}{
			"neonEffect": a.engine.gameState.NeonEffect,
		},
	}

	msgBytes, _ := json.Marshal(msg)

	a.mu.RLock()
	defer a.mu.RUnlock()

	for client := range a.clients {
		select {
		case client.send <- msgBytes:
		default:
			// Ignore if channel is full
		}
	}
}
```

B. Dans la fonction `main()`, après initialisation de engine, ajouter :

```go
// Synchronize neon config to engine's game state
app.engine.gameState.NeonEffect = app.config.NeonEffect
```

C. Dans la configuration du HTTPServer, ajouter callback :

```go
httpServer.OnConfigUpdate = func() {
	// Sync config to engine
	app.engine.gameState.NeonEffect = app.config.NeonEffect

	// Broadcast to all clients
	app.broadcastConfigUpdate()
}
```

D. Optionnel : Inclure NeonEffect dans `sendStateToClient()` :

```go
// Dans la fonction sendStateToClient(client), modifier le message UPDATE :
stateMsg := map[string]interface{}{
	"ACTION": "UPDATE",
	"MSG": map[string]interface{}{
		"GAME":   a.engine.GetState(),
		"teams":  a.engine.GetTeams(),
		"bumpers": a.engine.GetBumpers(),
	},
	"VERSION": a.config.Version,
}
```

## Tests à implémenter

### Test 1: Validation des valeurs

**Fichier** : `server-go/internal/server/http_test.go`

```go
func TestValidateNeonConfig(t *testing.T) {
	h := &HTTPServer{}

	tests := []struct {
		name    string
		config  *NeonEffectConfig
		wantErr bool
	}{
		{
			name:    "Valid default",
			config:  &NeonEffectConfig{Enabled: true, ArcWidth: 60, IntensityGap: 80, RotationSpeed: 4.0},
			wantErr: false,
		},
		{
			name:    "Arc width too low",
			config:  &NeonEffectConfig{ArcWidth: 20},
			wantErr: true,
		},
		{
			name:    "Arc width too high",
			config:  &NeonEffectConfig{ArcWidth: 200},
			wantErr: true,
		},
		{
			name:    "Intensity gap negative",
			config:  &NeonEffectConfig{IntensityGap: -10},
			wantErr: true,
		},
		{
			name:    "Intensity gap over 100",
			config:  &NeonEffectConfig{IntensityGap: 150},
			wantErr: true,
		},
		{
			name:    "Rotation speed too low",
			config:  &NeonEffectConfig{RotationSpeed: 0.5},
			wantErr: true,
		},
		{
			name:    "Rotation speed too high",
			config:  &NeonEffectConfig{RotationSpeed: 15.0},
			wantErr: true,
		},
		{
			name:    "Nil config",
			config:  nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := h.validateNeonConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateNeonConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
```

### Test 2: Config defaults

**Fichier** : `server-go/internal/config/config_test.go`

```go
func TestNeonConfigDefaults(t *testing.T) {
	cfg := &Config{}
	// Call LoadConfig or similar that initializes defaults

	if cfg.NeonEffect.ArcWidth != 60 {
		t.Errorf("Expected ArcWidth=60, got %d", cfg.NeonEffect.ArcWidth)
	}
	if cfg.NeonEffect.IntensityGap != 80 {
		t.Errorf("Expected IntensityGap=80, got %d", cfg.NeonEffect.IntensityGap)
	}
	if cfg.NeonEffect.RotationSpeed != 4.0 {
		t.Errorf("Expected RotationSpeed=4.0, got %f", cfg.NeonEffect.RotationSpeed)
	}
	if !cfg.NeonEffect.Enabled {
		t.Error("Expected Enabled=true")
	}
}
```

## Checklist de validation

- [ ] NeonEffectConfig struct créée avec 4 champs
- [ ] Valeurs par défaut initialisées correctement
- [ ] validateNeonConfig() couvre tous les cas limites
- [ ] GET /config.json retourne neon_effect
- [ ] POST /config.json accepte et valide neon_effect
- [ ] Erreurs HTTP 400 retournées pour valeurs invalides
- [ ] OnConfigUpdate callback appelé après POST réussi
- [ ] broadcastConfigUpdate() implémentée et testée
- [ ] GameState.NeonEffect synchronisé
- [ ] Logs serveur montrent les changements de config
- [ ] config.json sauvegardé correctement avec neon_effect
- [ ] Tests unitaires passent
- [ ] Version z incrémentée (2.46.1 → 2.46.2)
- [ ] Commit atomique avec message clair

## Fichiers à modifier

```
✏️ server-go/internal/config/config.go        (Struct + defaults)
✏️ server-go/internal/server/http.go          (Validation + handlers)
✏️ server-go/internal/game/models.go          (GameState.NeonEffect)
✏️ server-go/cmd/server/main.go               (Callbacks + broadcast)
✏️ server-go/internal/server/http_test.go     (Tests validation)
✏️ server-go/internal/config/config_test.go   (Tests defaults)
✏️ server-go/config.json                       (Version 2.46.2)
```

## Commandes de test

```bash
# Lancer les tests
cd server-go
go test ./... -v

# Construire et lancer le serveur
go build -o server.exe ./cmd/server
./server.exe

# Tester les endpoints
curl http://localhost/config.json

curl -X POST http://localhost/config.json \
  -H "Content-Type: application/json" \
  -d '{"neon_effect": {"arc_width": 90, "intensity_gap": 60, "rotation_speed": 5.0}}'

# Tester une validation d'erreur
curl -X POST http://localhost/config.json \
  -H "Content-Type: application/json" \
  -d '{"neon_effect": {"arc_width": 25}}'  # Doit retourner 400
```

## Estimation

- Struct + defaults : 10 min
- Validation : 20 min
- HTTP handlers : 15 min
- GameState + sync : 10 min
- Broadcast WebSocket : 15 min
- Tests : 15 min
- **Total** : ~85 min (arrondi 45 min estimé au plan = estimation initiale était optimiste)

## Notes importantes

1. **Imports** : Assurez-vous que `config.NeonEffectConfig` est importable depuis les autres packages
2. **JSON tags** : Utilisez `json:"neon_effect"` (snake_case) pour la cohérence avec le reste de la config
3. **Validation zéro** : Acceptez les valeurs par défaut (0) et les remplacez dans LoadConfig()
4. **Thread-safety** : Utilisez les locks existants (a.mu) quand vous accédez à a.clients
5. **Logging** : Ajoutez des logs Debug/Info pour les changements de config (si logger existant)

## Prochaine étape

Une fois ce backend complété et testé :
1. Faire un commit atomique
2. Signaler à CDP que le backend est prêt
3. CDP lancera le dev-frontend avec le résumé de l'implémentation backend
