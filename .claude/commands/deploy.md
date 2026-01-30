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

**Contexte projet :**
- Répertoire : C:\Users\cyril\Documents\VScode\buzzcontrol
- Serveur Go : server-go/
- Config version : server-go/config.json
- Procédure QUALIF : docs/QUALIF_PROCEDURE.md
- Procédure PROD : docs/RELEASE_PROCEDURE.md
- GitHub repo : https://github.com/CCoupel/BuzzMaster

**Environnement cible :** $ARGUMENTS (QUALIF par défaut si vide)

**Étapes à exécuter :**

## PHASE 1 : PRÉPARATION

1. **Arrêter le serveur en cours**
   curl -s http://localhost/shutdown || echo "Serveur non actif"
   # Attendre 2 secondes pour l'arrêt complet

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

9. **Build selon environnement**
   cd server-go

   # QUALIF : Windows uniquement
   go build -o server.exe ./cmd/server

   # PREPROD/PROD : Windows + ARM64
   go build -o server.exe ./cmd/server
   GOOS=linux GOARCH=arm64 go build -o buzzcontrol ./cmd/server

10. **Redémarrer le serveur Windows**
    - Lancer server.exe en arrière-plan
    - Attendre 2-3 secondes pour le démarrage complet
    - Le serveur reste actif après les tests

11. **Vérification de la version (avec retry automatique)**
    - Lire la version attendue depuis server-go/config.json
    - Appeler curl http://localhost/version
    - **COMPARER** : La version retournée DOIT correspondre à celle de config.json
    - **Si DIFFÉRENCE** :
      1. Afficher WARNING : "Version mismatch: expected X.Y.Z, got A.B.C"
      2. Arrêter le serveur : curl http://localhost/shutdown
      3. Attendre 2 secondes
      4. **RELANCER** les étapes 9, 10, 11 (rebuild + restart + verify)
      5. Maximum 2 tentatives de retry
      6. Si échec après 2 retries → **ERREUR CRITIQUE** : Arrêter et escalader
    - **Si IDENTIQUE** : Vérifier /listGame fonctionne → Continuer

## PHASE 4 : GIT ET CI (PROD uniquement)

12. **Push et merge (PROD uniquement)**
    - Push la branche feature (avec le commit de doc)
    - Checkout main et pull
    - Squash merge vers main
    - Push main
    - Tag annotée v<version>
    - Push tag
    - **NE PAS supprimer la branche** (conservée pour rollback CI)

13. **Attendre et vérifier la CI (PROD uniquement)**
    La CI GitHub Actions se déclenche automatiquement au push du tag.

    a) **Demander à l'utilisateur de vérifier** :
       Afficher : "⏳ La CI GitHub Actions est en cours.
       Veuillez vérifier sur : https://github.com/CCoupel/BuzzMaster/actions

       Attendez que tous les jobs soient ✅ (verts) puis confirmez.
       Si un job est ❌ (rouge), informez-moi pour lancer le rollback."

    b) **Utiliser AskUserQuestion** pour demander :
       - Option 1 : "✅ CI réussie - Continuer"
       - Option 2 : "❌ CI échouée - Rollback nécessaire"

    c) **Si CI échouée** (ROLLBACK) :
       1. Revert le merge sur main :
          git checkout main && git revert HEAD --no-edit && git push origin main
       2. Supprimer le tag :
          git tag -d v<version> && git push origin --delete v<version>
       3. Informer l'utilisateur de vérifier les logs CI sur GitHub
       4. Retourner sur la branche feature pour corriger
       5. **ARRÊTER LE WORKFLOW** et informer l'utilisateur

## PHASE 5 : VALIDATION RELEASE GITHUB (PROD uniquement)

14. **Arrêter le serveur local (PROD uniquement)**
    Après validation CI, arrêter le serveur buildé localement :
    curl -s http://localhost/shutdown
    # Attendre 2 secondes

15. **Télécharger l'exécutable GitHub Release (PROD uniquement)**
    Télécharger le binaire Windows depuis la release GitHub :

    # PowerShell (Windows)
    $version = "X.Y.0"
    $url = "https://github.com/CCoupel/BuzzMaster/releases/download/v$version/buzzcontrol-v$version-windows-amd64.exe"
    Invoke-WebRequest -Uri $url -OutFile "server-go/server.exe"

    # Ou avec curl
    curl -L -o server-go/server.exe "https://github.com/CCoupel/BuzzMaster/releases/download/vX.Y.0/buzzcontrol-vX.Y.0-windows-amd64.exe"

16. **Lancer le serveur dans une fenêtre visible (PROD uniquement)**
    L'utilisateur doit voir les logs dans la console :

    # PowerShell (depuis la racine du projet)
    Start-Process -FilePath "server.exe" -WorkingDirectory "server-go"

    # Ou CMD (depuis la racine du projet)
    start cmd /k "cd server-go && server.exe"

    Attendre 3-5 secondes pour le démarrage

17. **Validation finale de la release (PROD uniquement)**
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

18. **Générer le rapport de redéploiement** avec :
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
| Build Windows | Oui | Oui | Oui |
| Build ARM64 | Non | Oui | Oui |
| Redémarrage Windows (build local) | Oui | Oui | Oui |
| Push branche feature | Non | Non | Oui |
| Squash merge main | Non | Non | Oui |
| Tag Git | Non | Non | Oui |
| Attendre + vérifier CI (utilisateur) | Non | Non | Oui |
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
- TOUJOURS arrêter le serveur avant de rebuild
- TOUJOURS finaliser la documentation AVANT le build (PROD)
- TOUJOURS mettre z=0 dans la version pour une release (ex: 2.45.3 → 2.45.0)
- TOUJOURS marquer les tâches comme complétées (TaskUpdate) avant le push (PROD)
- TOUJOURS commit la doc AVANT le push et le merge (PROD)
- TOUJOURS vérifier que /version correspond à config.json après redémarrage
- TOUJOURS retry automatique (max 2x) si version mismatch
- TOUJOURS synchroniser package.json avec config.json
- TOUJOURS demander à l'utilisateur de vérifier la CI sur GitHub (PROD)
- TOUJOURS rollback (revert + delete tag) si CI échoue (PROD)
- TOUJOURS télécharger l'exécutable Windows depuis GitHub Release (PROD)
- TOUJOURS lancer l'exécutable release dans une FENÊTRE VISIBLE (pas en arrière-plan)
- TOUJOURS valider la version de l'exécutable release avant de terminer (PROD)
- JAMAIS déployer PROD sans PREPROD validée
- JAMAIS créer des tags Git en QUALIF ou PREPROD
- JAMAIS merge main en QUALIF ou PREPROD
- JAMAIS supprimer la branche feature après merge (garde pour rollback CI)
- PREPROD valide que le build ARM64 compile correctement
```

## Action immédiate

Lance maintenant le sous-agent deploy avec le Task tool.
