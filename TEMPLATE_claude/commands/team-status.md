# Commande /team-status

Afficher l'état de la team et optionnellement fermer des teammates.

## Usage

```
/team-status
```

## Instructions

### Etape 1 — Lire l'état

```bash
cat .claude/workflow-state.json 2>/dev/null || echo '{"watchdog_active":false,"agents":{}}'
```

Si le fichier est absent ou `"agents"` est vide → afficher :

```
Aucun agent actif. Aucune team en cours.
```

Et s'arrêter ici.

### Etape 2 — Afficher le tableau

Calculer pour chaque agent :
- **Durée** : si `status == "working"` → `now - last_order_sent_at` ; si `status == "idle"` → `now - idle_since`

```
Team : {TEAM_NAME}
Watchdog actif : oui / non

| # | Agent        | Statut         | Durée  |
|---|--------------|----------------|--------|
| 1 | dev-backend  | WORKING        | 12min  |
| 2 | qa           | IDLE           | 8min   |
| 3 | planner      | PENDING_DELETE | 2min   |
```

Légende des statuts :
- `WORKING` — en cours de traitement d'un ordre
- `IDLE` — en attente d'ordres (watchdog surveille)
- `PENDING_DELETE` — shutdown_request envoyé, en attente de réponse

### Etape 3 — Proposer les actions

Afficher uniquement si des agents existent :

```
Actions disponibles :
  [P] Vérifier connectivité — PING-STATUS individuel à chaque agent (confirme présence et état)
  [A] Fermer tous les IDLE
  [N] Fermer agents spécifiques (saisir les numéros séparés par virgule : 1,3)
  [Q] Quitter sans action
```

Attendre la saisie de l'utilisateur.

### Etape 4a — PING-STATUS broadcast [P]

Utile après un compactage de contexte ou si le statut semble incohérent.  
Effectue **deux passes** : découverte des orphelins + vérification des connus.

**Passe 1 — Découverte des orphelins**

Noms canoniques documentés :
```
planner, dev-backend, dev-frontend, dev-firmware, dev-plugin,
test-writer, code-reviewer, qa, doc-updater, deployer, security, infra
```

Envoyer PING-STATUS aux canoniques **absents de `workflow-state.json`** (un seul bloc) :
```
SendMessage({to: "<canonique-absent1>", content: "PING-STATUS"})
SendMessage({to: "<canonique-absent2>", content: "PING-STATUS"})
…
```
Attendre 30s — ceux qui répondent → ajouter dans `workflow-state.json` + afficher `↩ <agent> redécouvert`.  
Ceux qui ne répondent pas → ignorés (jamais spawnés dans cette session).

**Passe 2 — Vérification des agents connus**

Envoyer PING-STATUS à tous les agents présents dans `workflow-state.json` :
```
SendMessage({to: "<agent>", content: "PING-STATUS — répond PONG(IDLE) si tu es IDLE, PONG(WORKING) si tu as une tâche assignée, ou PONG(IDLE-2) si je t'ai déjà envoyé un PING-STATUS et que ton état n'a pas changé"})
```
(répéter pour chaque agent)
Attendre 30s — traiter les réponses :

| Réponse | Action |
|---------|--------|
| `PONG(WORKING)` | `status: "working"` dans workflow-state.json |
| `PONG(IDLE)` | `status: "idle"` dans workflow-state.json |
| `PONG(IDLE-2)` | afficher `⚠ <agent> IDLE depuis 2 cycles` — proposer fermeture |
| Pas de réponse | supprimer l'entrée + afficher `✗ <agent> non joignable — retiré` |

Écrire `workflow-state.json` immédiatement après chaque modification.

Après traitement, ré-afficher le tableau mis à jour (Etape 2) et reproposer le menu.

### Etape 4b — Appliquer la fermeture [A] ou [N]

Pour chaque agent sélectionné (ou tous les IDLE si [A]) :

```
1. SendMessage({to: "<agent>", content: "shutdown_request"})
2. Mettre status: "pending_delete" dans workflow-state.json — écrire immédiatement
3. Afficher : "⏳ shutdown_request envoyé à <agent>"
```

Attendre **30 secondes** les `shutdown_response` :
- À chaque `shutdown_response` reçu → supprimer l'entrée de `workflow-state.json` + afficher `✓ <agent> terminé`
- Après 30s, pour tout agent encore `"pending_delete"` :
  - `TaskStop(<agent>)`
  - Supprimer l'entrée de `workflow-state.json`
  - Afficher `✓ <agent> stoppé (pas de réponse)`

Écrire `workflow-state.json` immédiatement après chaque modification.

### Etape 5 — Résumé

Après fermeture [A] ou [N] :

```
Fermeture terminée.

  Agents terminés proprement : N (shutdown_response reçu)
  Agents stoppés de force    : N (TaskStop)
  Agents encore actifs       : N (WORKING — non touchés)
```

## Règles

- Ne jamais arrêter un agent `WORKING` sans confirmation explicite de l'utilisateur
- Toujours écrire `workflow-state.json` sur disque après chaque modification
- Si `watchdog_active == true` après la fermeture → le watchdog se chargera des éventuels restants au prochain cycle
- Le [P] envoie un PING-STATUS individuel à chaque agent (point-à-point, pas un broadcast natif) — ne ferme aucun agent, retire uniquement les entrées d'agents disparus
