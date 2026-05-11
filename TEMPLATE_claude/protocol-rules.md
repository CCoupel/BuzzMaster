# Règles Critiques Teamleader — NE PAS OUBLIER APRÈS COMPACTAGE

## Nommage des agents

- Le paramètre `name` dans `Task` est TOUJOURS le **nom canonique simple** :
  `qa`, `dev-backend`, `planner`, `deployer`… **Jamais de suffixe** (`qa-1`, `qa-2`…)
- Un rôle = un nom = une adresse `SendMessage` permanente
- Si le système impose un suffixe → l'agent précédent tourne encore → envoyer PING au nom simple

## Protocole PING (avant tout dispatch)

- Toujours `SendMessage({to: "<nom>", content: "PING"})` avant d'envoyer du travail
- Attendre la réponse **30 secondes maximum**
- Réponse `<NOM> ACTIF` → dispatcher via SendMessage
- Pas de réponse après 30s → spawner via `Task` avec le nom canonique

## Watchdog IDLE (singleton)

- **Un seul watchdog** à la fois : vérifier `watchdog_active` dans `workflow-state.json` avant tout `ScheduleWakeup`
- Si `watchdog_active == true` → ne pas en lancer un second
- Le watchdog cible uniquement les agents `status: "idle"` (jamais `"working"`)
- Mesure IDLE depuis `idle_since`, pas depuis `last_order_sent_at`

## Tracking agents — écriture immédiate obligatoire

| Événement | Mise à jour workflow-state.json |
|-----------|--------------------------------|
| Dispatch (SendMessage de travail) | `status: "working"`, `last_order_sent_at: <ISO>`, `idle_since: null` |
| Réception DONE | `status: "idle"`, `idle_since: <ISO>` |
| Envoi shutdown_request | `status: "pending_delete"` |
| Réception shutdown_response | supprimer l'entrée agent |
| TaskStop (pas de réponse au cycle suivant) | supprimer l'entrée agent |

> Écrire sur disque **immédiatement** — jamais en mémoire. Ce fichier est la source de vérité.

## Format de workflow-state.json

```json
{
  "workflow": {
    "type": "FEATURE|BUGFIX|REFACTOR|HOTFIX|DEPLOY|SECU",
    "description": "description courte",
    "phase": "PLAN|DEV|REVIEW|QA|DOC|DEPLOY",
    "cycle": 1,
    "issue_nums": [42, 43],
    "milestone_num": 7,
    "started_at": "2026-01-01T10:00:00Z"
  },
  "watchdog_active": false,
  "agents": {
    "<nom>": {
      "status": "working|idle|pending_delete",
      "last_order_sent_at": "2026-01-01T10:05:00Z",
      "idle_since": null
    }
  }
}
```

> Si aucun workflow actif : `{ "watchdog_active": false, "agents": {} }`
