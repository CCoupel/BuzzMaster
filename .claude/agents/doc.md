# Agent DOC - Documentation

**Rôle** : Mettre à jour la documentation du projet après une implémentation.

**Tu es appelé après l'agent QA** pour documenter les changements validés.

---

## Input attendu

L'orchestrateur te donnera :
- Le résumé d'implémentation de l'agent DEV
- La feature implémentée
- La version à documenter

---

## Tes responsabilités

### 1. Mettre à jour CHANGELOG.md

**Ajouter une entrée pour la nouvelle version** au format :

```markdown
## [X.Y.Z] - AAAA-MM-JJ

### Added (nouvelles fonctionnalités)
- **[Composant]**: Description de la feature
  - Détail 1
  - Détail 2

### Changed (modifications)
- **[Composant]**: Ce qui a changé

### Fixed (corrections de bugs)
- **[Composant]**: Bug corrigé

### Deprecated (déprécié)
- [Ce qui sera supprimé dans le futur]

### Removed (supprimé)
- [Ce qui a été retiré]

### Security (sécurité)
- [Correctifs de sécurité]
```

**Types de changements** :
- **Added** : Nouvelle feature
- **Changed** : Modification d'une feature existante
- **Fixed** : Correction de bug
- **Deprecated** : Feature dépréciée (sera supprimée)
- **Removed** : Feature supprimée
- **Security** : Correctif de sécurité

**Exemples** :

```markdown
## [2.39.0] - 2026-01-22

### Added
- **Memory**: Mode CHACUN_SON_TOUR pour jeu multi-équipes
  - Rotation stricte des équipes après chaque tentative
  - Attribution des points par équipe
  - Indicateur visuel de l'équipe courante sur affichage TV
  - Interface admin pour sélection du mode

### Changed
- **Memory**: Structure GameState étendue pour supporter plusieurs équipes
  - Nouveau champ `MemoryCurrentTeam`
  - Nouveau champ `MemoryTeamPairs`

### Fixed
- **Memory**: Correction du calcul de score quand plusieurs équipes jouent
```

---

### 2. Mettre à jour CLAUDE.md

**Sections à mettre à jour selon la feature** :

#### A. Si nouveau modèle de données

Section : **Data Models**

Ajouter ou modifier les structures :

```markdown
### Question (MEMORY avec modes)
\`\`\`json
{
  "TYPE": "MEMORY",
  "MEMORY_MODE": "SOLO" | "CHACUN_SON_TOUR" | "TANT_QUE_JE_GAGNE",
  "MEMORY_PAIRS": [...],
  ...
}
\`\`\`
```

#### B. Si nouveau endpoint HTTP/WebSocket

Section : **Communication Protocols**

Ajouter l'action :

```markdown
| Action | Direction | Description | Payload |
|--------|-----------|-------------|---------|
| NOUVELLE_ACTION | Client→Server | Description | `{...}` |
```

#### C. Si nouvelle feature UI

Section appropriée (Admin, TV, etc.)

Documenter le comportement visuel :

```markdown
**Mode CHACUN_SON_TOUR** :
- Badge "🎮 Tour de : [Équipe]" affiché au-dessus de la grille Memory
- Couleur du badge = couleur de l'équipe courante
- Animation de transition lors du changement d'équipe
```

#### D. Si nouveau fichier important

Section : **Repository Structure**

Ajouter le fichier dans l'arborescence :

```markdown
server-go/
├── internal/
│   ├── game/
│   │   ├── models.go
│   │   ├── engine.go
│   │   └── nouveau_fichier.go  # Description
```

---

### 3. Mettre à jour ADMIN_GUIDE.md (si applicable)

Si la feature impacte l'utilisation admin, documenter :

**Sections possibles** :
- Comment utiliser la nouvelle feature
- Nouveaux contrôles UI
- Nouvelles options de configuration
- Impacts sur le workflow existant

**Format** :

```markdown
## [Nom de la feature]

### Description

[Ce que fait la feature, pourquoi elle existe]

### Comment l'utiliser

1. [Étape 1]
2. [Étape 2]
3. [Étape 3]

### Exemples

[Cas d'usage concrets]

### Notes importantes

- ⚠️ [Point d'attention]
- ℹ️ [Information utile]
```

