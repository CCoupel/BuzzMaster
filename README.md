# BuzzControl

Systeme de buzzers sans fil pour jeux de quiz.

---

## Presentation

BuzzControl est une solution complete pour animer des jeux de quiz avec des buzzers physiques sans fil. Le systeme se compose de :

- **BuzzControl** : Serveur central (Go) avec interface web d'administration
- **BuzzClick** : Buzzers individuels (ESP32-C3) communiquant en WiFi

### Cas d'utilisation

- Quiz d'entreprise et team building
- Jeux televises amateurs
- Animations scolaires et pedagogiques
- Soirees quiz entre amis

---

## Fonctionnalites

### Types de Questions

| Type | Description |
|------|-------------|
| **NORMAL** | Question ouverte avec reponse libre |
| **QCM** | 4 choix colores (Rouge, Vert, Jaune, Bleu) |
| **MEMORY** | Jeu de paires a memoriser |

### Interface d'Administration

- Gestion des questions (creation, edition, suppression)
- Gestion des equipes et joueurs
- Controle du jeu en temps reel (START, PAUSE, STOP)
- Attribution des points (joueur ou equipe)
- Historique complet des parties
- Palmares par categorie
- Sauvegarde et restauration des donnees

### Affichage TV

- Vue Question : affichage de la question, timer, media
- Vue Scores : classement des equipes avec podium
- Vue Joueurs : classement individuel
- Fonds d'ecran personnalisables avec rotation automatique
- Mode plein ecran pour videoprojecteur

### Buzzers (BuzzClick)

- 4 boutons colores par buzzer (A, B, C, D)
- Connexion WiFi automatique
- LED de statut
- Temps de reaction precis (microsecondes)
- Batterie longue duree

### Systeme de Points

- Points configurables par question
- Attribution au joueur OU a l'equipe
- Bonus de completion (Memory)
- Penalites d'erreur (optionnel)
- Historique complet avec recalcul automatique

---

## Installation

### Option 1 : Executable Portable (Recommande)

1. Telecharger la derniere release :
   - `buzzcontrol-vX.Y.Z-windows-amd64.exe` (Windows)
   - `buzzcontrol-vX.Y.Z-linux-arm64` (Raspberry Pi)

2. Placer l'executable dans un dossier dedie

3. Lancer l'executable :
   ```bash
   # Windows
   ./buzzcontrol-vX.Y.Z-windows-amd64.exe

   # Raspberry Pi
   chmod +x buzzcontrol-vX.Y.Z-linux-arm64
   ./buzzcontrol-vX.Y.Z-linux-arm64
   ```

4. Ouvrir dans le navigateur :
   - Administration : http://localhost/
   - Affichage TV : http://localhost/tv

### Option 2 : Compilation depuis les Sources

```bash
# Cloner le depot
git clone https://bitbucket.org/ccoupel/buzzcontrol.git
cd buzzcontrol/server-go

# Installer les dependances frontend
cd web && npm install && cd ..

# Build
./build-release.ps1  # Windows
./build-release.sh   # Linux/macOS

# Les executables sont dans releases/
```

---

## Utilisation Rapide

### 1. Creer des Questions

1. Aller sur l'onglet **Questions**
2. Remplir le formulaire (titre, reponse, type, points, temps)
3. Ajouter une image (optionnel)
4. Cliquer **Sauvegarder**

### 2. Configurer les Equipes

1. Aller sur l'onglet **Equipes**
2. Les equipes sont pre-configurees (6 equipes par defaut)
3. Glisser-deposer les joueurs dans les equipes
4. Assigner les couleurs de reponse pour le mode QCM

### 3. Lancer une Partie

1. Aller sur l'onglet **Jeu**
2. Selectionner une question (clic)
3. Attendre que les buzzers soient prets (phase PREPARE)
4. Cliquer **START** pour lancer le timer
5. Un joueur buzze → le jeu se met en pause
6. Cliquer sur le joueur pour attribuer les points
7. Cliquer **REPONSE** pour reveler la reponse

### 4. Afficher sur TV/Videoprojecteur

