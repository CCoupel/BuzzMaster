# Commande /review - Workflow de Revue de Code Périodique

Orchestre un workflow autonome de revue de code pour améliorer la qualité, la sécurité et la maintenabilité du codebase.

## Argument reçu (optionnel)

$ARGUMENTS

**Formats possibles** :
- `/review` : Revue complète du codebase
- `/review security` : Focus sur la sécurité
- `/review performance` : Focus sur les performances
- `/review rationalization` : Focus sur la rationalisation/refactoring

## Workflow complet

```
[Vérifications] → [Git] → PLAN → [Validation] → DEV → QA → [Validation] → DOC → DEPLOY(QUALIF) → [FIN]
```

## Instructions

### Phase 0 : Vérifications préalables (OBLIGATOIRE)

**Avant toute action, vérifier que le codebase est propre :**

1. **Vérifier qu'aucune feature/bugfix n'est en cours**
   ```bash
   git branch --list "feature/*" "bugfix/*" "hotfix/*"
   ```
   - Si des branches existent → **STOP** : Informer l'utilisateur et attendre qu'elles soient mergées ou supprimées

2. **Vérifier que toutes les branches sont mergées sur main**
   ```bash
   git checkout main
   git pull origin main
   git branch --no-merged main
   ```
   - Si des branches non-mergées existent → **STOP** : Lister les branches et demander action

3. **Vérifier l'état du working directory**
   ```bash
   git status
   ```
   - Si des fichiers non-commités → **STOP** : Demander commit ou stash

**⏸️ POINT DE VALIDATION : Toutes les vérifications doivent passer avant de continuer**

### Phase 1 : Préparation Git

Créer la branche de review :

```bash
git checkout main
git pull origin main
git checkout -b review/code-review-YYYY-MM-DD
```

Incrémenter la version mineure dans `server-go/config.json` : `2.43.0` → `2.44.0`

```bash
git add server-go/config.json
git commit -m "chore(version): Start v2.44.0 - Code review"
git push -u origin review/code-review-YYYY-MM-DD
```

### Phase 2 : Analyse et Plan de Review (PLAN via DEV)

Lance le sous-agent **dev-feature-implementation** pour analyser le code et produire un plan de review :

```
subagent_type: "dev-feature-implementation"
description: "Analyser code et créer plan de review"
prompt: "Analyse le codebase BuzzControl et produis un plan de review structuré.

**Contexte projet :**
- Répertoire : C:\Users\cyril\Documents\VScode\buzzcontrol
- Serveur Go : server-go/
- Frontend React : server-go/web/src/
- Architecture : CLAUDE.md

**Focus demandé :** $ARGUMENTS (ou revue complète si vide)

**Catégories d'analyse :**

1. **Sécurité (OWASP)**
   - Injection (SQL, command, XSS)
   - Authentication/Authorization
   - Secrets hardcodés
   - Validation des entrées
   - CORS et headers

2. **Fiabilité**
   - Gestion des erreurs
   - Null/nil checks
   - Race conditions
   - Timeout et retry
   - Graceful shutdown

3. **Performance**
   - Boucles inefficaces
   - Copies mémoire inutiles
   - N+1 queries
   - Re-renders React
   - Bundle size

4. **Rationalisation**
   - Code dupliqué (>70% similaire)
   - Patterns répétés (3+ occurrences)
   - Fonctions trop longues (>50 lignes)
   - Abstractions manquantes
   - Dead code

5. **Maintenabilité**
   - Naming incohérent
   - Comments obsolètes
   - Tests manquants
   - Documentation code

**Output attendu :**

Pour chaque catégorie, lister les optimisations trouvées avec :
- Fichier(s) concerné(s)
- Description du problème
- Solution proposée
- Impact estimé (High/Medium/Low)
- Effort estimé (Small/Medium/Large)

Grouper les optimisations par catégorie et les présenter pour validation.

**IMPORTANT :** Ne pas implémenter. Produire uniquement le plan d'analyse."
```

**⏸️ POINT DE VALIDATION : Présenter les optimisations par groupe/catégorie**

L'utilisateur valide les optimisations qu'il souhaite implémenter :
- ✅ Approuvé → Sera implémenté
- ❌ Rejeté → Ignoré
- 🔄 Reporter → Pour une prochaine review

### Phase 3 : Implémentation des optimisations (DEV)

