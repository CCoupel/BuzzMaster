# Procedure de Test — Éditeur de questions ROUE (phrases à deviner)

**Version** : 12.0.0 (LOT-1.1)
**Date** : 2026-11-16
**Testeur** : QA
**Issue** : #145 — Phrases à deviner : type ROUE et éditeur
**Références** : `contracts/roue.md` §3, §3.1, §9, §11 ; maquette `docs/mockup/v12.0.0/ui/admin_question_editor__roue_phrase.html`

> Périmètre LOT-1.1 : **création, modification, aperçu et confidentialité** des questions ROUE.
> Le jeu lui-même (roue, tours, gains) arrive avec LOT-1.2 (#144) : une question ROUE **ne peut pas encore démarrer**.

## Prerequis

- [ ] Environnement : LOCAL ou QUALIF, serveur BuzzControl v12.0.0 démarré
- [ ] Données : au moins une catégorie existante (ex. « Animaux »)
- [ ] Accès : interface admin (`/questions`), navigateur récent ; écran TV (`/tv`) ouvert dans un 2ᵉ onglet

## Scenarios

### Scenario 1 — Créer une question ROUE avec la phrase d'exemple et juger l'aperçu du plateau

**Validation** : HUMAIN — **Issues** : #145

**Objectif** : Vérifier que l'éditeur ressemble à la maquette M3 et que le plateau est lisible.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `/questions`, cliquer le type **Roue** (🎡, dernier de la liste) | Le formulaire affiche : Phrase à deviner, Catégorie, Phrase de présentation, Mode de jeu, Gains de la manche, Forme de la roue, Lettres déjà proposées. **Aucun** champ Réponse / Points / Temps | | |
| 2 | Saisir « Combien pour ce chien dans la vitrine » | Sous le champ : « ✓ 3 lignes sur 4 · 12 consonnes différentes · 17 lettres différentes » | | |
| 3 | Regarder le plateau d'aperçu | Grille 4 × 14 ; lignes centrées `COMBIEN POUR` / `CE CHIEN DANS` / `LA VITRINE` ; voyelles teintées différemment des consonnes ; légende présente | | |
| 4 | Comparer à la maquette M3 (cas « Phrase valide ») | Disposition, libellés et couleurs conformes | | |
| 5 | Choisir la catégorie « Animaux », saisir la présentation « Chanson de Line Renaud » | Compteur « 22 / 80 » | | |
| 6 | Cliquer **Ajouter** | La question apparaît dans la liste, type Roue ; le formulaire revient à l'état « nouvelle question » | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 2 — Phrase refusée : trop longue, mot trop long, caractère interdit, vide

**Validation** : AUTO — **Issues** : #145
*(équivalent automatisé : `internal/game/roue_board_145_test.go`, `internal/server/roue_upload_145_test.go`, `RoueEditor.145.test.jsx`, `QuestionsPage.roue.145.test.jsx`)*

**Objectif** : Vérifier qu'une phrase qui ne tient pas ne peut pas être enregistrée et que le refus est lisible.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Saisir « Il ne faut pas vendre la peau de l'ours avant de l'avoir tué » | Message rouge « Phrase trop longue : … 4 lignes au plus » ; bouton **Ajouter** désactivé | | |
| 2 | Saisir un mot de 15 lettres (`ABCDEFGHIJKLMNO`) | Refus « un mot dépasse 14 caractères » ; **Ajouter** désactivé | | |
| 3 | Saisir `mon_chien` puis `mon*chien` | Refus « caractère non admis » ; **Ajouter** désactivé | | |
| 4 | Vider le champ | Pas de message d'erreur bloquant, **Ajouter** désactivé | | |
| 5 | Saisir `Été à Noël !` | Accepté : plateau `ETE A NOEL !` (accents repliés, majuscules) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 3 — Limites exactes du plateau (14 colonnes, 4 lignes)

**Validation** : AUTO — **Issues** : #145
*(équivalent automatisé : `TestRoueBoard145_Layout_Boundaries`, `TestRoueUpload145_BoundaryPhrasesAccepted`)*

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Saisir `ABCDEFGHIJKLMN` (14 lettres) | Accepté, 1 ligne pleine | | |
| 2 | Saisir `AAAAAAA BBBBBB` (7 + espace + 6 = 14) | Accepté, 1 ligne | | |
| 3 | Saisir `AAAAAAA BBBBBBB` (15) | Accepté, passe sur 2 lignes (mot non coupé) | | |
| 4 | Saisir quatre mots de 14 lettres séparés par des espaces | Accepté, 4 lignes pleines | | |
| 5 | Ajouter un cinquième mot de 14 lettres | Refusé : trop long (5 lignes) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 4 — Les réglages sont enregistrés puis retrouvés à la modification

**Validation** : HUMAIN — **Issues** : #145

**Objectif** : Vérifier qu'aucun des six champs n'est perdu à l'enregistrement ni à la réouverture (leçon #107).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Créer une question : mode **CHACUN SON TOUR**, gains **Tout le monde encaisse**, forme **Arc**, interrupteur « lettres déjà proposées » **désactivé**, présentation renseignée | Enregistrée | | |
| 2 | Ouvrir cette question en modification | Les six valeurs sont retrouvées à l'identique (phrase, présentation, mode, gains, forme, interrupteur) | | |
| 3 | Créer une autre question en laissant les défauts | Défauts : **TANT QUE JE GAGNE**, **Seul le gagnant encaisse**, **Bandeau**, interrupteur **activé** | | |
| 4 | Rouvrir la 2ᵉ question | Mêmes défauts | | |
| 5 | Modifier la phrase de la 1ʳᵉ question avec une phrase invalide puis tenter d'enregistrer | Impossible ; la question d'origine est inchangée dans la liste | | |
| 6 | Vérifier les modes proposés | Exactement 3 : SOLO, CHACUN SON TOUR, TANT QUE JE GAGNE (pas de « Maillon faible ») | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 5 — La phrase reste secrète (TV, tablettes) — critical

**Validation** : AUTO — **Issues** : #145
*(équivalent automatisé : `internal/protocol/roue_leak_test.go`, `internal/server/roue_leak_routing_145_test.go`, `internal/vplayer/roue_leak_fanout_145_test.go`)*

**Objectif** : S'assurer que la phrase n'est jamais lisible côté TV ni tablette. Le jeu ne démarre pas encore : on vérifie le garde-fou et ce que l'écran TV reçoit.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `/tv` ; dans les outils développeur, onglet Réseau/WS, suivre les messages de `/ws/tv` | — | | |
| 2 | Sélectionner la question ROUE créée au scénario 1 comme question courante (préparation de partie) | La partie ne passe pas à « Prêt » : une question ROUE ne peut pas démarrer avant la v12.0 LOT-1.2 | | |
| 3 | Rechercher « vitrine » et `ROUE_PHRASE` dans les messages reçus par `/ws/tv` | Aucune occurrence | | |
| 4 | Idem sur une tablette (`/player`) | Aucune occurrence | | |
| 5 | Vérifier la liste des questions côté admin | La phrase est visible côté admin (éditeur) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 6 — Non-régression des autres types

**Validation** : AUTO — **Issues** : #145
*(équivalent automatisé : `TestRoueUpload145_NonRoueTypeNeverCarriesRoueFields`, `TestRoueLeak145_NonRegression_OtherQuestionsUnchangedOnTV`, `questionTypeMeta.roue.145.test.js`)*

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Créer une question SPEEDY, QCM, RAFALE | Enregistrées normalement, formulaires inchangés | | |
| 2 | Ouvrir « Générer par IA » | Le type ROUE n'est pas proposé | | |
| 3 | Vérifier une question existante d'un autre type | Aucun champ `ROUE_*` dans son éditeur | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Criteres de Validation

- [ ] L'éditeur ROUE est conforme à la maquette M3
- [ ] Toute phrase hors règle est refusée avec un message lisible, rien n'est enregistré
- [ ] Les six champs ROUE_* survivent à l'enregistrement et à la réouverture
- [ ] La phrase ne figure dans aucun message TV / tablette
- [ ] Aucune régression sur les autres types de question

## Notes QA

[Espace pour observations]
