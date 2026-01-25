# Commande /backlog - Gestion du Backlog

## Argument reçu

$ARGUMENTS

## Structure du backlog

Le backlog est organisé en fichiers séparés dans le dossier `backlog/` :
- `backlog/README.md` : Index principal avec statuts
- `backlog/<nom-feature>.md` : Spécification détaillée par feature

## Comportement

### Si aucun argument fourni → Afficher le backlog EXHAUSTIF

1. Lire le fichier `backlog/README.md` pour identifier tous les fichiers
2. **Lire CHAQUE fichier** du backlog pour extraire le contenu détaillé
3. Pour chaque feature, identifier :
   - Les phases/sections implémentées (cochées `[x]`)
   - Les phases/sections non implémentées (non cochées `[ ]`)
   - Le statut global et la version

4. Afficher dans cet ordre STRICT :

```
## Backlog BuzzControl

---

### ⏳ EN COURS

#### feature-name.md (vX.Y.Z)
**Description** : [description courte]

**Non implémenté :**
- [ ] Phase X - Nom de la phase
  - Sous-tâche 1
  - Sous-tâche 2
- [ ] Phase Y - Autre phase
  - ...

**Implémenté :**
- [x] Phase 1 - Nom (vX.Y.Z)
- [x] Phase 2 - Nom (vX.Y.Z)

---

### 📋 PLANIFIÉ (non implémenté)

#### autre-feature.md
**Description** : [description courte]

**À implémenter :**
- [ ] Phase 1 - Nom
  - Détails des tâches
- [ ] Phase 2 - Nom
  - Détails des tâches

---

### ✅ COMPLÉTÉ

| Feature | Version | Description |
|---------|---------|-------------|
| feature-a.md | v2.18.0 | Description courte |
| feature-b.md | v2.34.0 | Description courte |

---

### 🔮 IDÉES
- (aucune)
```

**IMPORTANT** : Être EXHAUSTIF sur les évolutions définies. Lister toutes les phases, tous les modes, toutes les options documentées dans chaque fichier. Ne pas résumer, montrer le détail.

### Si argument fourni → Ajouter au backlog

**RÈGLE IMPORTANTE** : Toujours créer une NOUVELLE entrée de backlog. Ne JAMAIS modifier une entrée existante, sauf si l'utilisateur demande explicitement de modifier un fichier backlog spécifique.

**Exemples** :
- "ajouter des marqueurs QCM" → Créer `backlog/qcm-marqueurs.md` (même si `qcm-indices-penalites.md` existe)
- "modifier backlog/qcm-indices-penalites.md pour ajouter X" → OK, modification explicite demandée

1. Générer un nom de fichier à partir de la description (kebab-case)
2. Créer le fichier `backlog/<nom>.md` avec le template :

```markdown
# <Titre de la fonctionnalité>

**Statut** : 📋 Planifié

## Description

<Description fournie par l'utilisateur>

## Objectifs

- [ ] À définir

## Tâches

### Phase 1
- [ ] À définir

## Version cible

vX.Y.Z (à déterminer)
```

3. Mettre à jour `backlog/README.md` pour ajouter la référence
4. **Afficher un résumé** de ce qui a été créé/modifié
5. **Demander confirmation** à l'utilisateur avant de commit et push
6. Si confirmé → Commit et push

## Exemples

### Mode lecture

```
/backlog
```

→ Affiche le backlog EXHAUSTIF avec TOUTES les évolutions définies dans chaque fichier

### Mode ajout

```
/backlog Mode sombre pour l'interface admin
```

→ Crée `backlog/mode-sombre-admin.md` et met à jour le README

## Légende des statuts

- ⏳ **En cours** : Implémentation en cours (montrer détail non implémenté + implémenté)
- 📋 **Planifié** : Non démarré (montrer tout le détail à implémenter)
- ✅ **Complété** : Tout implémenté (tableau résumé avec version)
- 🔮 **Idée** : Concept à explorer

## Ordre d'affichage (STRICT)

1. **⏳ En cours** - Priorité max, travail actif
2. **📋 Planifié** - Prochaines fonctionnalités
3. **✅ Complété** - Référence historique (en dernier)
4. **🔮 Idées** - Si présentes

## Commence maintenant

**Argument reçu** : $ARGUMENTS

- Si vide → Lire `backlog/README.md` ET tous les fichiers référencés, afficher exhaustivement
- Si texte → Créer un nouveau fichier backlog et mettre à jour le README
