# CDP_WORKFLOWS.md - Workflows Orchestrés par CDP

Ce fichier centralise les patterns partagés par les commandes `/feature`, `/bugfix`, `/hotfix`, et `/refactor`.

---

## 1. Contexte CDP

```yaml
Agent: CDP (Chef De Projet)
Modèle: haiku (rapide pour orchestration)
Rôle: Orchestrer workflows multi-agents avec validation utilisateur
```

---

## 2. Workflows Disponibles

| Commande | Type | Workflow | Version |
|----------|------|----------|---------|
| `/feature` | FEATURE | Complet | Incrémente Y |
| `/bugfix` | BUGFIX | Simplifié | Incrémente Z |
| `/hotfix` | HOTFIX | Accéléré | Incrémente Z + suffix |
| `/refactor` | REFACTOR | Léger | Aucun changement |

---

## 3. Workflow Standard CDP

```
[INIT] → [ANALYSE] → [DEV] → [REVIEW] → [QA] → [DOC] → [DEPLOY] → [FIN]
```

### Variantes par type

| Phase | FEATURE | BUGFIX | HOTFIX | REFACTOR |
|-------|---------|--------|--------|----------|
| Backlog | Oui | Non | Non | Non |
| Plan | Oui | Souvent | Non | Rarement |
| Dev | Complet | Ciblé | Minimal | Structure |
| Review | Oui | Oui | Rapide | Oui |
| QA | Complet | Régression | Critique | Complet |
| Doc | Oui | Si majeur | Post-mortem | Non |
| Deploy QUALIF | Oui | Oui | Optionnel | Oui |

---

## 4. Phases Communes

### Phase Init (Git)

```bash
# FEATURE
git checkout main && git pull origin main
git checkout -b feature/<nom-court>

# BUGFIX
git checkout main && git pull origin main
git checkout -b bugfix/<nom-court>

# HOTFIX (depuis production)
git checkout main && git pull origin main
git checkout -b hotfix/<nom-court>

# REFACTOR
git checkout main && git pull origin main
git checkout -b refactor/<nom-court>
```

### Phase Versionnement

| Type | Action | Exemple |
|------|--------|---------|
| FEATURE | Incrémente Y, reset Z | 2.40.3 → 2.41.0 |
| BUGFIX | Incrémente Z | 2.40.0 → 2.40.1 |
| HOTFIX | Incrémente Z + suffix | 2.40.1 → 2.40.2-hotfix |
| REFACTOR | Aucun | 2.40.1 (inchangé) |

### Phase Dev (Dispatch)

```
Analyser le scope :
├── Backend seul → dev-backend
├── Frontend seul → dev-frontend
├── Les deux (dépendants) → dev-backend PUIS dev-frontend
└── Les deux (indépendants) → dev-backend ET dev-frontend (parallèle)
```

### Phase Review

```
Lancer code-reviewer
├── APPROVED → Phase QA
├── APPROVED WITH RESERVATIONS → Phase QA (noter réserves)
└── REJECTED → Retour Phase Dev (cycle++)
```

### Phase QA

```
Lancer QA
├── VALIDATED → Phase Doc
├── VALIDATED WITH RESERVATIONS → Demander confirmation utilisateur
└── NOT VALIDATED → Retour Phase Dev (cycle++)

Si cycle > 3 → ESCALADE utilisateur
```

---

## 5. Points de Validation Utilisateur

| Point | Conditions | Options |
|-------|------------|---------|
| Backlog | FEATURE uniquement | Confirmer / Refuser / Autre |
| Plan | Si création plan | Valider / Modifier / Refuser |
| QA réserves | Réserves mineures | Continuer / Corriger |
| Escalade | 3 cycles atteints | Continuer / Abandonner |

---

## 6. Gestion des Erreurs CDP

| Situation | Action |
|-----------|--------|
| Backlog non trouvé | Proposer création ou continuer sans |
| Plan refusé | Demander modifications |
| Review rejetée | Retour DEV avec corrections |
| QA échoue | Retour DEV avec erreurs |
| Build échoue | Retour DEV avec erreur build |
| 3 cycles atteints | Escalade utilisateur |

---

## 7. Rapport Final CDP

```markdown
## Rapport de Workflow [TYPE]

**Informations**
- Type : [FEATURE|BUGFIX|HOTFIX|REFACTOR]
- Branche : [nom]
- Version : [X.Y.Z]
- Durée : [temps]
- Cycles : [nombre]

**Livrables**
- Code : [fichiers modifiés]
- Tests : [ajoutés/modifiés]
- Documentation : [mise à jour]

**Prochaines étapes**
- Valider QUALIF
- `/deploy PROD` après validation
```

---

## 8. Règles par Type

### FEATURE

- Scope large autorisé
- Refactoring autorisé
- Tests nouveaux requis
- Documentation complète
- QUALIF obligatoire

### BUGFIX

- Scope minimal obligatoire
- Pas de refactoring
- Test non-régression OBLIGATOIRE
- Doc si majeur
- QUALIF obligatoire

### HOTFIX

- Fix minimal UNIQUEMENT
- Pas de refactoring
- Test critique obligatoire
- QUALIF optionnel si urgent
- Post-mortem requis

### REFACTOR

- Comportement identique obligatoire
- Tests AVANT refactoring
- Incrémental (petits changements)
- Pas de documentation
- QUALIF pour validation

---

## Usage

Dans les commandes CDP, référencer ce fichier :

```markdown
**Workflow CDP :** Voir `context/CDP_WORKFLOWS.md`
- Type : FEATURE|BUGFIX|HOTFIX|REFACTOR
- Phases : section 3
- Validation : section 5
- Erreurs : section 6
```