Après validation, lance le sous-agent **dev-feature-implementation** pour implémenter :

```
subagent_type: "dev-feature-implementation"
description: "Implémenter optimisations validées"
prompt: "Implémente les optimisations de code validées pour BuzzControl.

**Contexte projet :**
- Répertoire : C:\Users\cyril\Documents\VScode\buzzcontrol
- Serveur Go : server-go/
- Frontend React : server-go/web/src/

**Optimisations validées :** [Liste des optimisations approuvées]

**Actions :**
1. Incrémenter z dans config.json à chaque cycle
2. Implémenter chaque optimisation avec commit atomique
3. Format commit : `refactor(<scope>): <description>`
4. Pour sécurité : `security(<scope>): <description>`
5. Pour perf : `perf(<scope>): <description>`
6. Tests unitaires si applicable
7. Push en fin de cycle"
```

### Phase 4 : Tests QA avec validation Chrome

Lance le sous-agent **QA** via Task tool :

```
subagent_type: "QA"
description: "Tests QA avec validation Chrome"
prompt: "Exécute la procédure de tests QA complète pour BuzzControl avec validation via Chrome.

**Contexte projet :**
- Répertoire : C:\Users\cyril\Documents\VScode\buzzcontrol
- Serveur Go : server-go/

**Actions :**
1. Build de production
2. Tests unitaires Go
3. Démarrer le serveur
4. Tests E2E via Chrome (MCP claude-in-chrome)
   - Tester pages admin
   - Tester affichage TV
   - Tester WebSocket logs
5. Produire rapport QA

**IMPORTANT :** Utiliser les outils MCP claude-in-chrome pour les tests navigateur."
```

**⏸️ POINT DE VALIDATION : Attendre tests manuels et validation utilisateur**

L'utilisateur effectue ses propres tests et confirme :
- ✅ Tests OK → Continuer vers DOC
- ❌ Problèmes → Retour à Phase 3 (DEV) avec feedback

### Phase 5 : Documentation (DOC)

Lance le sous-agent **doc-updater** via Task tool :

```
subagent_type: "doc-updater"
description: "Mettre à jour documentation"
prompt: "Mets à jour la documentation pour BuzzControl après la review de code.

**Contexte projet :**
- Répertoire : C:\Users\cyril\Documents\VScode\buzzcontrol
- Config version : server-go/config.json

**Type :** refactor (code review)

**Actions :**
1. CHANGELOG.md : Section 'Refactored' ou 'Security' selon les changements
2. CLAUDE.md : Mettre à jour si architecture impactée
3. Finaliser version (reset z à 0)
4. Commit et push"
```

### Phase 6 : Déploiement QUALIF (DEPLOY)

Lance le sous-agent **deploy** via Task tool :

```
subagent_type: "deploy"
description: "Déploiement QUALIF"
prompt: "Déploie le serveur BuzzControl vers l'environnement QUALIF.

**Contexte projet :**
- Répertoire : C:\Users\cyril\Documents\VScode\buzzcontrol
- Serveur Go : server-go/

**Environnement :** QUALIF

**Actions :**
1. Build Windows + ARM64
2. Tests post-build
3. Créer archive QUALIF"
```

## Fin du workflow /review

**✅ Le workflow /review s'arrête ici.**

Pour continuer vers la production :
1. **Valider la QUALIF** manuellement
2. **Lancer** `/deploy PROD` pour le déploiement en production

## Gestion des erreurs

| Situation | Action |
|-----------|--------|
| Branches non-mergées | STOP : Lister et demander action |
| Fichiers non-commités | STOP : Demander commit ou stash |
| QA échoue | Retour à Phase 3 (DEV) avec erreurs |
| Build échoue | Retour à Phase 3 (DEV) pour correction |
| Maximum 3 cycles DEV ↔ QA | Escalade vers utilisateur |

## Points de validation obligatoires

1. **Phase 0** : Toutes les vérifications Git doivent passer
2. **Après PLAN** : L'utilisateur valide les optimisations à implémenter
3. **Après QA** : L'utilisateur valide ses tests manuels

## Action immédiate

Lance maintenant la **Phase 0 (Vérifications préalables)** :

1. Vérifier qu'aucune branche feature/bugfix/hotfix n'existe
2. Vérifier que toutes les branches sont mergées sur main
3. Vérifier que le working directory est propre
4. Si tout est OK → Passer à Phase 1 (Git)
