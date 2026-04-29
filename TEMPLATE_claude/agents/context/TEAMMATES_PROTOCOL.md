# TEAMMATES_PROTOCOL.md — Protocole Standard des Agents

Ce fichier definit le comportement standard de tous les agents non-CDP de la team.
**Chaque agent doit lire ce fichier au demarrage et appliquer ces regles pour toute la session.**

---

## 1. Demarrage — Mode IDLE

Au demarrage, chaque agent :

```
1. Lit ce fichier (TEAMMATES_PROTOCOL.md)
2. Lit son propre fichier de spec (.claude/agents/<nom>.md)
3. Attend les instructions du CDP
```

**REGLE ABSOLUE** : Rester en mode IDLE jusqu'a recevoir un ordre explicite du CDP.
Ne pas verifier la TaskList. Ne pas prendre d'initiative. Attendre.

**REGLE DE CONFIRMATION** : Si tu souhaites confirmer ton demarrage, envoie
`SendMessage({to: "cdp", content: "[NOM-AGENT] DEMARRE — en attente d'ordres"})` — jamais a Claude directement.

---

## 2. Reception d'un Ordre

Le CDP active un agent en lui envoyant un message via `SendMessage`.
Quand un agent recoit un message du CDP :

```
Recevoir l'ordre du CDP
    |
    v
Lire et comprendre la tache
    |
    v
Signaler le DEMARRAGE au CDP
    |
    v
Executer le travail
  (signaler les jalons au CDP en cours de route)
    |
    v
Envoyer le rapport TERMINE au CDP (SendMessage)
    |
    v
Retourner en mode IDLE
```

---

## 3. Communication

### Regles absolues

- **Jamais d'initiative** — attendre l'ordre du CDP
- **Jamais de communication directe** avec l'utilisateur — tout passe par le CDP
- **Jamais de contact entre agents** — chaque agent ne parle qu'au CDP
- **Texte naturel uniquement** — pas de JSON structure dans les messages

### Livrables — Regle Fondamentale

**Tout livrable est un fichier. Jamais de contenu inline dans un message.**

Avant d'envoyer le rapport DONE, ecrire le livrable dans le bon emplacement :

| Type d'agent | Livrable | Emplacement |
|-------------|----------|-------------|
| dev-*, test-writer | Code commite | Reference par SHA uniquement |
| planner, code-reviewer, qa, security | Rapport d'analyse | `.claude/reports/[agent]-[YYYYMMDD-HHmmss].md` |

Le message au CDP ne contient que la reference, jamais le contenu :
```
Rapport : .claude/reports/[filename]
```

### Handoff — Transmission de Contexte

Avant d'envoyer le DONE, chaque agent écrit son handoff dans `.claude/handoff/[agent]-[YYYYMMDD-HHmmss].md` :

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

Le CDP passe la référence du handoff dans le SendMessage au prochain agent :
```
Handoff [agent précédent] : .claude/handoff/[agent]-[timestamp].md
```

**Règle** : un agent qui reçoit une référence handoff doit la lire avant de commencer son travail.

### Envoyer un rapport au CDP

```
SendMessage({
  to: "cdp",
  content: "[rapport minimaliste — voir formats ci-dessous]"
})
```

> **REGLE ABSOLUE** : Jamais de contenu de code, de diff, ni d'extraits de fichiers dans les messages.
> Les messages sont des references a des fichiers, pas des rapports techniques.

### Push proactif de progression

**REGLE** : Ne pas attendre d'etre sollicite. Envoyer un `SendMessage` au CDP a chaque jalon :

| Jalon | Quand |
|-------|-------|
| DEMARRE | Des le debut de l'execution de la tache |
| BLOQUE | Des qu'un blocage survient |
| TERMINE | Quand la tache est completement terminee |

Format de mise a jour de progression (une seule ligne) :

```
[NOM-AGENT] EN COURS — X% — [etape courante en < 10 mots]
```

### Reponse a une demande de progression (/progression)

Quand le CDP demande un statut de progression, repondre avec ce format exact :

```
[NOM-AGENT] | [TERMINE | EN COURS X% | ATTENTE | BLOQUE] | [une ligne]
```

### Format du rapport de fin de tache

Agent de code (dev-*, test-writer) :
```
[NOM-AGENT] DONE
Handoff : .claude/handoff/[agent]-[YYYYMMDD-HHmmss].md
Fichiers : chemin/fichier1, chemin/fichier2
SHA : <commit-sha>
```

Agent d'analyse (planner, code-reviewer, qa, security) :
```
[NOM-AGENT] DONE
Handoff : .claude/handoff/[agent]-[YYYYMMDD-HHmmss].md
Rapport : .claude/reports/[agent]-[YYYYMMDD-HHmmss].md
```

En cas d'echec :
```
[NOM-AGENT] FAILED
Raison : [une ligne — cause technique precise]
Action requise : [ce dont j'ai besoin]
```

### Format de rapport de blocage

```
[NOM-AGENT] BLOQUE
Raison : [une ligne]
Action requise : [ce dont j'ai besoin]
```

---

## 4. Reponse au Shutdown

Quand le CDP envoie un `shutdown_request` :

```
SendMessage({
  to: "cdp",
  content: "shutdown_response approve: true"
})
```

---

## 5. Regles Generales

1. **IDLE par defaut** — l'etat de repos est l'attente, pas le polling
2. **Un travail a la fois** — terminer une tache avant d'en accepter une autre
3. **Rapport systematique** — toujours envoyer un rapport au CDP apres chaque tache
4. **Push proactif** — signaler demarrage, jalons importants, blocages sans attendre d'etre sollicite
5. **Pas d'initiative** — ne jamais commencer un travail sans ordre du CDP
6. **Pas de communication directe** — l'utilisateur parle via le CDP, pas directement
7. **Texte naturel** — les messages sont lisibles, pas en JSON

---

## 6. Exemple de Session Typique

```
[AGENT DEMARRE]
→ Lit TEAMMATES_PROTOCOL.md ✓
→ Lit .claude/agents/[nom].md ✓
→ MODE IDLE — en attente d'un ordre du CDP

[CDP envoie un ordre via SendMessage]
→ "Implemente l'endpoint POST /api/auth avec JWT. Voir contracts/http-endpoints.md."
→ SendMessage(cdp, "DEV-BACKEND EN COURS — 0% — demarrage implementation /api/auth")
→ [Travail effectue...]
→ SendMessage(cdp, "DEV-BACKEND DONE\nFichiers : internal/auth/handler.go, internal/auth/handler_test.go\nSHA : a3f1c2d")
→ MODE IDLE — en attente du prochain ordre

[CDP envoie shutdown_request]
→ SendMessage(cdp, "shutdown_response approve: true")
```
