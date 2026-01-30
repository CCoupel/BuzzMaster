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

**Environnement cible :** $ARGUMENTS (QUALIF par défaut si vide)

**Étapes à exécuter :**

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
   - Documentation à jour (PROD)
   - Version incrémentée

4. **Build selon environnement**
   cd server-go

   # QUALIF : Windows uniquement
   go build -o server.exe ./cmd/server

   # PREPROD/PROD : Windows + ARM64
   go build -o server.exe ./cmd/server
   GOOS=linux GOARCH=arm64 go build -o buzzcontrol ./cmd/server

5. **Redémarrer le serveur Windows**
   - Lancer server.exe en arrière-plan
   - Attendre 2-3 secondes pour le démarrage complet
   - Le serveur reste actif après les tests

6. **Vérification finale de la version (avec retry automatique)**
   - Lire la version attendue depuis server-go/config.json
   - Appeler curl http://localhost/version
   - **COMPARER** : La version retournée DOIT correspondre à celle de config.json
   - **Si DIFFÉRENCE** :
     1. Afficher WARNING : "Version mismatch: expected X.Y.Z, got A.B.C"
     2. Arrêter le serveur : curl http://localhost/shutdown
     3. Attendre 2 secondes
     4. **RELANCER** les étapes 4, 5, 6 (rebuild + restart + verify)
     5. Maximum 2 tentatives de retry
     6. Si échec après 2 retries → **ERREUR CRITIQUE** : Arrêter et escalader
   - **Si IDENTIQUE** : Vérifier /listGame fonctionne → Continuer

7. **Synchroniser package.json (PROD uniquement)**
   - Lire la version depuis server-go/config.json
   - Mettre à jour server-go/web/package.json → "version": "<version>"
   - Commit: git commit -am "chore(version): Sync package.json to vX.Y.Z"
   - Push la branche feature

8. **Vérifier état Git avant merge (PROD uniquement - OBLIGATOIRE)**
   - Exécuter `git status` pour vérifier qu'il n'y a pas de fichiers non commités
   - Exécuter `git log origin/<branche>..<branche>` pour vérifier qu'il n'y a pas de commits non pushés
   - **Si fichiers non commités** → STOP : Commiter et pusher d'abord
   - **Si commits non pushés** → STOP : Pusher d'abord
   - **Si tout est propre** → Continuer vers le merge

9. **Git (PROD uniquement)**
   - Squash merge vers main
   - Tag annotée v<version>
   - Push tag
   - NE PAS supprimer la branche (garder l'historique)

10. **Générer le rapport de redéploiement** avec :
   - Informations : Version, env, date, branche, commit
   - Arrêt : Résultat de l'arrêt du serveur précédent
   - Builds : Résultats + tailles binaires (Windows + ARM64 si applicable)
   - Redémarrage : Résultats des tests post-build
   - Vérification version : Version attendue vs version serveur (DOIT MATCHER)
   - Retries : Nombre de tentatives si version mismatch (0 = succès direct)
   - Git (PROD) : Merge, tag, cleanup
   - Décision : SUCCESS / FAILED

**Différences par environnement :**
| Action | QUALIF | PREPROD | PROD |
|--------|--------|---------|------|
| Arrêt serveur | Oui | Oui | Oui |
| Build Windows | Oui | Oui | Oui |
| Build ARM64 | Non | Oui | Oui |
| Redémarrage Windows | Oui | Oui | Oui |
| Sync package.json | Non | Non | Oui |
| Squash merge main | Non | Non | Oui |
| Tag Git | Non | Non | Oui |
| Conserver branche | - | - | Oui |

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
- TOUJOURS vérifier que /version correspond à config.json après redémarrage
- TOUJOURS retry automatique (max 2x) si version mismatch
- TOUJOURS synchroniser package.json avec config.json avant le tag PROD
- JAMAIS déployer PROD sans PREPROD validée
- JAMAIS créer des tags Git en QUALIF ou PREPROD
- JAMAIS merge main en QUALIF ou PREPROD
- Le serveur Windows doit rester actif après le redéploiement
- PREPROD valide que le build ARM64 compile correctement
```

## Action immédiate

Lance maintenant le sous-agent deploy avec le Task tool.
