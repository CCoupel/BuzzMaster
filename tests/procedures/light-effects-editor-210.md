# Procédure de Test — Éditeur d'effets lumière pas à pas (#210, v11.8.0 LOT-2)

**Version** : v11.8.0 (QUALIF)
**Date** : 2026-11-12
**Testeur** : QA (scénarios `AUTO`) ; l'utilisateur pour les scénarios `HUMAIN` (rendu, ergonomie, lisibilité, essai sur les vraies ampoules)
**Contrats** : `contracts/light-effects.md` §11.1 à §11.4, §11.8, §11.10, §12 (format révision 7)
**Maquette** : `docs/mockup/v11.8.0/ui/ambiance_page__effect_editor.html` (révision 4)
**Tests automatiques** : `tests/INDEX.md` (lignes #210)

> Sauvegarder `data/files/effects/` avant. Les exemples A et B du contrat §12.5 servent de jeux d'essai : les créer dans l'onglet Texte (copier-coller depuis le contrat).

## Prérequis

- [ ] Environnement : QUALIF ou LOCAL, page Ambiance › Lumière › Effets
- [ ] Données : aucun effet utilisateur (ou en sauvegarde) ; au moins un pont ou une zone avec groupes pour le scénario 9
- [ ] Un navigateur récent ; zoom 100 %

## Scénarios

### Scénario 1 — Chronogramme de l'exemple A (format révision 7)

**Validation** : AUTO — **Issues** : #210

**Objectif** : le serveur rend les positions du contrat §12.5 (expressions, suivre, pas relatif).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `curl -s -X POST http://<hôte>/api/lighting/effects/draft -H 'Content-Type: application/json' -d '{"yaml": "<exemple A du §12.5>", "groups": [5]}'` | `ok: true`, `previews[0].frames[k].vars` : gA = 2,3,4,5,1 ; gB = 3,4,5,1,2 ; gC = 5,4,3,2,1 (ticks 0 à 4) | | |
| 2 | Même appel avec `"groups": [2]` | gC reste à 2 (pas nul : ne bouge pas) | | |
| 3 | `ls data/files/effects/` | Rien n'a été écrit par `draft` | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 2 — Refus de `gN` non déclaré (BREAKING) et messages de validation

**Validation** : AUTO — **Issues** : #210

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Déposer à la main un fichier utilisateur dont un pas écrit `gN: [theme, 100]` sans variable `gN`, puis « Recharger les fichiers » | Fichier écarté : bandeau d'erreur du catalogue avec nom, ligne et message « gN … écrivez g* » ; les autres effets restent chargés | | |
| 2 | Remplacer `gN` par `g*`, recharger | L'effet apparaît au catalogue | | |
| 3 | `draft` avec `gA: N//2` | `ok: false`, message « l'expression « N//2 » est mal écrite », `line` renseignée | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 3 — Créer un effet dans la grille

**Validation** : HUMAIN — **Issues** : #210

**Objectif** : l'éditeur se comprend sans notice : fiche, pinceau, grille, variables, bulles « ? ».

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | « ＋ Nouvel effet », saisir nom et identifiant | L'éditeur s'ouvre sur un pas (g0 au thème), aucune variable, conforme à la maquette §04 | | |
| 2 | Choisir le pinceau « urgence », cliquer des cases | Les cases se remplissent (couleur et intensité lisibles) ; « modifications non enregistrées » apparaît | | |
| 3 | Pinceau « couleur libre » | La roue apparaît ; cliquer dans la roue, saisir un code `#RRGGBB`, utiliser une couleur récente | | |
| 4 | Pinceau « éteint », « gomme » | Case « éteint » ; la gomme vide la case | | |
| 5 | Survoler (puis atteindre au clavier, Tab) chaque pinceau et « Intensité ⓘ » | Info-bulle de définition, au survol et au focus clavier | | |
| 6 | « + Ajouter » une variable, départ `N/2`, pas `+1`, puis « ? » | La ligne gA apparaît en haut de la grille ; la bulle donne la phrase et la suite pour 1, 3, 6 et 12 groupes ; elle se ferme au clic suivant et à Échap | | |
| 7 | Saisir un départ erroné (`N//2`) | Message d'erreur sous la variable, bouton Enregistrer grisé | | |
| 8 | Ajouter, dupliquer, déplacer, supprimer un pas (menu ▾ de l'en-tête) | Chaque opération agit sur le bon pas | | |
| 9 | « Ajouter un groupe » / « Retirer un groupe » | La ligne g* garde son contenu et devient g6 ; une nouvelle g* vide apparaît ; au retrait avec g* non vide, une confirmation est demandée | | |
| 10 | Enregistrer | « modifications non enregistrées » disparaît ; l'effet est au catalogue ; « Télécharger » apparaît | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 4 — Fichier ⇄ grille ⇄ texte

**Validation** : AUTO — **Issues** : #210

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `chenillard-traine` (livré) dans l'éditeur | Lecture seule ; message « voisins remplacés par les variables gB, gC : le rendu est le même » | | |
| 2 | `curl -s .../effects/chenillard-traine/source` | `grid.converted` = 2 entrées ; `grid.effect.variables` = gB: gA-1, gC: gA+1, gA | | |
| 3 | Dupliquer, modifier un pas dans la grille, passer à Texte | Le texte devient le fichier régénéré (commentaires perdus, avertissement si le fichier en avait) | | |
| 4 | Dans Texte, saisir un texte invalide, cliquer « Grille » | Refus avec message ; l'onglet Texte reste affiché | | |
| 5 | Corriger le texte, cliquer « Grille » | La grille reprend le texte | | |
| 6 | Enregistrer depuis Texte un fichier avec des commentaires `#` | `diff` du fichier du serveur avec le texte saisi : identiques à l'octet près | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 5 — Effets livrés en lecture seule, Dupliquer

**Validation** : AUTO — **Issues** : #210

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir « Pulsation » (livré) | Bandeau « effet livré : il ne se modifie pas », pas de bouton Enregistrer, champs verrouillés | | |
| 2 | `curl -s -X PUT .../effects/pulsation -d '{"yaml":"…"}'` | `409 {"reason":"native"}`, rien d'écrit | | |
| 3 | « Dupliquer » | Nom et identifiant demandés (identifiant proposé `<id>-copie`) ; l'éditeur s'ouvre sur la copie modifiable | | |
| 4 | Dupliquer avec un identifiant déjà pris | Refus lisible à l'enregistrement (« Un effet porte déjà cet identifiant ») | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 6 — Modifications non enregistrées

**Validation** : HUMAIN — **Issues** : #210

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Modifier une case puis « ← Retour au catalogue » | Question « Modifications non enregistrées » : Enregistrer / Abandonner / Rester | | |
| 2 | Rester, puis Abandonner | Rester garde l'éditeur ; Abandonner revient au catalogue sans rien écrire | | |
| 3 | Aucune modification, « ← Retour » | Retour immédiat sans question | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 7 — Simulateur multi-salles et pas à pas

**Validation** : HUMAIN — **Issues** : #210

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir un effet avec variables ; observer les rangées 1, 3, 6, 12 et les zones réelles | Une rangée d'ampoules par taille, jouées avec une seule horloge ; le pas joué est surligné dans la grille | | |
| 2 | Pause, tick précédent / suivant, début, réglette, clic sur un pas | La lecture se déplace dans les trames déjà reçues ; tick, pas joué et variables affichés | | |
| 3 | Effet demandant plus de groupes que la rangée | « sous le minimum : non joué en salle » ; les cases à cocher masquent une rangée | | |
| 4 | Modifier la grille pendant la lecture | L'aperçu se met à jour après la vérification par le serveur (~300 ms), sans saccade ni erreur | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 8 — Compatibilité des effets livrés et des fichiers v11.7.0

**Validation** : AUTO — **Issues** : #210

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `go test ./internal/effects -run Compat` (ou équivalent QA) | Les 5 effets livrés rendent les mêmes trames qu'en v11.7.0 pour N = 1 à 12 | | |
| 2 | Aperçu d'un effet livré avant / après mise à jour | Même aperçu (le champ `vars` est le seul ajout) | | |
| 3 | Réglages d'ambiance existants | Aucun réglage n'a changé ; les moments continuent de jouer leur effet | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 9 — Essai sur les vraies ampoules (brouillon, avec choix de zone)

**Validation** : HUMAIN — **Issues** : #210

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Dans l'éditeur, modifier le brouillon (sans enregistrer), choisir la zone Animateur, « Essayer sur la salle » | Le **brouillon** est joué sur les ampoules de la zone Animateur, puis l'éclairage revient à la normale ; rien n'a été écrit (`ls data/files/effects/`) | | |
| 2 | Essayer avec un brouillon invalide | Refus lisible, rien n'est joué | | |
| 3 | Essayer pendant une question en cours, ou éclairage désactivé | Bouton grisé avec la raison | | |
| 4 | Lancer un autre essai pendant le premier | Le second remplace le premier ; « arrêter » rend l'éclairage | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] Tous les scénarios nominaux passent
- [ ] Aucun fichier écrit hors de `data/files/effects/`, jamais dans `natifs/`
- [ ] Les 5 effets livrés et les écritures v11.7.0 sont inchangés
- [ ] L'éditeur reste utilisable à la souris et au clavier (info-bulles, bulles « ? », menus)

## Notes QA

[Espace pour observations]
