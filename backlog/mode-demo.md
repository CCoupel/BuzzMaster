# Mode Demo

**Statut** : ✅ Complété (v2.40.0)

## Description

Mode de démonstration permettant de charger des données complètes pour présenter toutes les fonctionnalités de BuzzControl sans configuration manuelle.

## Fonctionnalités implémentées

- [x] **Endpoint `/load-demo` (POST)**
  - Accessible depuis la page Configuration
  - Bouton "Charger la demo" dans la section "Mode Demo"
  - Réinitialise et charge toutes les données de démonstration

- [x] **Données de démonstration créées**

  | Type | Quantité | Détails |
  |------|----------|---------|
  | Équipes | 6 | Avec TeamPoints pré-remplis |
  | Joueurs | 24 | 4 par équipe, toutes couleurs QCM (A/B/C/D) |
  | Questions | 10 | QCM (avec indices), MEMORY, NORMAL |
  | Catégories | 8 | GEOGRAPHY, ENTERTAINMENT, HISTORY, etc. |
  | Historique | 10 | Événements pour vue PALMARES |
  | Fonds | 3 | Opacités variées (100%, 80%, 60%) |
  | Images | 5 | Embarquées dans l'exécutable |

- [x] **Questions de démonstration**
  - 4 QCM (3 avec `QCM_HINTS_ENABLED: true`)
  - 4 NORMAL (points joueur)
  - 2 MEMORY (pays/capitales, superhéros/pouvoirs)

- [x] **Images embarquées (extraites automatiquement)**

  | Question | Image Question | Image Réponse |
  |----------|----------------|---------------|
  | demo1 | Australie (paysage) | - |
  | demo4 | Chercheur d'or | Tableau périodique |
  | demo7 | Pizza | Carte de l'Italie |

- [x] **Assets embarqués dans l'exécutable**
  - `assets/demo/demo_bg_1.jpg`, `demo_bg_2.jpg`, `demo_bg_3.jpg` (fonds)
  - `assets/demo/demo1_australia.jpg`
  - `assets/demo/demo4_gold_miner.jpg`, `demo4_periodic_table.jpg`
  - `assets/demo/demo7_pizza.jpg`, `demo7_italy.jpg`

## Fichiers implémentés

| Fichier | Rôle |
|---------|------|
| `assets/embed.go` | Embed des assets demo via `//go:embed` |
| `internal/server/http.go` | Endpoint `/load-demo`, callback `OnLoadDemo` |
| `cmd/server/main.go` | `loadDemoData()`, `createDemoQuestions()`, `createDemoBackgrounds()`, `createDemoHistory()` |
| `web/src/pages/ConfigPage.jsx` | Bouton "Charger la demo" |

## Commits associés

- `eca2cde` feat(config): Add demo mode with comprehensive sample data
- `ea75218` feat(demo): Auto-download background images from Unsplash
- `a5b68e8` feat(demo): Embed background images in executable
- `d07c388` feat(demo): Add embedded images for demo questions
- `31e2cc3` style: Replace demo backgrounds with festive images
- `bd4c505` release: BuzzControl v2.40.0

## Version

v2.40.0
