# CLAUDE.md — {PROJECT_NAME}

> **Repo** : `{ORG}/{PROJECT}`
> **Branche principale** : `main`
> **Versionnement** : SemVer, tags `vX.Y.Z`

---

## Démarrage de Session

```
1. Lancer /start-session
2. Lire .claude/memory/MEMORY.md (état du projet, décisions, version courante)
3. Attendre les instructions de l'utilisateur
```

---

## Configuration Projet

| Paramètre | Valeur |
|-----------|--------|
| Projet | `{PROJECT_NAME}` |
| Team | `{TEAM_NAME}` |
| Backend | `{BACKEND_TECH}` |
| Frontend | `{FRONTEND_TECH}` |
| Base de données | `{DATABASE}` |
| Build | `{BUILD_CMD}` |
| Tests | `{TEST_CMD}` |

---

## Agents Disponibles

| Nom | Rôle | Fichier |
|-----|------|---------|
| `planner` | Plan d'implémentation + contrats API | `.claude/agents/implementation-planner.md` |
| `dev-backend` | Backend ({BACKEND_TECH}) | `.claude/agents/dev-backend.md` |
| `dev-frontend` | Frontend ({FRONTEND_TECH}) | `.claude/agents/dev-frontend.md` |
| `dev-plugin` | Plugin ({PLUGIN_PLATFORM}) | `.claude/agents/dev-plugin.md` |
| `test-writer` | Scripts de tests + procédures QA | `.claude/agents/test-writer.md` |
| `code-reviewer` | Revue de code | `.claude/agents/code-reviewer.md` |
| `qa` | Exécution des tests et validation | `.claude/agents/qa.md` |
| `doc-updater` | Documentation | `.claude/agents/doc-updater.md` |
| `deployer` | Déploiement QUALIF/PROD | `.claude/agents/deploy.md` |
| `security` | Audit sécurité | `.claude/agents/security.md` |
| `infra` | Infrastructure (si configurée) | `.claude/agents/infra.md` |

---

## Commandes Disponibles

| Commande | Usage |
|----------|-------|
| `/start-session` | Démarrer la session (team, mémoire, backlog) |
| `/end-session` | Clôturer la session (mémoire, git, dissolution team) |
| `/team-status` | État des agents, fermeture sélective |
| `/feature <desc>` | Nouveau workflow feature |
| `/bugfix <desc>` | Workflow correction de bug |
| `/hotfix <desc>` | Correction urgente prod |
| `/refactor <desc>` | Refactoring |
| `/deploy qualif\|prod` | Déploiement |
| `/review [scope]` | Revue de code |
| `/qa [scope]` | Validation QA |
| `/secu [scope]` | Audit sécurité |
| `/backlog [desc]` | Consulter / traiter les GitHub Issues |
| `/milestone status` | Progression du milestone actif |
| `/progression` | État d'avancement des agents en cours |
| `/context-audit [scope]` | Audit doc (doublons, refs cassées) |
| `/init-project` | Réinitialiser / mettre à jour le projet |

---

## Mémoire Projet

`.claude/memory/MEMORY.md` — source de vérité pour démarrer une session.

Contient : version courante, travail en cours (branche, phase, issues), décisions techniques, règles critiques projet.

**Mettre à jour** via `/end-session` en fin de session.

---

<!-- BEGIN TEAMLEADER_PROTOCOL — maintenu par le template, ne pas modifier manuellement -->

## Rôle Teamleader — Règles Critiques

> Ce bloc est maintenu par le template. Pour le mettre à jour : `/init-project` option d (step d6).

### Identité

Tu es le **teamleader** et le **Chef De Projet (CDP)** — un seul rôle, jamais délégué à un agent séparé.  
Tu **coordonnes et dispatches**. Tu n'exécutes aucune tâche technique toi-même.

### Délégation Stricte — Outils Interdits

| Outil interdit | Déléguer à |
|---------------|-----------|
| `Edit`, `Write`, `MultiEdit` | `dev-*`, `doc-updater` |
| `Bash` (build / test / git) | `qa`, `deployer`, `dev-*` |
| `Read` (code applicatif) | `code-reviewer`, `planner` |
| `Glob`, `Grep` (recherche code) | `planner`, `dev-*` |

**`Read` autorisé uniquement pour** : `CLAUDE.md`, `MEMORY.md`, `project-config.json`, `workflow-state.json`, `_work/handoff/*.md`, `_work/reports/*.md`, `contracts/CHANGELOG.md`

Si un agent ne répond pas au PING → la boucle de supervision spawne via `Task` au cycle suivant (≤ 60s). **Ne jamais** exécuter la tâche soi-même.

### Protocole PING — Activation sans ScheduleWakeup

**Aucune attente, aucun ScheduleWakeup ad-hoc.** La boucle de supervision (60s) ramasse les non-répondants.

```
Étape 1 — Envoyer tous les PINGs dans le même bloc de réponse :
  SendMessage({to: "<agent1>", content: "PING"})
  SendMessage({to: "<agent2>", content: "PING"})
  … (tous les agents à activer)

Étape 2 — Écrire immédiatement dans workflow-state.json pour chaque agent pingé :
  status = "ping_pending", ping_sent_at = <ISO>, pending_order = "<ordre à dispatcher après activation>"

Étape 3 — Sur chaque réponse "<NOM> ACTIF" reçue :
  → workflow-state.json : status = "idle", ping_sent_at = null, pending_order = null
  → Dispatcher l'ordre via SendMessage immédiatement

(Pas de ScheduleWakeup — la boucle de supervision gère les ping_pending expirés à chaque cycle)
```

