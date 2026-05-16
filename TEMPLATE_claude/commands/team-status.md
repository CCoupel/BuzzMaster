# Commande /team-status

Afficher l'état de la team et optionnellement fermer des teammates.

## Usage

```
/team-status
```

## Instructions

### Etape 1 — Lire l'état

```bash
cat .claude/workflow-state.json 2>/dev/null || echo '{"agents":{}}'
```

Si le fichier est absent ou `"agents"` est vide → afficher :

```
Aucun agent actif. Aucune team en cours.
```

Et s'arrêter ici.

### Etape 2 — Afficher le tableau

Calculer pour chaque agent :
- **Durée** : `now - spawned_at`

```
Team : {TEAM_NAME}

| # | Agent        | Statut         | Durée  | Tâche                    |
|---|--------------|----------------|--------|--------------------------|
| 1 | dev-backend  | WORKING        | 12min  | implémentation endpoint  |
| 2 | planner      | SPAWN_PENDING  | 0min   | plan feature X           |
```

Légende des statuts :
- `WORKING` — ACTIF reçu, en cours de traitement
- `SPAWN_PENDING` — spawné, en attente de l'ACK ACTIF (< 60s normal)
- `FAILED` — a envoyé FAILED, action requise

### Etape 3 — Proposer les actions

Afficher uniquement si des agents existent :

```
Actions disponibles :
  [R] Respawner les SPAWN_PENDING expirés (> 60s sans ACTIF)
  [F] Fermer agents spécifiques (saisir les numéros séparés par virgule : 1,3)
  [Q] Quitter sans action
```

Attendre la saisie de l'utilisateur.

### Etape 4a — Respawn [R]

Pour chaque agent `spawn_pending` depuis > 60s :

```
1. TaskStop(<agent>) si le pane existe encore
2. Agent({ team_name, name: "<agent>", subagent_type: "general-purpose",
           prompt: "<prompt de spawn original avec tâche incluse>" })
3. Mettre à jour workflow-state.json : spawned_at: <ISO maintenant>
4. Afficher : "↑ <agent> respawné"
```

### Etape 4b — Fermeture [F]

Pour chaque agent sélectionné :

```
1. TaskStop(<agent>)
2. Supprimer l'entrée de workflow-state.json — écrire immédiatement
3. Afficher : "✓ <agent> stoppé"
```

> Ne jamais arrêter un agent `WORKING` sans confirmation explicite de l'utilisateur.

### Etape 5 — Résumé

```
Terminé.
  Agents actifs restants : N
```

## Règles

- Toujours écrire `workflow-state.json` sur disque après chaque modification
- Ne pas fermer un agent `WORKING` sans confirmation utilisateur explicite
- Un agent absent du JSON s'est auto-fermé normalement après son DONE
