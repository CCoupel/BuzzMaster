# Instructions Dev-Backend - Effet Néon (v2.46)

## Objectif

Implémenter la configuration de l'effet néon au backend :
- Struct NeonEffectConfig dans config.go
- Endpoints GET/POST /config.json avec validation
- GameState synchronisé
- Broadcast WebSocket CONFIG_UPDATE

## Tâches détaillées

### 1. NeonEffectConfig Struct

**Fichier** : `server-go/internal/config/config.go`

Ajouter avant la struct `Config` :

```go
type NeonEffectConfig struct {
	Enabled       bool    `json:"enabled"`              // Activer/désactiver l'effet
	ArcWidth      int     `json:"arc_width"`            // Largeur arc en degrés (30-180, défaut: 60)
	IntensityGap  int     `json:"intensity_gap"`        // Écart intensité (0-100, défaut: 80)
	RotationSpeed float64 `json:"rotation_speed"`       // Vitesse rotation en secondes (1-10, défaut: 4)
}
```

Ajouter dans `Config` struct :

```go
type Config struct {
	// ... champs existants ...
	NeonEffect NeonEffectConfig `json:"neon_effect"`
}
```

Initialiser dans la fonction `LoadConfig()` :

```go
// Valeurs par défaut pour NeonEffect
if conf.NeonEffect.ArcWidth == 0 {
	conf.NeonEffect.ArcWidth = 60
}
if conf.NeonEffect.IntensityGap == 0 {
	conf.NeonEffect.IntensityGap = 80
}
if conf.NeonEffect.RotationSpeed == 0 {
	conf.NeonEffect.RotationSpeed = 4.0
}
conf.NeonEffect.Enabled = true // Par défaut activé
```

### 2. Validation et Handler HTTP

**Fichier** : `server-go/internal/server/http.go`

Créer fonction de validation :

```go
func (h *HTTPServer) validateNeonConfig(config *NeonEffectConfig) error {
	if config.ArcWidth < 30 || config.ArcWidth > 180 {
		return fmt.Errorf("arc_width doit être entre 30 et 180 degrés, reçu: %d", config.ArcWidth)
	}
	if config.IntensityGap < 0 || config.IntensityGap > 100 {
		return fmt.Errorf("intensity_gap doit être entre 0 et 100, reçu: %d", config.IntensityGap)
	}
	if config.RotationSpeed < 1.0 || config.RotationSpeed > 10.0 {
		return fmt.Errorf("rotation_speed doit être entre 1 et 10 secondes, reçu: %.1f", config.RotationSpeed)
	}
	return nil
}
```

Modifier le handler `configHandler` pour POST :

```go
func (h *HTTPServer) handleConfigUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		var updatedConfig struct {
			NeonEffect *NeonEffectConfig `json:"neon_effect"`
		}

		if err := json.NewDecoder(r.Body).Decode(&updatedConfig); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Si neon_effect est fourni, valider et appliquer
		if updatedConfig.NeonEffect != nil {
			if err := h.validateNeonConfig(updatedConfig.NeonEffect); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			h.config.NeonEffect = *updatedConfig.NeonEffect
		}

		// Sauvegarder config au fichier
		if err := h.config.Save(); err != nil {
			http.Error(w, "Failed to save config", http.StatusInternalServerError)
			return
		}

		// Notifier les clients WebSocket
		if h.OnConfigUpdate != nil {
			h.OnConfigUpdate()
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(h.config)
	}
}
```

### 3. GameState Synchronisation

**Fichier** : `server-go/internal/game/models.go`

Ajouter dans `GameState` struct :

```go
type GameState struct {
	// ... champs existants ...
	NeonEffect config.NeonEffectConfig `json:"neon_effect"`
}
```

### 4. App/Engine Synchronisation

**Fichier** : `server-go/cmd/server/main.go`

Dans la fonction `main()` ou init, initialiser le GameState avec la config :

