# Backlog BuzzMaster

> **Suivi via GitHub Issues** : Le backlog est suivi via [GitHub Issues (label `backlog`)](https://github.com/CCoupel/BuzzMaster/issues?q=label%3Abacklog).
> Ce dossier contient les **spécifications détaillées** référencées par les issues.

Ce dossier contient les spécifications détaillées de toutes les fonctionnalités du projet BuzzMaster, organisées par statut.

## Structure

```
backlog/
├── TODO/           # Spécifications des fonctionnalités planifiées
├── En-Cours/       # Spécifications des fonctionnalités en cours
├── DONE/           # Spécifications des fonctionnalités complétées
├── REMOVED/        # Spécifications des fonctionnalités abandonnées (gardées pour mémoire)
└── README.md       # Ce fichier
```

## TODO (Planifié)

| Fichier | Description |
|---------|-------------|
| [usb-modal-layout-compact.md](TODO/usb-modal-layout-compact.md) | Réorganisation modale USB : sans scroll, bouton Flash USB sur même ligne que "Envoyer et configurer" |
| [websocket-broadcast-filtre.md](TODO/websocket-broadcast-filtre.md) | Filtrage des broadcasts WebSocket par type de client |
| [generateur-ia.md](TODO/generateur-ia.md) | Générateur de jeu complet via IA |
| [metadata-binaires.md](TODO/metadata-binaires.md) | Métadonnées dans les binaires (nom, version, icône) |

## En-Cours

| Fichier | Description |
|---------|-------------|
| [memory-game.md](En-Cours/memory-game.md) | Jeu de mémoire avec paires (Phases 1-6 complétées, Phase 7 restante) |

## DONE (Complété)

| Fichier | Version | Description |
|---------|---------|-------------|
| [udp-broadcast-server-discovery.md](DONE/udp-broadcast-server-discovery.md) | v3.2.0 | Découverte automatique de l'IP serveur via UDP heartbeat (DHCP friendly, LED boot sequence enrichie) |
| [buzzer-firmware-ota-update.md](DONE/buzzer-firmware-ota-update.md) | v3.1.0 | Mise à jour OTA du firmware des buzzers (détection version, pastille obsolète, upload + restore embarqué) |
| [buzzer-protocol-websocket.md](DONE/buzzer-protocol-websocket.md) | v3.0.0 | Migration protocole buzzers TCP → WebSocket (hub dédié /ws/buzzer) |
| [vjoueur-qcm-multicolore.md](DONE/vjoueur-qcm-multicolore.md) | v2.53.0 | VJoueur valide pour les 4 couleurs QCM + buzz direct sur écran |
| [admin-joueur-card-style.md](DONE/admin-joueur-card-style.md) | v2.49.0 | Style neutre gris pour cartes joueurs (page admin Jeu) |
| [effet-neon-categorie.md](DONE/effet-neon-categorie.md) | v2.46.0 | Effet néon couleur catégorie sur TV et VJoueur |
| [vjoueur-websocket-identification.md](DONE/vjoueur-websocket-identification.md) | v2.47.0 | Identification WebSocket des VJoueurs (type vplayer distinct) |
| [tri-rapidite-reponse.md](DONE/tri-rapidite-reponse.md) | v2.44.1 | Tri équipes/joueurs par rapidité de buzz |
| [page-joueur.md](DONE/page-joueur.md) | v2.45.0 | Interface personnalisée pour jouer depuis smartphone (Phase 1) |
| [page-logs.md](DONE/page-logs.md) | v2.43.0 | Affichage des logs serveur en temps réel (WebSocket dédiée) |
| [mode-demo.md](DONE/mode-demo.md) | v2.40.0 | Mode démonstration avec données complètes |
| [qcm-indices-penalites.md](DONE/qcm-indices-penalites.md) | v2.38.0 | Indices automatiques pour QCM avec pénalités |
| [categories-questions.md](DONE/categories-questions.md) | v2.34.0 | Système de catégorisation et palmarès |
| [affichage-tv.md](DONE/affichage-tv.md) | v2.30.0 | Synchronisation des fonds d'écran |
| [timer-gameplay.md](DONE/timer-gameplay.md) | v2.29.0 | Décompte de préparation avant timer |
| [debug-tests.md](DONE/debug-tests.md) | v2.28.0 | Fonctionnalités de test sans buzzers |
| [navbar-menu-connexion.md](DONE/navbar-menu-connexion.md) | v2.49.0 | Menu déroulant abeille dans la navbar (Config, Logs, Backup, MAJ) |
| [gestion-scores.md](DONE/gestion-scores.md) | v2.18.0 | Points d'équipe dissociés des points joueurs |

## REMOVED (Abandonné)

| Fichier | Raison |
|---------|--------|
| [buzzer-wifi-provisioning-smartconfig.md](REMOVED/buzzer-wifi-provisioning-smartconfig.md) | Remplacé par config WiFi directe USB depuis l'Admin (v3.0.x) |

## Légende des statuts

- **TODO** : Spécification validée, pas encore démarré
- **En-Cours** : Implémentation en cours
- **DONE** : Fonctionnalité implémentée et livrée
- **REMOVED** : Fonctionnalité abandonnée (spécification conservée pour mémoire)

## Contribution

Pour ajouter une nouvelle fonctionnalité au backlog :

1. Créer une issue GitHub avec les labels `enhancement`, `backlog` et `TODO`
   ```bash
   gh issue create --repo CCoupel/BuzzMaster \
       --title "<Titre>" \
       --label "enhancement,backlog,TODO"
   ```
2. Optionnellement, créer un fichier de spécification détaillée dans `backlog/TODO/`
3. Lier le fichier de spec dans l'issue GitHub

## Cycle de vie d'une fonctionnalité

Le cycle de vie est géré via **GitHub Issues** :

```
[TODO] ──► [En-Cours] ──► [DONE] (issue fermée)
  │
  └──────────────────────► [REMOVED] (issue fermée, "not planned")
```

### Labels GitHub

| Action | Labels à modifier |
|--------|-----------------|
| Démarrage | Retirer `TODO`, ajouter `En-Cours` |
| Completion | Retirer `En-Cours`, ajouter `DONE`, fermer l'issue |
| Abandon | Retirer tous labels de statut, ajouter `REMOVED`, fermer "not planned" |

### Fichiers de spécification (optionnels)

Les fichiers dans `backlog/` sont des spécifications de référence. Ils ne sont pas le tracker principal.
Si un fichier existe, mettre à jour le champ `**Statut**` en cohérence avec l'issue GitHub :

```markdown
**Statut** : ✅ Complété (vX.Y.Z)
```