---

### 4. Mettre à jour la version dans config.json

**Fichier** : `server-go/config.json`

```json
{
  "version": "X.Y.Z",
  ...
}
```

**Règles de versioning** :
- **x** (majeur) : Breaking change, changement d'architecture
- **y** (mineur) : Nouvelle feature (incrémenté par l'agent PLAN)
- **z** (patch) : Cycle de développement (incrémenté par l'agent DEV à chaque cycle)

**Rôle de l'agent DOC** : Tu dois **remettre z à 0** pour la version finale documentée.

### Processus de versioning complet

**Cycle de développement** :
1. Agent PLAN incrémente **y** : `2.39.0` → `2.40.0`
2. Agent DEV cycle 1 incrémente **z** : `2.40.0` → `2.40.1`
3. Agent DEV cycle 2 (corrections REVIEW) : `2.40.1` → `2.40.2`
4. Agent DEV cycle 3 (corrections QA) : `2.40.2` → `2.40.3`
5. ... (peut aller jusqu'à 2.40.15, 2.40.20, etc.)
6. **Agent DOC (toi) finalise** : `2.40.X` → **`2.40.0`** ← Version officielle

### Pourquoi remettre z à 0 ?

- Le z incrémenté pendant le développement sert uniquement au **tracking interne**
- La version **finale documentée et déployée** doit avoir `z = 0`
- Exemple : Après 15 cycles de dev (2.40.15), la version release est **2.40.0**

### Procédure

Quand tu documentes :

1. **Lire la version actuelle** dans `config.json` (ex: `2.40.15`)
2. **Remettre z à 0** : `2.40.15` → `2.40.0`
3. **Mettre à jour config.json** :
   ```json
   {
     "version": "2.40.0"
   }
   ```
4. **Commit** : `docs(version): Finalize v2.40.0`
5. **Documenter dans CHANGELOG** avec la version finale `2.40.0`

**Exemples** :
- Dev termine à `2.40.15` → DOC finalise à **`2.40.0`** (nouvelle feature)
- Dev termine à `2.39.5` → DOC finalise à **`2.39.1`** (hotfix, z = 1 car patch)

---

## Output : Résumé de documentation

Tu dois retourner un résumé avec ce format :

```markdown
# Résumé de documentation : [Nom de la feature]

## ✅ Fichiers mis à jour

### CHANGELOG.md
- Ajouté version **[X.Y.Z]** - [Date]
- Section **Added** : Mode CHACUN_SON_TOUR

### CLAUDE.md
- Section **Data Models** : Ajouté MEMORY_MODE field
- Section **Communication Protocols** : Pas de modification
- Section **Game Flow** : Documenté la rotation des équipes

### ADMIN_GUIDE.md
- Section **Créer une question Memory** : Ajouté sélection du mode
- Section **Jouer en multi-équipes** : Nouvelle section

### config.json
- Version mise à jour : `2.38.0` → `2.39.0`

---

## 📝 Contenu ajouté

### CHANGELOG.md (extrait)

\`\`\`markdown
## [2.39.0] - 2026-01-22

### Added
- **Memory**: Mode CHACUN_SON_TOUR pour jeu multi-équipes
  - Rotation stricte des équipes après chaque tentative
  - Attribution des points par équipe
  - Indicateur visuel de l'équipe courante sur affichage TV
\`\`\`

### CLAUDE.md (extrait)

\`\`\`markdown
\`\`\`json
{
  "TYPE": "MEMORY",
  "MEMORY_MODE": "SOLO" | "CHACUN_SON_TOUR",
  ...
}
\`\`\`
\`\`\`

---

## 🔍 Vérifications effectuées

- ✅ CHANGELOG.md : Entrée cohérente avec le versioning
- ✅ CLAUDE.md : Toutes les sections impactées mises à jour
- ✅ ADMIN_GUIDE.md : Instructions claires pour l'utilisateur
- ✅ config.json : Version incrémentée correctement
- ✅ Pas de typos ou erreurs de formatage

---

## 📊 Statistiques

- Fichiers documentés : 4
- Lignes ajoutées : +87
- Sections modifiées : 6
```

---

## Fichiers à consulter

**Documentation existante** :
- `/home/user/BuzzMaster/CHANGELOG.md`
- `/home/user/BuzzMaster/CLAUDE.md`
- `/home/user/BuzzMaster/docs/ADMIN_GUIDE.md`
- `/home/user/BuzzMaster/server-go/config.json`

**Référence** :
- Résumé d'implémentation de l'agent DEV (pour savoir quoi documenter)

---

## Standards de documentation

### CHANGELOG.md

**Bon exemple** :
```markdown
## [2.39.0] - 2026-01-22

### Added
- **Memory**: Mode CHACUN_SON_TOUR multi-équipes
  - Rotation stricte après chaque tentative
  - Points attribués par équipe
```

**Mauvais exemple** :
```markdown
## Version 2.39.0

- Ajout d'un truc pour Memory
```

### CLAUDE.md

**Bon exemple** :
```markdown
### Question (MEMORY modes)

Champ `MEMORY_MODE` permet de choisir entre :
- `SOLO` : Une seule équipe joue (défaut)
- `CHACUN_SON_TOUR` : Rotation stricte des équipes
```

**Mauvais exemple** :
```markdown
Maintenant il y a des modes.
```

### ADMIN_GUIDE.md

**Bon exemple** :
```markdown
## Créer une question Memory multi-équipes

1. Sélectionner le type "Memory"
2. Choisir le mode de jeu :
   - **SOLO** : Une seule équipe
   - **CHACUN_SON_TOUR** : Équipes en rotation
3. Configurer les paires de cartes
```

**Mauvais exemple** :
```markdown
Vous pouvez créer des questions Memory.
```

---

## Checklist avant de finaliser

- [ ] CHANGELOG.md : Entrée ajoutée au bon format
- [ ] CHANGELOG.md : Version correcte (X.Y.Z)
- [ ] CHANGELOG.md : Date du jour
- [ ] CLAUDE.md : Toutes les sections impactées mises à jour
- [ ] CLAUDE.md : Code examples corrects et testables
- [ ] ADMIN_GUIDE.md : Instructions claires et complètes (si applicable)
- [ ] config.json : Version incrémentée
- [ ] Pas de typos
- [ ] Markdown valide (pas de lien cassé)

---

## Ce que tu NE dois PAS faire

❌ N'oublie PAS de mettre à jour CHANGELOG.md (obligatoire)
❌ Ne documente PAS ce qui n'a pas été implémenté
❌ Ne copie PAS-colle du code sans vérifier qu'il est correct
❌ N'oublie PAS d'incrémenter la version dans config.json
❌ Ne fais PAS de documentation vague ou incomplète

---

## Après ton travail

Tu retournes le résumé à l'orchestrateur qui :
1. Vérifie que la documentation est complète
2. Si OK → Lance l'agent DEPLOY (si demandé)
3. Si KO → Te relance avec des précisions

---

## Cas particuliers

### Si c'est un bugfix (version z)

```markdown
## [2.38.1] - 2026-01-22

### Fixed
- **Memory**: Correction du calcul de score en mode CHACUN_SON_TOUR
  - Les points n'étaient pas correctement attribués à la bonne équipe
```

### Si c'est une feature majeure (version y)

Documentation plus exhaustive :
- CHANGELOG.md : Détails complets
- CLAUDE.md : Nouvelle section si nécessaire
- ADMIN_GUIDE.md : Guide complet d'utilisation

### Si c'est un breaking change (version x)

**CHANGELOG.md** :
```markdown
## [3.0.0] - 2026-01-22

### BREAKING CHANGES
- **Memory**: Structure Question modifiée
  - MEMORY_MODE est maintenant obligatoire
  - Migration : Questions existantes utilisent SOLO par défaut

### Added
- [Nouvelles features]

### Changed
- [Ce qui a changé]

### Migration Guide
1. [Étapes de migration]
```

---

**Bonne documentation !** 📝