```go
// Au démarrage, synchroniser neon_effect
app.engine.gameState.NeonEffect = app.config.NeonEffect
```

Ajouter callback `OnConfigUpdate` au HTTP server :

```go
httpServer.OnConfigUpdate = func() {
	// Synchroniser la config au GameState
	app.engine.gameState.NeonEffect = app.config.NeonEffect

	// Broadcast aux clients
	app.broadcastConfigUpdate()
}
```

Ajouter la fonction de broadcast :

```go
func (a *App) broadcastConfigUpdate() {
	msg := map[string]interface{}{
		"ACTION": "CONFIG_UPDATE",
		"MSG": map[string]interface{}{
			"neonEffect": a.config.NeonEffect,
		},
	}

	a.mu.RLock()
	for client := range a.clients {
		select {
		case client.send <- msg:
		default:
			// Channel full, skip
		}
	}
	a.mu.RUnlock()
}
```

Ajouter le message CONFIG_UPDATE dans le broadcast `sendStateToClient()` si nécessaire :

```go
// Dans sendStateToClient, inclure la config neon
stateMsg := map[string]interface{}{
	"ACTION": "UPDATE",
	"MSG": map[string]interface{}{
		"GAME":      a.engine.GetState(),
		"teams":     a.engine.GetTeams(),
		"bumpers":   a.engine.GetBumpers(),
		"neonEffect": a.config.NeonEffect,
	},
	"VERSION": a.config.Version,
}
```

## Tests attendus

### Test unitaire

```go
func TestNeonConfigValidation(t *testing.T) {
	h := &HTTPServer{}

	tests := []struct {
		name    string
		config  NeonEffectConfig
		wantErr bool
	}{
		{"Valid", NeonEffectConfig{Enabled: true, ArcWidth: 60, IntensityGap: 80, RotationSpeed: 4.0}, false},
		{"Arc too low", NeonEffectConfig{ArcWidth: 20}, true},
		{"Arc too high", NeonEffectConfig{ArcWidth: 200}, true},
		{"Intensity negative", NeonEffectConfig{IntensityGap: -10}, true},
		{"Intensity over 100", NeonEffectConfig{IntensityGap: 120}, true},
		{"Speed too low", NeonEffectConfig{RotationSpeed: 0.5}, true},
		{"Speed too high", NeonEffectConfig{RotationSpeed: 15.0}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := h.validateNeonConfig(&tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateNeonConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
```

### Test HTTP

```
GET /config.json
→ Response inclut neon_effect : { enabled: true, arc_width: 60, intensity_gap: 80, rotation_speed: 4.0 }

POST /config.json avec { neon_effect: { arc_width: 90, ... } }
→ 200 OK, config sauvegardée, fichier config.json modifié

POST /config.json avec { neon_effect: { arc_width: 25 } }  (invalide)
→ 400 Bad Request, erreur "arc_width doit être entre 30 et 180"
```

## Validation avant de terminer

Checklist :

- [ ] NeonEffectConfig struct créée avec 4 champs
- [ ] Config struct contient NeonEffect
- [ ] Valeurs par défaut initialisées (60, 80, 4.0)
- [ ] validateNeonConfig() implémentée et testée
- [ ] POST /config.json accepte et valide neon_effect
- [ ] GET /config.json retourne neon_effect
- [ ] OnConfigUpdate callback défini
- [ ] broadcastConfigUpdate() envoie CONFIG_UPDATE à tous les clients
- [ ] GameState.NeonEffect synchronisé
- [ ] config.json peut être reloadée avec neon_effect

## Commits

- Commit atomique : "feat(config): Add neon effect configuration with validation"
- Version z incrémentée : 2.46.2 (en dev)

## Prochaine étape

Une fois ce backend complété :
1. Tester les endpoints HTTP
2. Vérifier les logs du serveur
3. Passer au dev-frontend avec le résumé de ce qui a été implémenté
