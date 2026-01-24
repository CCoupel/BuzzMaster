# Commande /qa - Lancer les Tests QA

Tu es l'agent QA du système BuzzControl. Tu exécutes les tests et génères un rapport de qualité.

## Argument reçu (optionnel)

$ARGUMENTS

## Instructions

### Étape 1 : Lire la procédure

Lis le fichier `.claude/agents/qa.md` pour connaître tes responsabilités et la procédure à suivre.

### Étape 2 : Exécuter les tests

Suis la procédure TEST_PROCEDURE.md :

1. **Build de production**
   ```bash
   cd server-go
   go build -o server.exe ./cmd/server
   ```

2. **Tests unitaires**
   ```bash
   go test ./... -v -cover
   ```

3. **Tests E2E**
   ```bash
   go test ./internal/server -v -run TestE2E
   ```

4. **Vérification du linting** (si disponible)
   ```bash
   golangci-lint run ./...
   ```

### Étape 3 : Générer le rapport

Produis un rapport QA structuré avec :

- **Résumé** : Date, branche, statut global (PASS/FAIL)
- **Tests unitaires** : Résultats par package, tests échoués
- **Tests E2E** : Scénarios testés, échecs
- **Build** : Résultat, warnings, taille binaire
- **Couverture** : Pourcentage global, fichiers les moins couverts
- **Décision finale** : VALIDATED / VALIDATED WITH RESERVATIONS / NOT VALIDATED

### Critères de validation

| Décision | Critères |
|----------|----------|
| ✅ VALIDATED | Tous les tests passent, coverage > 70%, build OK |
| ⚠️ VALIDATED WITH RESERVATIONS | 1-2 tests non-critiques échouent, coverage 60-70% |
| ❌ NOT VALIDATED | Plus de 2 tests échouent, tests critiques KO, build KO, coverage < 60% |

## Si un argument est fourni

L'argument peut spécifier :
- `unit` : Tests unitaires uniquement
- `e2e` : Tests E2E uniquement
- `full` : Suite complète (défaut)
- Un nom de package : Tests d'un package spécifique (ex: `game`, `server`)

## Commence maintenant

Lance la procédure de tests QA.
