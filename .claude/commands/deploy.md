# Commande /deploy - Arrêt et Redéploiement QUALIF / PROD

Arrête le serveur en cours puis redéploie vers l'environnement cible.

## Argument reçu

$ARGUMENTS

**Format** : `/deploy [QUALIF|PREPROD|PROD|hotfix]`

- **QUALIF** (défaut) : Build Windows, redémarrage local pour tests
- **PREPROD** : Build Windows + ARM64, redémarrage local pour validation finale
- **PROD** : Build Windows + ARM64 + squash merge + tag + release GitHub
- **hotfix** : Mode urgence pour bugs critiques

## Instructions

Utilise le Task tool pour lancer le sous-agent deploy avec les paramètres suivants :

```
subagent_type: "deploy"
description: "Redéploiement [ENV]"
prompt: voir ci-dessous
```

### Prompt à transmettre au sous-agent

```
Arrête et redéploie le serveur BuzzControl vers l'environnement cible.

**Contexte projet :** Voir `.claude/context/COMMON.md` section 1
**Contrôle serveur :** Voir `.claude/context/COMMON.md` section 3
**Build :** Voir `.claude/context/COMMON.md` section 2
**Checklists :** Voir `.claude/context/COMMON.md` section 7

**Procédures détaillées :**
- QUALIF : docs/QUALIF_PROCEDURE.md
- PROD : docs/RELEASE_PROCEDURE.md

**Environnement cible :** $ARGUMENTS (QUALIF par défaut si vide)

**Étapes à exécuter :**

## PHASE 1 : PRÉPARATION

1. **Arrêter le serveur en cours**
   Voir `.claude/context/COMMON.md` section 3.1 (Arrêt Gracieux)

2. **Collecter les informations**
   - Version : lire server-go/config.json → champ "version"
   - Branche : git branch --show-current
   - Dernier commit : git log -1 --oneline

3. **Vérifier les prérequis**
   - Tests QA passés (PREPROD/PROD)
   - Review approuvée (PREPROD/PROD)
   - Version incrémentée

## PHASE 2 : DOCUMENTATION ET TÂCHES (PROD uniquement)

**Cette phase se fait AVANT le build pour inclure la doc dans le commit final.**

4. **Finaliser la version (PROD uniquement)**
   - Lire server-go/config.json → version actuelle (ex: 2.45.3)
   - Mettre z=0 pour la release (ex: 2.45.0)
   - Mettre à jour server-go/web/package.json → même version

5. **Mettre à jour CHANGELOG.md (PROD uniquement)**
   - Ajouter section ## [X.Y.0] - YYYY-MM-DD
   - Lister les nouveautés (Ajouté/Modifié/Corrigé)

6. **Mettre à jour CLAUDE.md si nécessaire (PROD uniquement)**
   - Nouvelles fonctionnalités
   - Nouveaux endpoints/actions WebSocket
   - Décisions d'architecture

7. **Marquer les tâches comme complétées (PROD uniquement)**
   - Utiliser TaskList pour voir les tâches en cours
   - Utiliser TaskUpdate(status: "completed") pour chaque tâche terminée

8. **Commit de la documentation (PROD uniquement)**
   git add CHANGELOG.md CLAUDE.md server-go/config.json server-go/web/package.json
   git commit -m "docs: Release vX.Y.0 - [description courte]"

## PHASE 3 : BUILD ET TEST LOCAL

9. **Build Frontend + Backend**
   Voir `.claude/context/COMMON.md` section 2.1 (Build Complet)

   ⚠️ RÈGLE CRITIQUE : Frontend AVANT Backend (mode portable)

10. **Build Go selon environnement**
   Voir `.claude/context/COMMON.md` section 2.2 (Build Cross-Platform)

   - QUALIF : Windows uniquement
   - PREPROD/PROD : Windows + ARM64

11. **Redémarrer le serveur Windows**
    - Lancer server.exe en arrière-plan
    - Attendre 2-3 secondes pour le démarrage complet
    - Le serveur reste actif après les tests

12. **Vérification de la version (avec retry automatique)**
    Voir `.claude/context/COMMON.md` section 3.3 (Vérification Post-Démarrage)
    et section 3.4 (Gestion Version Mismatch)

    - Maximum 2 tentatives de retry
    - Si échec après 2 retries → **ERREUR CRITIQUE** : Arrêter et escalader

## PHASE 4 : GIT ET CI (PROD uniquement)

13. **Push et merge (PROD uniquement)**
    - Push la branche feature (avec le commit de doc)
    - Checkout main et pull
    - Squash merge vers main
    - Push main
    - Tag annotée v<version>
    - Push tag
    - **NE PAS supprimer la branche** (conservée pour rollback CI)

14. **Attendre et vérifier la CI automatiquement (PROD uniquement)**
    La CI GitHub Actions se déclenche automatiquement au push du tag.

    **IMPORTANT : Deux étapes distinctes pour détecter correctement la CI**

    a) **Étape 1 - Attendre que le workflow APPARAISSE** (filtrer par commit SHA) :
       ```bash
       # Récupérer le SHA du commit actuel (celui qui est taggé)
       COMMIT_SHA=$(git rev-parse HEAD)

       # Attendre que le workflow apparaisse (max 2 minutes, intervalle 10s)
       echo "⏳ Waiting for CI workflow to start for commit $COMMIT_SHA..."
       RUN_ID=""
       for i in $(seq 1 12); do
           # Filtrer les runs par head_sha pour trouver NOTRE run spécifique
           # Note: tr -d '\n' nécessaire car curl retourne du JSON formaté multilignes
           RESPONSE=$(curl -s "https://api.github.com/repos/CCoupel/BuzzMaster/actions/runs?head_sha=$COMMIT_SHA&per_page=1" | tr -d '\n')
           TOTAL_COUNT=$(echo $RESPONSE | grep -oP '"total_count":\K[0-9]+')

           if [ "$TOTAL_COUNT" != "0" ] && [ -n "$TOTAL_COUNT" ]; then
               RUN_ID=$(echo $RESPONSE | grep -oP '"id":\K[0-9]+' | head -1)
               echo "✅ CI workflow started (Run ID: $RUN_ID)"
               break
           fi

           echo "   Attempt $i/12 - Workflow not yet started, waiting 10s..."
           sleep 10
       done

       # Si pas de workflow après 2 min → afficher erreur
       if [ -z "$RUN_ID" ]; then
           echo "❌ ERROR: CI workflow did not start after 2 minutes"
           echo "   Manual check: https://github.com/CCoupel/BuzzMaster/actions"
       fi
       ```

    b) **Étape 2 - Attendre que le workflow SE TERMINE** :
       ```bash
       # Poll le status du workflow spécifique par son ID (max 10 minutes, intervalle 30s)
       echo "⏳ Waiting for CI workflow $RUN_ID to complete..."
       for i in $(seq 1 20); do
           # Note: tr -d '\n' nécessaire pour parser le JSON correctement
           RESPONSE=$(curl -s "https://api.github.com/repos/CCoupel/BuzzMaster/actions/runs/$RUN_ID" | tr -d '\n')
           STATUS=$(echo $RESPONSE | grep -oP '"status":\s*"\K[^"]+')
           # Note: conclusion peut être vide/null quand le workflow n'est pas terminé
           CONCLUSION=$(echo $RESPONSE | grep -oP '"conclusion":\s*"\K[^"]+' | head -1)

           echo "   Attempt $i/20 - Status: $STATUS, Conclusion: $CONCLUSION"

           if [ "$STATUS" = "completed" ]; then
               break
           fi
           sleep 30
       done
       ```

    c) **Analyser le résultat** :
       - Si `status` = "completed" ET `conclusion` = "success" → Continuer vers Phase 5
       - Si `status` = "completed" ET `conclusion` != "success" → ROLLBACK et correction
       - Si timeout (10 min + 2 min) → Afficher erreur et demander à l'utilisateur

    d) **Si CI réussie** :
       Afficher : "✅ CI GitHub Actions terminée avec succès"
       Continuer vers Phase 5

    e) **Si CI échouée** (ANALYSE ET CORRECTION automatique) :
       1. Afficher : "❌ CI échouée - Analyse de l'erreur en cours..."

       2. **Récupérer les logs d'erreur via GitHub API** (utiliser le RUN_ID déjà obtenu) :
          ```bash
          # Récupérer les jobs du workflow avec l'ID qu'on a déjà
          JOBS=$(curl -s "https://api.github.com/repos/CCoupel/BuzzMaster/actions/runs/$RUN_ID/jobs")

          # Identifier le job qui a échoué et récupérer ses logs
          ```

       3. **Analyser le type d'erreur** :
          - Erreur de build Go → Problème backend (syntaxe, import, compilation)
          - Erreur de build npm → Problème frontend (syntaxe JS/CSS, dépendances)
          - Erreur de test → Test unitaire ou E2E échoué
          - Erreur de lint → Problème de formatage ou règles

       4. **Revert temporaire et retour sur branche feature** :
          git checkout main && git revert HEAD --no-edit && git push origin main
          git tag -d v<version> && git push origin --delete v<version>
          git checkout <feature-branch>

       5. **Lancer l'agent de correction approprié** :
          - Si erreur backend Go → Lancer agent `dev-backend` avec le message d'erreur
          - Si erreur frontend React → Lancer agent `dev-frontend` avec le message d'erreur
          - Si erreur mixte → Lancer les deux agents séquentiellement

          **Prompt pour l'agent de correction** :
          ```
          La CI a échoué avec l'erreur suivante :
          [COLLER LE MESSAGE D'ERREUR]

          Analyse cette erreur et corrige-la. Après correction :
          1. Rebuild et vérifie localement
          2. Commit avec message "fix: [description de la correction]"
          ```

       6. **Après correction, relancer le déploiement** :
          - Incrémenter z de la version (ex: 2.47.0 → 2.47.1)
          - Relancer `/deploy PROD` automatiquement

       7. **Si 3 tentatives échouent** → ARRÊTER et escalader à l'utilisateur

## PHASE 5 : VALIDATION RELEASE GITHUB (PROD uniquement)

15. **Arrêter le serveur local (PROD uniquement)**
    Après validation CI, arrêter le serveur buildé localement :
    curl -s http://localhost/shutdown
    # Attendre 2 secondes

16. **Télécharger l'exécutable GitHub Release (PROD uniquement)**
    Télécharger le binaire Windows depuis la release GitHub :

    # PowerShell (Windows)
    $version = "X.Y.0"
    $url = "https://github.com/CCoupel/BuzzMaster/releases/download/v$version/buzzcontrol-v$version-windows-amd64.exe"
    Invoke-WebRequest -Uri $url -OutFile "server-go/server.exe"

    # Ou avec curl
    curl -L -o server-go/server.exe "https://github.com/CCoupel/BuzzMaster/releases/download/vX.Y.0/buzzcontrol-vX.Y.0-windows-amd64.exe"

17. **Lancer le serveur dans une fenêtre visible (PROD uniquement)**
    L'utilisateur doit voir les logs dans la console :

    # PowerShell (depuis la racine du projet)
    Start-Process -FilePath "server.exe" -WorkingDirectory "server-go"

    # Ou CMD (depuis la racine du projet)
    start cmd /k "cd server-go && server.exe"

    Attendre 3-5 secondes pour le démarrage

18. **Validation finale de la release (PROD uniquement)**
    a) **Vérifier la version** :
       curl http://localhost/version
       # DOIT retourner exactement la version de la release (X.Y.0)

    b) **Vérifier le fonctionnement** :
       curl http://localhost/listGame
       # DOIT retourner un JSON valide

    c) **Informer l'utilisateur** :
       "✅ Release v<version> validée. Le serveur tourne avec l'exécutable GitHub Release.
        Les logs sont visibles dans la fenêtre de console."

## PHASE 6 : RAPPORT

19. **Générer le rapport de redéploiement** avec :
    - Informations : Version, env, date, branche, commit
    - Arrêt : Résultat de l'arrêt du serveur précédent
    - Documentation : Fichiers mis à jour (CHANGELOG, CLAUDE.md, config.json)
    - Tâches : Nombre de tâches marquées complétées
    - Builds : Résultats + tailles binaires (Windows + ARM64 si applicable)
    - Redémarrage : Résultats des tests post-build
    - Vérification version : Version attendue vs version serveur (DOIT MATCHER)
    - Retries : Nombre de tentatives si version mismatch (0 = succès direct)
    - Git (PROD) : Push, merge, tag
    - CI GitHub : Statut validé par l'utilisateur
    - Release GitHub : URL de la release, binaires téléchargés
    - Exécutable final : Source (GitHub Release), version validée
    - Décision : SUCCESS / FAILED

**Différences par environnement :**
| Action | QUALIF | PREPROD | PROD |
|--------|--------|---------|------|
| Arrêt serveur | Oui | Oui | Oui |
| Finalisation doc (CHANGELOG, CLAUDE.md) | Non | Non | Oui |
| Version z=0 + sync package.json | Non | Non | Oui |
| TaskUpdate(completed) | Non | Non | Oui |
| Commit documentation | Non | Non | Oui |
| **Build Frontend React (npm run build)** | **Oui** | **Oui** | **Oui** |
| Build Windows | Oui | Oui | Oui |
| Build ARM64 | Non | Oui | Oui |
| Redémarrage Windows (build local) | Oui | Oui | Oui |
| Push branche feature | Non | Non | Oui |
| Squash merge main | Non | Non | Oui |
| Tag Git | Non | Non | Oui |
| Vérifier CI automatiquement (API GitHub) | Non | Non | Oui |
| Télécharger exe GitHub Release | Non | Non | Oui |
| Lancer exe release (fenêtre visible) | Non | Non | Oui |
| Validation version release | Non | Non | Oui |
| Conservation branche | - | - | Oui (rollback) |

**Workflow recommandé :**
```
QUALIF → Tests rapides (Windows uniquement)
   ↓
PREPROD → Validation finale (Windows + ARM64 prêt)
   ↓
PROD → Release (merge + tag + binaires prêts pour Raspberry Pi)
```

**Règles critiques :**
Voir `.claude/context/COMMON.md` sections 2-7 pour les règles détaillées.

Résumé :
- Build : Frontend AVANT Backend (mode portable) - COMMON.md 2.1
- Serveur : Arrêt gracieux via API - COMMON.md 3.1
- Version : Vérification + retry automatique - COMMON.md 3.3-3.4
- PROD : z=0, sync package.json, doc AVANT build - COMMON.md 5.4, 7.3
- Git : Jamais supprimer branche feature (rollback) - COMMON.md 6.4
```

## Action immédiate

Lance maintenant le sous-agent deploy avec le Task tool.
