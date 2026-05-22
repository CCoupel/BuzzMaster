# Commande /team-status

Afficher l'état de la team et optionnellement fermer des teammates.

## Instructions

### Etape 1 — Lire l'état natif

Utiliser les outils natifs Claude Code pour lister les tâches/agents actifs dans la team `{TEAM_NAME}`.

Si aucun teammate actif → afficher `"Aucun teammate actif."` et s'arrêter.

### Etape 2 — Afficher le tableau

```
Team : {TEAM_NAME}

| # | Teammate    | État    |
|---|-------------|---------|
| 1 | dev-backend | actif   |
| 2 | planner     | actif   |
```

### Etape 3 — Actions

```
[F] Fermer des teammates (numéros séparés par virgule)
[Q] Quitter
```

### Etape 4 — Fermeture [F]

```
TaskStop(<nom>) pour chaque teammate sélectionné
```

## Règles

- Ne pas fermer un teammate en cours de tâche sans confirmation explicite de l'utilisateur
