# Procédure de Test — #221 : sélecteurs catégorie / difficulté unifiés (`CategoryPicker` / `DifficultyPicker`)

**Version** : 11.2.2 (LOT-4)
**Date** : 2026-10-15
**Testeur** : utilisateur (QA n'exécute pas les procédures manuelles — pas de navigateur en session agent)
**Automatique associé** : `server-go/web/src/components/CategoryPicker.test.jsx`, `CategoryPicker.create.test.jsx`, `DifficultyPicker.test.jsx` (composants) + tests de pages adaptés (`QuestionsPage.rafale`, `RafalePage.rafale`, `GamePage.categories`, `QuestionsPage.v571`)
**Pourquoi un test manuel** : les tests automatiques vérifient rôles, `aria-*` et libellés dans jsdom ; ils ne voient ni le rendu visuel (icônes, couleurs, ✓, étoiles ambre), ni le thème sombre de l'éditeur de carte MEMOTION, ni la navigation clavier réelle, ni la persistance côté serveur (sauvegarde puis rechargement).
**Maquette de référence** : `docs/mockup/v11.2.2/ui/admin_pickers__category_difficulty_unified.html`

## Ce qui change (à garder en tête)

Quinze sites utilisent désormais les deux mêmes composants. Chaque site affiche un **badge de mode** dans le DOM :

| Badge | Signification | Comportement |
|-------|---------------|--------------|
| **une seule** | sélection unique | boutons radio ; re-cliquer la catégorie active la désélectionne (catégorie) ; une difficulté, elle, reste sélectionnée |
| **plusieurs possibles** | sélection multiple | chips avec ✓ sur les éléments choisis |
| **une seule ou aucune** | filtre ★ du réservoir | re-cliquer l'étoile active retire le filtre |

Hors périmètre (doit rester **inchangé**, ne pas être testé comme migré) : chips Facile…Expert / publics / langue du `QuizMetaForm`, chip « Non utilisees » du réservoir, chips publics / langue / taille de lot des modales IA, affichages en lecture seule (carte de question, bandeau RAFALE de la page Jeu, anim, TV).

## Prérequis

- [ ] Environnement : QUALIF, binaire du LOT-4 déployé
- [ ] Navigateur de bureau (Chrome ou Edge), console (F12) ouverte : noter toute erreur rouge, en particulier un avertissement React « two children with the same key » (cause du bug #197)
- [ ] Données :
  - un quiz avec des questions couvrant plusieurs catégories (au moins Géographie, Histoire, Sciences & Nature)
  - **une catégorie personnalisée** (créée par le scénario 1, ou déjà présente) — indispensable au scénario 9
  - une manche RAFALE **créée avant la migration** (présente dans une sauvegarde de la version précédente) si disponible, pour le scénario 2
  - un réservoir RAFALE avec quelques questions de difficultés ★ / ★★ / ★★★ différentes
  - une clé IA configurée, sinon les scénarios 7 et 8 se limitent à l'ouverture de la modale
- [ ] Notation : `<QUALIF>` = URL de base de l'instance QUALIF ; « badge » = le petit libellé « une seule » / « plusieurs possibles » à côté du titre du sélecteur

---

## Scénario 1 — Quiz : éditeur de question QCM (site 1, catégorie « une seule » + création)
**Objectif** : la sélection unique de catégorie fonctionne comme avant, avec le badge et la création inline.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `<QUALIF>/admin/quiz`, créer une nouvelle question de type QCM | Le sélecteur « Catégorie » affiche le badge **une seule** ; une seule étiquette « Catégorie » (pas de libellé en double) | | |
| 2 | Cliquer « Histoire » | Seule « Histoire » est mise en évidence (bordure + fond à sa couleur) | | |
| 3 | Cliquer « Sciences & Nature » | « Histoire » se désélectionne, « Sciences & Nature » devient la seule active | | |
| 4 | Re-cliquer « Sciences & Nature » | Plus aucune catégorie active (désélection conservée) | | |
| 5 | Cliquer « + Créer » | Le formulaire inline s'ouvre : nom, choix d'image, Valider, Annuler | | |
| 6 | Cliquer « Valider » sans rien saisir | Message « Nom invalide » | | |
| 7 | Saisir un nom, Valider sans image | Message « Image requise » | | |
| 8 | Appuyer sur Échap dans le champ nom | Le formulaire se ferme | | |
| 9 | Rouvrir « + Créer », saisir un nom (ex. « Mascottes »), choisir une image PNG, Valider | La catégorie apparaît dans la liste **une seule fois** et est **déjà sélectionnée** ; le formulaire est refermé | | |
| 10 | Rouvrir « + Créer », saisir « Mascottes » (nom existant) + image, Valider | Message « Cette catégorie existe déjà », le formulaire reste ouvert | | |
| 11 | Compléter la question, enregistrer, recharger la page, rouvrir la question | La catégorie choisie est restaurée à l'identique | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Quiz : manche RAFALE, catégories ET difficultés « plusieurs possibles » (sites 2, 3) — `critical`
**Objectif** : la chaîne RAFALE (sélection → barème → pool → enregistrement → réouverture) est intacte.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Créer une question de type **Rafale** | Deux sélecteurs : « Catégories » et « Difficultés », chacun avec le badge **plusieurs possibles** ; aucun sélecteur de catégorie en « une seule » ni bouton « + Créer » | | |
| 2 | Cocher « Histoire » puis « Géographie » | Les deux sont actives avec un **✓** ; rien d'autre n'est affecté | | |
| 3 | Re-cliquer « Histoire » | « Histoire » perd son ✓, « Géographie » reste cochée | | |
| 4 | Cocher les difficultés ★ et ★★★ | ★ et ★★★ portent un ✓ ; ★★ n'en porte pas | | |
| 5 | Observer le bloc « Points par bonne réponse » | Un champ de barème par ★ cochée (★ et ★★★ ici), aucun pour ★★ | | |
| 6 | Décocher toutes les difficultés | Le bloc de barème par ★ disparaît | | |
| 7 | Recocher ★★ et ★★★, saisir 15 dans le barème ★★ | La valeur est conservée | | |
| 8 | Observer l'alerte du pool RAFALE | L'alerte affiche le nombre de questions disponibles du réservoir pour la sélection courante (comme avant) | | |
| 9 | Basculer le type sur QCM puis revenir sur Rafale | La sélection de catégories est conservée | | |
| 10 | Enregistrer, recharger la page, rouvrir la manche | Mêmes catégories ✓, mêmes difficultés ✓, même barème | | |
| 11 | Si disponible : ouvrir une manche RAFALE créée **avant** la migration (sauvegarde de la version précédente) | Sélection identique à celle enregistrée à l'époque ; la ré-enregistrer sans modification ne change rien au contenu | | |
| 12 | Tenter d'enregistrer sans aucune catégorie ni difficulté | Enregistrement refusé (validation « au moins une catégorie et une difficulté » toujours active) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — MEMOTION : « Valeur de la carte » vs difficultés des questions tirées (sites 4, 5, 6) — `critical`
**Objectif** : la valeur de la carte (une seule) reste distincte des critères de tirage RAFALE (plusieurs).

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Créer une question de type MEMOTION, ajouter une carte de type **RAFALE** | La carte affiche, dans cet ordre : « **Valeur de la carte :** » (badge **une seule**), « **Catégories des questions tirées** » (badge **plusieurs possibles**), « **Difficultés des questions tirées** » (badge **plusieurs possibles**) | | |
| 2 | Vérifier le libellé de la valeur de la carte | Il est **exactement** « Valeur de la carte : » (pas « Difficulté ») | | |
| 3 | Cliquer ★★ puis ★★★ dans « Valeur de la carte » | Une seule valeur active à la fois (radio) ; re-cliquer la valeur active ne la désélectionne pas | | |
| 4 | Cocher deux catégories et deux difficultés dans « questions tirées » | ✓ sur les éléments cochés ; la « Valeur de la carte » ne bouge pas | | |
| 5 | Ajouter une carte **SPEEDY**, puis **QCM**, puis **MEMORY** | Chacune n'affiche que « Valeur de la carte : » ; aucun sélecteur « questions tirées » | | |
| 6 | Enregistrer, recharger, rouvrir | Valeurs de carte, catégories et difficultés tirées restaurées à l'identique pour chaque carte | | |
| 7 | Lancer la carte RAFALE en jeu (page Jeu ou anim) | Le tirage respecte les catégories et difficultés choisies ; les points correspondent à la valeur de la carte | | |
| 8 | Vérifier que l'éditeur n'a pas d'autre changement | Aucun autre contrôle déplacé ou manquant sur la carte | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Quiz : filtre de la liste (site 7)
**Objectif** : le filtre par catégorie de la liste Quiz fonctionne comme avant.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `<QUALIF>/admin/quiz` (liste) | Barre « Filtrer » avec le badge **plusieurs possibles** ; une pastille ronde (icône + libellé) par catégorie présente dans la liste | | |
| 2 | Cliquer « Histoire » | Seules les questions Histoire restent ; la pastille est active | | |
| 3 | Cliquer aussi « Géographie » | Les questions des deux catégories s'affichent | | |
| 4 | Observer le bouton **×** | Il apparaît dès qu'un filtre est actif | | |
| 5 | Cliquer **×** | Tous les filtres sont retirés, toute la liste revient, le × disparaît | | |
| 6 | Filtrer sur une catégorie sans question | Message « aucune question … » | | |
| 7 | Filtrer sur la catégorie d'une manche RAFALE | La manche RAFALE reste trouvée via ses catégories | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Réservoir RAFALE : filtres et formulaire (sites 8, 9, 10, 11)
**Objectif** : le réservoir est cohérent avec le Quiz (catégories en icônes, ★ unique).

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `<QUALIF>/admin/quiz?tab=rafale` | Filtre catégories (badge **plusieurs possibles**, icônes + libellés) et filtre ★ (badge **une seule ou aucune**) avec étoiles fantômes ★☆☆ / ★★☆ / ★★★ ; la chip « Non utilisees » est toujours là, à l'identique | | |
| 2 | Cliquer une catégorie puis une deuxième | Le réservoir se filtre sur l'union ; × pour réinitialiser | | |
| 3 | Cliquer ★★ | Seules les questions ★★ restent | | |
| 4 | Re-cliquer ★★ | Le filtre ★ est retiré (« ou aucune ») | | |
| 5 | Activer « Non utilisees » en même temps qu'un filtre | Les deux filtres se combinent comme avant | | |
| 6 | Ouvrir le formulaire de question : catégorie | Sélecteur « Catégorie » badge **une seule**, avec « + Créer » ; création d'une catégorie comme au scénario 1 | | |
| 7 | Formulaire : difficulté | Sélecteur badge **une seule**, ★ / ★★ / ★★★ ; une difficulté reste toujours sélectionnée | | |
| 8 | Enregistrer une question, la modifier à nouveau | Catégorie et ★ pré-remplies avec les valeurs enregistrées | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — Page Jeu : filtre à icônes seules (site 12)
**Objectif** : la page Jeu garde son filtre compact.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `<QUALIF>/admin` avec un quiz chargé | Barre de filtre catégorie en **icônes seules** (pas de libellé texte à côté) ; survol = infobulle avec le nom | | |
| 2 | Cliquer une icône puis une autre | Les questions se filtrent sur l'union des catégories | | |
| 3 | Cliquer **×** | Filtres retirés, toutes les questions de retour | | |
| 4 | Filtrer sur une catégorie sans question | Message « Aucune question dans cette categorie. » | | |
| 5 | Charger un quiz avec une manche RAFALE classique, puis un quiz avec une carte MEMOTION | Les deux restent filtrables par leurs catégories | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 7 — Modale IA Quiz : catégories cibles (site 13)
**Objectif** : la sélection multiple de catégories de la modale IA fonctionne et reste validée.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `<QUALIF>/admin/quiz`, lancer « Générer des questions » (IA) | « Catégories cibles » avec le badge **plusieurs possibles** | | |
| 2 | Cocher deux catégories | ✓ sur chacune | | |
| 3 | Décocher les deux, tenter de lancer la génération | Refus : « au moins une catégorie » requis | | |
| 4 | Vérifier les autres chips de la modale (public, langue, taille de lot) | Inchangés (autre domaine, non migrés) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 8 — Modale IA RAFALE : compteur du réservoir (sites 14, 15)
**Objectif** : le compteur du réservoir reste affiché dans chaque chip et suit les difficultés.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `<QUALIF>/admin/quiz?tab=rafale`, lancer « Générer des questions RAFALE » | « Catégories cibles » (badge **plusieurs possibles**) et « Difficultés » (badge **plusieurs possibles**) | | |
| 2 | Observer chaque chip de catégorie | Un compteur du réservoir dans la chip ; **0 en rouge** pour une catégorie vide | | |
| 3 | Cocher puis décocher des difficultés | Les compteurs évoluent avec les difficultés cochées | | |
| 4 | Cocher catégories et difficultés, observer la matrice « existant → après » | Valeurs identiques à celles de la version précédente pour la même sélection | | |
| 5 | Lancer une génération (si clé IA) | Les questions créées respectent la sélection | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 9 — #197 : aucun doublon avec une catégorie personnalisée, sur les 15 sites — `critical`
**Objectif** : la cause racine du bug #197 (liste brute de l'API = catégories codées en miroir + custom) ne réapparaît nulle part.
**Prérequis** : au moins une catégorie personnalisée existe (créée au scénario 1). Console ouverte : aucun avertissement « same key ».

Pour chaque ligne, compter les pastilles : **8 catégories standard + chaque catégorie personnalisée, chacune une seule fois**, aucune pastille identique en double.

| Etape | Site à balayer | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|----------------|-----------------|----------------|------|
| 1 | Quiz — éditeur de question (site 1) | Catégorie personnalisée présente une seule fois | | |
| 2 | Quiz — manche RAFALE, catégories (site 2) | Idem | | |
| 3 | Quiz — carte MEMOTION RAFALE, catégories tirées (site 4) | Idem | | |
| 4 | Quiz — filtre de la liste (site 7) | Idem (seulement les catégories présentes dans la liste) | | |
| 5 | Réservoir — formulaire (site 8) | Idem | | |
| 6 | Réservoir — filtre (site 10) | Idem | | |
| 7 | Page Jeu — filtre (site 12) | Idem | | |
| 8 | Modale IA Quiz (site 13) | Idem | | |
| 9 | Modale IA RAFALE (site 14) | Idem, un seul compteur par chip | | |
| 10 | Console du navigateur | Aucune erreur ni avertissement de clé dupliquée sur tout le balayage | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 10 — Accessibilité : badge de mode et navigation clavier
**Objectif** : le mode est lisible sans la couleur et les sélecteurs se pilotent au clavier.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Passer en revue les 15 sites (sans cliquer) | Chaque sélecteur affiche son badge en toutes lettres : « une seule », « plusieurs possibles » ou « une seule ou aucune » — même sans distinguer les couleurs | | |
| 2 | Sélection unique (catégorie du scénario 1) : naviguer avec Tab puis Espace ou Entrée | Le focus est visible ; Espace/Entrée sélectionne la catégorie focalisée | | |
| 3 | Sélection multiple (catégories d'une manche RAFALE) : Tab puis Espace/Entrée sur plusieurs chips | Chaque chip se coche/décoche avec ✓ ; le focus reste visible | | |
| 4 | Lecteur d'écran (facultatif) | Les sélections uniques sont annoncées comme des boutons radio (coché / non coché), les multiples comme des boutons activés / non activés | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 11 — Thème sombre de l'éditeur de carte MEMOTION
**Objectif** : les chips restent lisibles sur le fond sombre de l'éditeur de carte.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir une carte MEMOTION de type RAFALE (éditeur à fond sombre) | Les chips des trois sélecteurs (valeur, catégories tirées, difficultés tirées) sont lisibles : texte clair, bordures visibles | | |
| 2 | Cocher des catégories et des difficultés | Bordure à la couleur de la catégorie, ✓ visible, étoiles **ambre** sur les difficultés actives | | |
| 3 | Comparer avec la maquette (variante sombre) | Rendu conforme ; aucun texte gris sur gris, aucune chip « invisible » | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] Les 15 sites affichent leur badge de mode et se comportent comme décrit (scénarios 1 à 8)
- [ ] **Scénario 2 (`critical`)** : chaîne RAFALE intacte, y compris une manche créée avant la migration
- [ ] **Scénario 3 (`critical`)** : « Valeur de la carte : » conservé, distinct des difficultés tirées
- [ ] **Scénario 9 (`critical`)** : aucun doublon avec une catégorie personnalisée, console sans clé dupliquée
- [ ] Aucune sélection existante (manche, carte, question du réservoir) ne change de contenu après rechargement et ré-enregistrement
- [ ] Le `QuizMetaForm` (Facile…Expert, publics, langue) est visuellement et fonctionnellement inchangé
- [ ] Aucune régression sur la page Jeu, l'anim ni la TV (lecture seule)

## Notes QA

[Espace pour observations]
