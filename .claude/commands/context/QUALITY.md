# QUALITY.md - Patterns de Qualité

Ce fichier centralise les patterns partagés par les commandes `/code-review`, `/qa`, `/test-write`, et `/review`.

---

## 1. Agents de Qualité

| Commande | Agent | Rôle |
|----------|-------|------|
| `/code-review` | code-reviewer | Analyser le code |
| `/qa` | QA | Exécuter les tests |
| `/test-write` | test-writer | Écrire les tests |
| `/review` | Workflow | Revue périodique complète |

---

## 2. Matrice Qualité

| Commande | Écrit tests | Exécute tests | Analyse code | Workflow |
|----------|-------------|---------------|--------------|----------|
| `/test-write` | Oui | Non | Non | Non |
| `/qa` | Non | Oui | Non | Non |
| `/code-review` | Non | Non | Oui | Non |
| `/review` | Oui | Oui | Oui | Oui |

---

## 3. Niveaux de Sévérité

| Niveau | Symbole | Description | Action |
|--------|---------|-------------|--------|
| Critical | 🔴 | Bloquant (sécurité, bug majeur) | Rejet obligatoire |
| Warning | 🟡 | Important mais non-bloquant | Signaler |
| Rationalization | 🟠 | Duplication > 70% | Recommander |
| Suggestion | 🔵 | Amélioration optionnelle | Proposer |

---

## 4. Framework de Review

### Catégories d'Analyse

| Catégorie | Vérifications |
|-----------|---------------|
| Qualité | Naming, fonctions courtes, comments, errors |
| Sécurité | Injection, XSS, secrets, validation |
| Performance | Boucles, re-renders, structures |
| Architecture | CLAUDE.md conformité, patterns |
| Rationalisation | Duplications, patterns répétés |

### Focus Spécialisés

```bash
/code-review security      # OWASP Top 10
/code-review performance   # Optimisations
/code-review rationalization  # Duplications
```

---

## 5. Verdicts Standards

### Code Review

| Verdict | Signification | Suite |
|---------|---------------|-------|
| APPROVED | Code prêt | → QA |
| APPROVED WITH RESERVATIONS | Mineur à noter | → QA (noter) |
| REJECTED | Issues critiques | → Retour DEV |

### QA

| Verdict | Signification | Suite |
|---------|---------------|-------|
| VALIDATED | Tests OK | → DOC |
| VALIDATED WITH RESERVATIONS | Mineurs échoués | → Confirmation |
| NOT VALIDATED | Tests critiques KO | → Retour DEV |

---

## 6. Critères de Validation QA

| Critère | VALIDATED | RESERVATIONS | NOT VALIDATED |
|---------|-----------|--------------|---------------|
| Tests | 100% pass | 1-2 non-critiques KO | >2 KO ou critiques KO |
| Coverage | > 70% | 60-70% | < 60% |
| Build | OK | OK | KO |

---

## 7. Commandes de Test

### Tests Unitaires

```bash
# Tous les tests
go test ./... -v -cover

# Package spécifique
go test ./internal/game/... -v

# Avec race detector
go test -race ./...

# Couverture HTML
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

### Tests E2E

```bash
# Tests E2E complets
go test ./internal/server -v -run TestE2E

# Tests E2E Chrome (nécessite Playwright)
cd server-go/web && npm run test:e2e
```

---

## 8. Structure Rapport Review

```markdown
## Rapport de Revue de Code

📊 **Overview**
- Fichiers analysés : [nombre]
- Lignes de code : [nombre]
- Statut global : [APPROVED|RESERVATIONS|REJECTED]

✅ **Points positifs**
- [liste]

⚠️ **Issues détectées**
- 🔴 Critical : [liste]
- 🟡 Warning : [liste]
- 🟠 Rationalization : [liste]
- 🔵 Suggestion : [liste]

🔒 **Sécurité**
- [analyse OWASP]

📈 **Performance**
- [analyse]

🏗️ **Architecture**
- [conformité CLAUDE.md]

🔄 **Rationalisation**
- [duplications détectées]

📝 **Tests**
- [qualité des tests]

🎯 **Recommandations**
- [liste prioritaire]

✅ **Décision** : [VERDICT]
```

---

## 9. Structure Rapport QA

```markdown
## Rapport QA

📊 **Résumé**
- Date : [date]
- Branche : [branche]
- Version : [X.Y.Z]
- Statut : [VALIDATED|RESERVATIONS|NOT VALIDATED]

🧪 **Tests Unitaires**
- Total : [nombre]
- Passés : [nombre]
- Échoués : [nombre]
- Coverage : [%]

🔗 **Tests E2E**
- Scénarios : [nombre]
- Passés : [nombre]
- Échoués : [nombre]

📦 **Build**
- Statut : [OK|KO]
- Taille : [MB]

❌ **Échecs détaillés**
- [liste avec messages d'erreur]

✅ **Décision** : [VERDICT]
```

---

## 10. Règles Critiques

| Agent | Règle |
|-------|-------|
| code-reviewer | NE PAS corriger le code (juste reviewer) |
| code-reviewer | NE PAS approuver si issue critique sécurité |
| QA | NE PAS modifier de code (juste tester) |
| test-writer | NE PAS exécuter les tests (juste écrire) |
| Tous | NE PAS ignorer les duplications |

---

## 11. Workflow /review (Revue Périodique)

```
[Vérifications] → PLAN → [Validation] → DEV → QA → [Validation] → DOC → DEPLOY(QUALIF)
```

### Étapes

1. **Analyse initiale** : code-reviewer sur codebase
2. **Plan** : Créer plan de corrections/améliorations
3. **Validation utilisateur** : Confirmer le plan
4. **Développement** : Appliquer les corrections
5. **QA** : Valider les changements
6. **Documentation** : Mettre à jour si nécessaire

---

## Usage

Dans les commandes Qualité, référencer ce fichier :

```markdown
**Contexte Qualité :** Voir `context/QUALITY.md`
- Framework review : section 4
- Verdicts : section 5
- Tests : section 7
- Rapports : sections 8-9
```
