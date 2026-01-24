# Commande /qa - Lancer les Tests QA

Lance le sous-agent QA pour exécuter les tests et générer un rapport de qualité.

## Argument reçu (optionnel)

$ARGUMENTS

## Instructions

Utilise le Task tool pour lancer le sous-agent QA avec les paramètres suivants :

```
subagent_type: "QA"
description: "Exécuter tests QA"
prompt: voir ci-dessous
```

### Prompt à transmettre au sous-agent

```
Exécute la procédure de tests QA complète pour BuzzControl.

**Contexte projet :**
- Répertoire : C:\Users\cyril\Documents\VScode\buzzcontrol
- Serveur Go : server-go/
- Procédure : docs/TEST_PROCEDURE.md

**Argument utilisateur (optionnel) :** $ARGUMENTS
- `unit` : Tests unitaires uniquement
- `e2e` : Tests E2E uniquement
- `full` ou vide : Suite complète (défaut)
- Nom de package : Tests d'un package spécifique (ex: `game`, `server`)

**Étapes à exécuter :**

1. Build de production
   cd server-go && go build -o server.exe ./cmd/server

2. Tests unitaires (sauf si argument = e2e)
   go test ./... -v -cover

3. Tests E2E (sauf si argument = unit)
   go test ./internal/server -v -run TestE2E

4. Génère un rapport QA structuré avec :
   - Résumé : Date, branche, statut global
   - Tests unitaires : Résultats par package, tests échoués
   - Tests E2E : Scénarios testés, échecs
   - Build : Résultat, taille binaire
   - Couverture : Pourcentage global
   - Décision finale : VALIDATED / VALIDATED WITH RESERVATIONS / NOT VALIDATED

**Critères de validation :**
- VALIDATED : Tous tests passent, coverage > 70%, build OK
- VALIDATED WITH RESERVATIONS : 1-2 tests non-critiques échouent, coverage 60-70%
- NOT VALIDATED : Plus de 2 tests échouent, tests critiques KO, build KO, coverage < 60%
```

## Action immédiate

Lance maintenant le sous-agent QA avec le Task tool.
