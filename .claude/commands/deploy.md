# Commande /deploy - Arrêt et Redéploiement QUALIF / PROD

Arrête le serveur en cours puis redéploie vers l'environnement cible.

## Argument reçu

$ARGUMENTS

**Format** : `/deploy [QUALIF|PROD|hotfix]`

- **QUALIF** (défaut) : Redéploiement de qualification
- **PROD** : Redéploiement de production (squash merge + tag + release)
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
   - Tests QA passés
   - Review approuvée
   - Documentation à jour
   - Version incrémentée

4. **Build**
   cd server-go
   # Windows
   go build -o server.exe ./cmd/server
   # Linux ARM64 (Raspberry Pi)
   GOOS=linux GOARCH=arm64 go build -o buzzcontrol ./cmd/server

5. **Redémarrer le serveur**
   - Lancer le serveur en arrière-plan
   - Vérifier /version et /listGame
   - Le serveur reste actif après les tests

6. **Git (PROD uniquement)**
   - Squash merge vers main
   - Tag annotée v<version>
   - Cleanup branche feature

7. **Générer le rapport de redéploiement** avec :
   - Informations : Version, env, date, branche, commit
   - Arrêt : Résultat de l'arrêt du serveur précédent
   - Builds : Résultats + tailles binaires
   - Redémarrage : Résultats des tests post-build
   - Git (PROD) : Merge, tag, cleanup
   - Décision : SUCCESS / FAILED

**Différences QUALIF vs PROD :**
| Action | QUALIF | PROD |
|--------|--------|------|
| Arrêt serveur | Oui | Oui |
| Build Windows + ARM64 | Oui | Oui |
| Redémarrage + tests | Oui | Oui |
| Squash merge main | Non | Oui |
| Tag Git | Non | Oui |
| Cleanup branche | Non | Oui |

**Règles critiques :**
- TOUJOURS arrêter le serveur avant de rebuild
- JAMAIS déployer PROD sans QUALIF validée
- JAMAIS créer des tags Git en QUALIF
- JAMAIS merge main en QUALIF
- Le serveur doit rester actif après le redéploiement
```

## Action immédiate

Lance maintenant le sous-agent deploy avec le Task tool.
