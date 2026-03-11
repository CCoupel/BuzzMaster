# BuzzControl - Backlog

Fonctionnalités à implémenter pour le projet BuzzMaster.

---

## Organisation

Le backlog est désormais géré via **GitHub Issues** avec le label `backlog` :

- **Issues ouvertes** : Fonctionnalités TODO et En-Cours
- **Issues fermées** : Fonctionnalités complétées (DONE) ou abandonnées (REMOVED)

👉 **[Voir les issues backlog](https://github.com/CCoupel/BuzzMaster/issues?q=label%3Abacklog)**

### Labels de statut

| Label | Description |
|-------|-------------|
| `TODO` | Fonctionnalité planifiée, pas encore démarrée |
| `En-Cours` | Implémentation en cours |
| `DONE` | Fonctionnalité complétée (issue fermée) |
| `REMOVED` | Fonctionnalité abandonnée (issue fermée, "not planned") |

### Labels techniques

| Label | Description |
|-------|-------------|
| `backend` | Serveur Go |
| `frontend` | Interface React |
| `firmware` | Firmware BuzzClick ESP32 |
| `ci-cd` | CI/CD et workflows |
| `memory-game` | Jeu Memory |
| `ai` | Intelligence artificielle |

---

## Spécifications détaillées

Les fichiers de spécification détaillées restent dans le dossier `backlog/` pour référence :

```
backlog/
├── TODO/           # Specs des fonctionnalités planifiées
├── En-Cours/       # Specs des fonctionnalités en cours
├── DONE/           # Specs des fonctionnalités complétées
├── REMOVED/        # Specs des fonctionnalités abandonnées
└── README.md       # Index des spécifications
```

Chaque issue GitHub contient un lien vers la spécification complète dans `backlog/`.

---

## Contribution

Pour proposer une nouvelle fonctionnalité :

1. Créer une issue GitHub avec le label `enhancement` et `backlog`
2. Optionnellement, créer un fichier de spécification détaillée dans `backlog/TODO/`
3. Lier le fichier de spec dans l'issue GitHub

---

## Migration depuis fichiers backlog

Le script `scripts/create-github-issues.sh` transpose les fichiers backlog en issues GitHub.
Prérequis : `gh auth login` (GitHub CLI authentifié).