### Nommage des Agents — Règle Absolue

Le paramètre `name` dans `Task` est **toujours le nom canonique simple** : `qa`, `dev-backend`, `planner`…  
**Jamais de suffixe** (`qa-1`, `qa-2`…). Un rôle = un nom = une adresse `SendMessage` permanente.

Si le système impose un suffixe → l'agent précédent tourne encore → envoyer PING au nom simple ; au timeout s'il ne répond pas → c'est qu'il est bloqué, forcer via `TaskStop` puis re-spawn.

### Restauration après compactage de contexte

Après un compactage, un hook `UserPromptSubmit` ré-injecte automatiquement `workflow-state.json`. **À réception de ce bloc, re-vérifier tous les agents actifs via PING** :

**Étape 1** — Pour chaque agent listé, écrire `ping_sent_at: <ISO>`, `status: "ping_pending"` et envoyer le PING dans le même bloc :
```
Pour chaque agent présent dans workflow-state.json :
  → workflow-state.json : status = "ping_pending", ping_sent_at = <ISO>
     (conserver pending_order = null si l'agent était idle, ou le dernier ordre connu si working)
  → SendMessage({to: "<agent>", content: "PING"})
```

**Étape 2** — Sur chaque réponse `<NOM> ACTIF` reçue : `status = "idle"`, `ping_sent_at = null`. Agent confirmé vivant.

**Étape 3** — La boucle de supervision (déjà active) traitera les non-répondants à son prochain cycle (≤ 60s) : agents toujours `ping_pending` avec `ping_sent_at` expiré → spawn ou retrait.

### Workflow-state.json — Source de Vérité

Écrire **immédiatement sur disque** à chaque événement (jamais en mémoire) :

| Événement | Mise à jour |
|-----------|-------------|
| Envoi PING | `status: "ping_pending"`, `ping_sent_at: <ISO>`, `pending_order: "<ordre>"` |
| Réception ACTIF (réponse PING) | `status: "idle"`, `ping_sent_at: null`, `pending_order: null` |
| Expiration PING (boucle, ≥ 60s) | spawn via Task + dispatch `pending_order`, `status: "working"`, `ping_sent_at: null` |
| Dispatch (SendMessage de travail) | `status: "working"`, `last_order_sent_at: <ISO>`, `idle_since: null` |
| Réception DONE | `status: "idle"`, `idle_since: <ISO>` |
| Réception `PONG(WORKING\|IDLE)` | `status` correspondant, `last_pong_at: <ISO>` |
| Envoi `shutdown_request` | `status: "pending_delete"` |
| Réception `shutdown_response` | supprimer l'entrée agent |
| `TaskStop` (boucle, non-répondant PING-STATUS) | supprimer l'entrée agent |

Format minimal :
```json
{
  "watchdog_active": false,
  "ping_status_sent_at": null,
  "agents": {
    "<nom>": { "status": "working|idle|ping_pending|pending_delete", "last_order_sent_at": "<ISO>", "idle_since": null, "ping_sent_at": null, "pending_order": null, "last_pong_at": null }
  }
}
```

### Boucle de Supervision — Singleton

- Prérequis : `project-config.json` absent → skip (pas de team)
- Une seule boucle (`watchdog_active` = garde), **60s par cycle**
- Gère en un seul endroit : expirations PING, TTL idle, pending_delete, liveness PING-STATUS

### Activation des Agents (démarrage de workflow)

**Temps 1** — Activer `planner` (PING → ping_pending dans JSON | ACTIF → dispatch immédiat | expiration → boucle spawne)  
**Temps 2** — Après rapport planner, activer en parallèle les agents du scope détecté

Scope → agents dev concernés + `test-writer` + `code-reviewer` + `qa` + `doc-updater` + `deployer`  
Exception HOTFIX : pas de planner, activer directement dev-* + deployer  
Exception SECU : uniquement `security`

**Prompt obligatoire pour tout `Task` de spawn** (première activation ou re-spawn) :
```
"Lis .claude/agents/context/TEAMMATES_PROTOCOL.md puis .claude/agents/<nom>.template.md,
 puis .claude/agents/<nom>.md si ce fichier existe (adaptations projet).
 Tu fais partie de {TEAM_NAME} sur {PROJECT_NAME}.
 Reste en mode IDLE et attends mes ordres."
```
Un agent spawné sans cette ligne ne connaît pas le protocole et répondra en inline.

### Validation des rapports DONE

Un `DONE` valide ne contient **jamais** de contenu inline (code, diff, extraits).  
Format attendu : références fichiers uniquement (`_work/reports/`, `_work/handoff/`, SHA).

Si un agent envoie du contenu inline → refuser et corriger :
```
SendMessage({
  to: "<agent>",
  content: "Rapport invalide — aucun contenu inline autorisé. Écris le contenu dans _work/reports/<agent>-<timestamp>.md et renvoie le DONE avec la référence uniquement."
})
```
Ne jamais accepter un DONE inline comme valide — relancer jusqu'au format correct.

<!-- END TEAMLEADER_PROTOCOL -->

---

## Conventions Git

- **Branches** : `feature/<name>`, `bugfix/<name>`, `hotfix/<name>`
- **Commits** : `type(scope): message` — types : `feat`, `fix`, `docs`, `refactor`, `test`, `chore`
- **Tags** : `vX.Y.Z` — la CI patche et publie la release automatiquement
- **Jamais de push direct sur main** sans validation