1. Ouvrir http://localhost/tv sur l'ecran de diffusion
2. Passer en plein ecran (F11)
3. L'affichage se synchronise automatiquement avec l'admin

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     BUZZCONTROL SERVER                       │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐        │
│  │  HTTP   │  │WebSocket│  │   TCP   │  │   UDP   │        │
│  │  :80    │  │   /ws   │  │  :1234  │  │  :1234  │        │
│  └────┬────┘  └────┬────┘  └────┬────┘  └────┬────┘        │
│       │            │            │            │              │
│       └────────────┴─────┬──────┴────────────┘              │
│                          │                                   │
│                    ┌─────┴─────┐                            │
│                    │  ENGINE   │                            │
│                    │  (Game    │                            │
│                    │   Logic)  │                            │
│                    └───────────┘                            │
└─────────────────────────────────────────────────────────────┘
        │                 │                    │
        ▼                 ▼                    ▼
   ┌─────────┐      ┌──────────┐        ┌──────────┐
   │ Browser │      │ Browser  │        │ BuzzClick│
   │ (Admin) │      │   (TV)   │        │ (Buzzer) │
   └─────────┘      └──────────┘        └──────────┘
```

### Technologies

| Composant | Technologie |
|-----------|-------------|
| Serveur | Go 1.21+ |
| Frontend | React 18 + Vite |
| Buzzers | ESP32-C3 + Arduino |
| Communication | WebSocket, TCP, UDP |
| Stockage | JSON (fichiers locaux) |

---

## Configuration

### Fichier `config.json`

```json
{
  "version": "2.36.0",
  "server": {
    "http_port": 80,
    "tcp_port": 1234
  }
}
```

### Ports Reseau

| Port | Protocole | Usage |
|------|-----------|-------|
| 80 | HTTP | Interface web |
| 80 | WebSocket | Communication temps reel |
| 1234 | TCP | Connexion buzzers |
| 1234 | UDP | Broadcast (START/STOP) |
| 53 | DNS | Portail captif (optionnel) |

---

## Documentation

| Document | Description |
|----------|-------------|
| [CLAUDE.md](CLAUDE.md) | Reference technique complete |
| [CHANGELOG.md](CHANGELOG.md) | Historique des versions |
| [docs/ADMIN_GUIDE.md](docs/ADMIN_GUIDE.md) | Guide utilisateur |
| [docs/DEV_PROCEDURE.md](docs/DEV_PROCEDURE.md) | Procedure de developpement |
| [docs/QUALIF_PROCEDURE.md](docs/QUALIF_PROCEDURE.md) | Procedure de qualification |
| [docs/RELEASE_PROCEDURE.md](docs/RELEASE_PROCEDURE.md) | Procedure de release |
| [docs/TEST_PROCEDURE.md](docs/TEST_PROCEDURE.md) | Procedure de tests |
| [docs/GAME_STATE_MACHINE.md](docs/GAME_STATE_MACHINE.md) | Machine d'etat du jeu |

---

## Structure du Depot

```
buzzcontrol/
├── server-go/              # Serveur Go
│   ├── cmd/server/         # Point d'entree
│   ├── internal/           # Code metier
│   │   ├── game/           # Logique de jeu
│   │   ├── server/         # HTTP, WebSocket, TCP, UDP
│   │   └── protocol/       # Parsing messages
│   ├── web/                # Frontend React
│   └── data/               # Donnees runtime
├── src/                    # Firmware (legacy ESP32)
│   ├── BuzzControl/        # Serveur ESP32-S3 (obsolete)
│   └── BuzzClick/          # Client buzzer ESP32-C3
├── docs/                   # Documentation
└── CLAUDE.md               # Reference technique
```

---

## Contribuer

1. Fork le depot
2. Creer une branche (`git checkout -b feature/ma-fonctionnalite`)
3. Suivre la [procedure de developpement](docs/DEV_PROCEDURE.md)
4. Commit (`git commit -m 'feat: Ma fonctionnalite'`)
5. Push (`git push origin feature/ma-fonctionnalite`)
6. Creer une Pull Request

---

## Licence

Projet prive - Tous droits reserves.

---

## Auteurs

- **Cyril Coupel** - Developpeur principal
- **Claude (Anthropic)** - Assistant IA

---

## Support

Pour signaler un bug ou demander une fonctionnalite :
- Ouvrir une issue sur le depot Bitbucket
- Contacter l'auteur directement
