# Procédure de Développement

Ce document décrit le workflow de développement pour BuzzControl.

---

## Vue d'ensemble du Cycle

```
┌─────────────────┐     Validation     ┌─────────────────┐     Validation     ┌─────────────────┐
│  DEV + TEST     │ ─────────────────► │  QUALIF + TEST  │ ─────────────────► │  RELEASE        │
│  (Développement)│                    │  (Qualification)│                    │  (Production)   │
└─────────────────┘                    └─────────────────┘                    └─────────────────┘
```

**Transition** : Chaque passage à la phase suivante nécessite une validation explicite de l'utilisateur.

---

## 1. Environnement de Développement

### Prérequis

- [ ] Go 1.21+
- [ ] Node.js 18+
- [ ] Git
- [ ] Éditeur (VS Code recommandé)

### Structure des dossiers

```
server-go/
├── cmd/server/       # Point d'entrée + embed
├── internal/         # Code métier
├── web/              # Frontend React
├── data/             # Données runtime (ignoré par git)
├── config.json       # Configuration serveur
└── server.exe        # Exécutable (ignoré par git)
```

---

## 2. Démarrage d'une Session de Dev

### 2.1 Vérifier l'état du dépôt

```bash
cd buzzcontrol
git status
git pull origin main
```

### 2.2 Incrémenter la version de dev

**Fichier** : `server-go/config.json`

```json
{
  "version": "x.y.z"  // Incrémenter z pour chaque session de dev
}
```

**Règle** : `z` est incrémenté à chaque relance du serveur pour tests.

### 2.3 Lancer le serveur en mode développement

```bash
cd server-go

# Option 1 : Build + Run (recommandé)
go build -o server.exe ./cmd/server && ./server.exe

# Option 2 : Run direct (plus lent)
go run ./cmd/server
```

### 2.4 Lancer le frontend en mode dev (hot reload)

```bash
cd server-go/web
npm run dev
# Ouvre http://localhost:5173 avec hot reload
# Le backend doit tourner sur :80
```

---

## 3. Workflow de Développement

### 3.1 Cycle de modification

```
1. Modifier le code
2. Rebuild si nécessaire
3. Tester manuellement
4. Répéter jusqu'à satisfaction
```

### 3.2 Modifications Backend (Go)

```bash
# Arrêter le serveur (Ctrl+C)
# Modifier les fichiers .go
# Rebuild et relancer
go build -o server.exe ./cmd/server && ./server.exe
```

### 3.3 Modifications Frontend (React)

```bash
# Si npm run dev est actif : hot reload automatique
# Sinon :
npm run build --prefix web
# Puis relancer le serveur Go
```

### 3.4 Tests rapides pendant le dev

```bash
# Test unitaire d'un package spécifique
go test ./internal/game/... -v

# Test d'une fonction spécifique
go test ./internal/game/... -v -run TestNomDuTest
```

---

## 4. Conventions de Code

### 4.1 Go

- Noms de fichiers en snake_case : `game_engine.go`
- Noms de fonctions en CamelCase : `ProcessButtonPress()`
- Commentaires en anglais pour le code, français pour les logs utilisateur
- Gestion d'erreurs explicite (pas de panic)

### 4.2 React/JSX

- Composants en PascalCase : `GamePage.jsx`
- Hooks en camelCase : `useWebSocket.js`
- CSS en kebab-case : `.game-page`, `.nav-link`
- Un fichier CSS par composant

### 4.3 Messages de commit (pendant dev)

```bash
# Format court pour les commits de dev
git commit -m "wip: description courte"
git commit -m "fix: correction bug X"
git commit -m "feat: ajout fonctionnalité Y"
```

---

## 5. Debugging

### 5.1 Logs serveur

Les logs s'affichent dans la console du serveur :
- `[HTTP]` : Requêtes HTTP
- `[WebSocket]` : Connexions WebSocket
- `[TCP]` : Connexions buzzers
- `[Engine]` : Logique de jeu
- `[App]` : Application principale

### 5.2 Console navigateur

- F12 → Console pour les erreurs JavaScript
- F12 → Network pour les requêtes réseau
- F12 → Network → WS pour les messages WebSocket

### 5.3 Debug Go avec Delve

```bash
# Installation
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug
dlv debug ./cmd/server
```

### 5.4 Simuler un buzzer (Ctrl+clic)

En mode admin, `Ctrl+clic` sur un joueur simule un appui buzzer.

---

## 6. Gestion des Erreurs Courantes

### Port 80 occupé

```bash
# Trouver le processus
netstat -ano | findstr :80

# Tuer le processus
taskkill /PID <PID> /F
```

### Port 1234 (TCP) occupé

```bash
taskkill /IM server.exe /F
```

### Erreur de build Go

```bash
# Nettoyer le cache
go clean -cache

# Rebuild
go build -v ./cmd/server
```

### Erreur npm

```bash
cd server-go/web
rm -rf node_modules
npm install
```

---

## 7. Checklist Fin de Session Dev

Avant de passer en QUALIF :

- [ ] Code compilé sans erreur
- [ ] Fonctionnalité principale testée manuellement
- [ ] Pas de console.log ou fmt.Println de debug oubliés
- [ ] Pas de fichiers temporaires non désirés

---

## 8. Passage en Phase QUALIF

Quand le développement est terminé :

1. **Informer l'utilisateur** : "Le développement de [fonctionnalité] est terminé"
2. **Attendre validation** : L'utilisateur doit explicitement valider
3. **Passer à** : [QUALIF_PROCEDURE.md](QUALIF_PROCEDURE.md)

**Note** : Ne jamais passer en QUALIF sans accord explicite de l'utilisateur.
