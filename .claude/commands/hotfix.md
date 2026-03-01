# Commande /hotfix - Correction Urgente en Production

Déclenche le workflow accéléré pour les bugs critiques en production. **Le CDP** (agent cdp) est le team leader qui coordonne tous les agents. Claude est l'interface utilisateur.

## Argument reçu

$ARGUMENTS

## Architecture CDP

```
Claude (interface) → SendMessage(cdp, "HOTFIX URGENT: xxx")
                          │
                     CDP (Team Leader)
                          │
    ┌──────────┬──────────┼──────────┬──────────┐
backend-dev  [test-writer]  [qa]   doc-updater  deployer
                          │
               Coordination via TaskUpdate + SendMessage
```

**IMPORTANT** : Claude délègue au CDP. Le CDP est le chef de projet — Claude est uniquement l'interface utilisateur.

## Workflow HOTFIX (Accéléré)

```
┌─────────────────────────────────────────────────────────────┐
│  ANALYSE → DEV → [TESTS]* → [QA]* → DOC → DEPLOY(PROD)     │
└─────────────────────────────────────────────────────────────┘
* = Optionnel si vraiment critique
```

### Phase 0: ANALYSE (TOI)
- Analyser la criticité du bug (sécurité, bloquant, régression)
- Identifier la portée (backend/frontend/firmware)
- Estimer l'urgence réelle
- Créer la branche `hotfix/<nom-court>`

### Phase 1: TEAM SETUP (TOI)
- TeamCreate({ team_name: "hotfix-xxx", agent_type: "team-lead" })
- Créer les agents MINIMAUX :
  - dev-backend/frontend/buzzclick (selon portée)
  - test-writer (si possible)
  - QA (si possible)
  - doc-updater (toujours)
  - deploy (toujours - PROD direct)

### Phase 2: DÉVELOPPEMENT (Agents DEV)
- Assigner via TaskUpdate(owner: "dev-xxx")
- Correction MINIMALE uniquement
- Commits atomiques avec "hotfix:"

### Phase 3: TESTS (test-writer) [OPTIONNEL]
- Si temps disponible : test de non-régression
- Sinon : skip et noter dans post-mortem

### Phase 4: QA (qa-tester) [OPTIONNEL]
- Si temps disponible : tests de base
- Sinon : skip et noter dans post-mortem

### Phase 5: DOCUMENTATION (doc-updater)
- Incrémenter Z + suffix : 2.40.1 → 2.40.2-hotfix
- CHANGELOG : section Fixed
- Post-mortem OBLIGATOIRE

### Phase 6: DÉPLOIEMENT PROD (deploy)
- **QUALIF OPTIONNEL** si vraiment critique
- Build et déploiement PROD direct
- Monitoring actif après déploiement

## Spécificités HOTFIX

| Règle | Contrainte |
|-------|-----------|
| **Versioning** | Incrémente Z + suffix : 2.40.1 → 2.40.2-hotfix |
| **Branche** | `hotfix/<nom-court>` |
| **Scope** | MINIMAL absolu - corriger UNIQUEMENT le bug critique |
| **QUALIF** | OPTIONNEL si vraiment critique |
| **Tests** | Optionnel si urgence vraie (noter dans post-mortem) |
| **Post-mortem** | OBLIGATOIRE (cause, impact, prévention) |
| **Commits** | Atomiques avec "hotfix:" |

## Quand utiliser /hotfix

| Situation | Action |
|-----------|--------|
| Bug bloquant en production | ✅ /hotfix |
| Problème de sécurité urgent | ✅ /hotfix |
| Régression critique après release | ✅ /hotfix |
| Bug gênant mais non bloquant | ❌ Utiliser /bugfix |
| Amélioration urgente | ❌ Utiliser /feature |

## Post-mortem OBLIGATOIRE

À la fin du hotfix, créer `docs/postmortem/hotfix-vX.Y.Z.md` :

```markdown
# Post-mortem Hotfix vX.Y.Z

## Résumé
[Description du bug critique]

## Chronologie
- HH:MM - Bug détecté
- HH:MM - Hotfix démarré
- HH:MM - Correction déployée

## Cause racine
[Pourquoi ce bug est apparu]

## Impact
- Utilisateurs affectés : [nombre/tous]
- Durée : [minutes]
- Données perdues : [oui/non]

## Correction
[Description de la correction appliquée]

## Tests skippés
- [ ] Tests unitaires (raison: urgence)
- [ ] QA E2E (raison: urgence)
- [ ] QUALIF (raison: urgence)

## Prévention future
[Actions pour éviter ce type de bug]

## Actions post-déploiement
- [ ] Surveiller logs pendant 1h
- [ ] Ajouter tests de non-régression
- [ ] Mettre à jour documentation
```

## Règles Critiques

### ✅ Claude DOIT
- Vérifier que c'est vraiment CRITIQUE avant de déléguer
- Transmettre au CDP avec le contexte d'urgence
- Relayer fidèlement les messages CDP ↔ utilisateur

### ❌ Claude NE DOIT PAS
- Coordonner les agents directement
- Écrire du code lui-même

### ✅ Le CDP DOIT (référence)
- Utiliser les agents MINIMAUX nécessaires
- Documenter ce qui a été skippé
- Créer le post-mortem OBLIGATOIRE
- Surveiller le déploiement

### ❌ Le CDP NE DOIT PAS
- Écrire du code lui-même
- Skip le post-mortem
- Autoriser du refactoring

## Agents que TU coordonnes

| Agent | Subagent Type | Usage HOTFIX |
|-------|---------------|--------------|
| Backend Dev | `dev-backend` | Si bug serveur Go |
| Frontend Dev | `dev-frontend` | Si bug React/CSS |
| Firmware Dev | `dev-buzzclick` | Si bug ESP32 |
| Test Writer | `test-writer` | Optionnel |
| QA Tester | `QA` | Optionnel |
| Doc Updater | `doc-updater` | Toujours (post-mortem) |
| Deployer | `deploy` | Toujours (PROD direct) |

## Action immédiate

**TU** (Claude) dois maintenant :

1. **Analyser** la criticité : `$ARGUMENTS`
2. **Vérifier** que c'est vraiment un HOTFIX (sinon → /bugfix)
3. **Transmettre** au CDP avec contexte d'urgence :
   ```
   SendMessage(recipient: "cdp", content: "HOTFIX URGENT: [description + criticité + portée]. Workflow accéléré requis.", summary: "Hotfix urgent: [titre court]")
   ```
4. **Relayer** tous les messages et validations du CDP vers l'utilisateur

**Le CDP prend en charge l'intégralité du workflow depuis cette étape.**


### Sans TEAM (pas de myTEAM actif)
```
subagent_type: "cdp"
description: "Workflow hotfix urgent"
prompt: HOTFIX URGENT: $ARGUMENTS. Workflow accéléré requis.
```
