# TEAMMATES_PROTOCOL.md — Protocole Standard des Agents

Ce fichier définit le comportement standard de tous les agents non-CDP de la team.
**Chaque agent doit lire ce fichier au démarrage et appliquer ces règles.**

---

## 1. Démarrage — Exécution immédiate

Au démarrage, chaque agent :

```
1. Lit ce fichier (TEAMMATES_PROTOCOL.md)
2. Lit son propre fichier de spec (.claude/agents/<nom>.md)
3. Envoie l'ACK de démarrage au teamleader
4. Exécute la tâche incluse dans le message de spawn
```

**La tâche est reçue dans le message de spawn. Commencer immédiatement après l'ACK.**

**ACK obligatoire** — envoyer en premier, avant toute autre action :
```
SendMessage({ to: "main", content: "[NOM-AGENT] ACTIF" })
```

---

## 2. Exécution de la tâche

```
Lire et comprendre la tâche
    |
    v
Envoyer ACK : SendMessage(main, "[NOM-AGENT] ACTIF")
    |
    v
Exécuter le travail
  (signaler les jalons au teamleader en cours de route)
    |
    v
Écrire le handoff dans _work/handoff/
    |
    v
Envoyer le rapport DONE au teamleader (SendMessage)
    |
    v
⚠ FERMER CE PANE IMMÉDIATEMENT — ne pas rester en attente
```

**RÈGLE ABSOLUE** : Après avoir envoyé DONE, fermer ce pane.
Ne pas afficher "IDLE". Ne pas attendre d'autres ordres. Terminer.

---

## 3. Communication

### Règles absolues

- **Jamais de communication directe** avec l'utilisateur — tout passe par le teamleader
- **Texte naturel uniquement** — pas de JSON structuré dans les messages
- **Livrables = fichiers** — jamais de contenu inline dans un message

### Push proactif de progression

Envoyer un `SendMessage` au teamleader à chaque jalon :

| Jalon | Quand |
|-------|-------|
| ACTIF | Immédiatement au démarrage — avant toute action |
| EN COURS | À chaque transition d'étape |
| BLOQUE | Dès qu'un blocage survient |
| DONE | Quand la tâche est complètement terminée |

Format de mise à jour de progression (une seule ligne) :

```
[NOM-AGENT] EN COURS — étape N/M — [label < 8 mots] — X%
```

- **N** : numéro de l'étape qui vient de démarrer (commence à 1)
- **M** : nombre total d'étapes (connu dès le démarrage)
- **X%** : `round(N / M × 100)`

Exemples :
```
DEV-BACKEND EN COURS — étape 1/6 — analyse codebase existant — 17%
DEV-BACKEND EN COURS — étape 3/6 — implémentation handler auth — 50%
QA EN COURS — étape 2/7 — exécution tests unitaires — 29%
```

### Livrables — Règle Fondamentale

**Tout livrable est un fichier. Jamais de contenu inline dans un message.**

| Type d'agent | Livrable | Emplacement |
|-------------|----------|-------------|
| dev-*, test-writer | Code commité | Référence par SHA uniquement |
| planner, code-reviewer, qa, security | Rapport d'analyse | `_work/reports/[agent]-[YYYYMMDD-HHmmss].md` |

### Handoff — Transmission de contexte

Avant d'envoyer DONE, écrire le handoff dans `_work/handoff/[agent]-[YYYYMMDD-HHmmss].md` :

```markdown
# Handoff — [Agent]

**Feature** : [description courte]
**SHA** : [commit sha ou N/A]

## Ce qui a été fait
[résumé en 3-5 lignes]

## Décisions clés
[décisions techniques prises, avec justification courte]

## Points d'attention
[ce que l'agent suivant doit savoir : risques, TODO, dépendances]

## Fichiers modifiés
[liste]
```

### Handoff direct entre agents

Le teamleader peut autoriser la transmission directe du handoff à l'agent suivant :
```
SendMessage({
  to: "<agent-suivant>",
  content: "Handoff de [NOM-AGENT] : _work/handoff/[agent]-[timestamp].md"
})
```
Toujours envoyer aussi le DONE au teamleader, même si le handoff a été transmis directement.

### Format du rapport DONE

Agent de code (dev-*, test-writer) :
```
[NOM-AGENT] DONE
Handoff : _work/handoff/[agent]-[YYYYMMDD-HHmmss].md
Fichiers : chemin/fichier1, chemin/fichier2
SHA : <commit-sha>
```

Agent d'analyse (planner, code-reviewer, qa, security) :
```
[NOM-AGENT] DONE
Handoff : _work/handoff/[agent]-[YYYYMMDD-HHmmss].md
Rapport : _work/reports/[agent]-[YYYYMMDD-HHmmss].md
```

En cas d'échec :
```
[NOM-AGENT] FAILED
Raison : [une ligne — cause technique précise]
Action requise : [ce dont j'ai besoin]
```

Format de rapport de blocage :
```
[NOM-AGENT] BLOQUE
Raison : [une ligne]
Action requise : [ce dont j'ai besoin]
```

---

## 4. Réponse au statut (/progression)

Quand le teamleader demande un statut :
```
[NOM-AGENT] | [EN COURS étape N/M X% | BLOQUE] | [une ligne]
```

Exemple :
```
DEV-BACKEND | EN COURS étape 3/6 50% | implémentation handler auth
QA | BLOQUE | impossible de lancer les tests — dépendance manquante
```

---

## 5. Règles Générales

1. **ACK immédiat** — envoyer ACTIF avant toute autre action
2. **Exécution directe** — commencer la tâche sans attendre de confirmation
3. **Un travail à la fois** — terminer avant d'accepter autre chose
4. **Push proactif** — signaler démarrage, jalons, blocages, fin sans être sollicité
5. **Pas d'initiative** — exécuter uniquement la tâche reçue dans le spawn
6. **Pas de communication directe** — l'utilisateur parle via le teamleader
7. **Auto-fermeture** — après DONE, fermer ce pane immédiatement

---

## 6. Exemple de Session Typique

```
[AGENT SPAWNÉ — tâche incluse dans le message de spawn]
→ Lit TEAMMATES_PROTOCOL.md ✓
→ Lit .claude/agents/dev-backend.md ✓
→ SendMessage(main, "DEV-BACKEND ACTIF")

→ SendMessage(main, "DEV-BACKEND EN COURS — étape 1/6 — lecture contrats et codebase — 17%")
→ [étape 1 terminée, étape 2 démarre]
→ SendMessage(main, "DEV-BACKEND EN COURS — étape 2/6 — création modèles et interfaces — 33%")
→ [...]
→ SendMessage(main, "DEV-BACKEND EN COURS — étape 6/6 — commit atomique — 100%")
→ SendMessage(main, "DEV-BACKEND DONE\nHandoff : _work/handoff/dev-backend-20240101-120000.md\nFichiers : internal/auth/handler.go\nSHA : a3f1c2d")
→ [FERME CE PANE]
```
