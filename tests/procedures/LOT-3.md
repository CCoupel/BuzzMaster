# Procédure de Test — LOT-3 v13.1.0 : type « Rafale Picto » (#318)

**Version** : v13.1.0.x (QUALIF)
**Date** : 2026-11-25
**Testeur** : QA (scénarios `AUTO`) ; **l'utilisateur** pour les scénarios `HUMAIN` (manche complète avec TV et tablettes réelles)
**Contrats** : `contracts/picto.md` §17 (type, champs, moteur §17.3 à §17.8, confidentialité §17.9, actions §17.10), `contracts/picto-pool.md` rév. 3, `contracts/award-preselect.md`
**Maquettes** : `_work/mockup/v13.1.0/ui/` (M3 éditeur, M4 TV, M5 tablettes, M6 anim, M7 admin), `conception/game_state_picto_rafale__words.html` (M8)
**Tests automatiques** : `tests/INDEX.md` (lignes #318)

> **Cycle 5 (13.1.0.8)** : les scénarios 1 à 11 ont été validés OK en 13.1.0.7 ; à revérifier sur les points modifiés (éditeur, fil des mots, zone de jeu admin).
> Sauvegarder `data/` avant. Prévoir un Pool Picto de 10 mots au moins, 3 équipes avec joueurs, une tablette de dessinateur, TV, `/admin` et `/anim` ouverts.

## Prérequis

- [ ] QUALIF, Pool Picto non vide, 3 équipes
- [ ] TV et tablettes réelles disponibles

---

## Scénario 0 — Non-régression et règles automatiques (rappel QA)

**Validation** : AUTO — **Issues** : #318

**Objectif** : EPOCH / STALE_WORD, limite de passes, rebouclage, plancher 0, fuite par mot, famille Picto, PICTO_PASS d'une tablette rejeté.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `go test ./cmd/server -run TestRafalePicto318` ; `go test ./internal/game -run 'TestRafalePicto_|TestIsPictoFamily'` ; `go test ./internal/server -run TestRafalePictoUpload` | Verts | | |
| 2 | `npx vitest run` sur `*.318.test.*` | Verts | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 1 — Création du type Rafale Picto dans l'éditeur

**Validation** : HUMAIN — **Issues** : #318

**Objectif** : Création du type Rafale Picto dans l'éditeur.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Page Quiz : nouvelle question, choisir « 🖍️ Rafale Picto » ; observer la grille, le sous-éditeur « Mots à dessiner » (source Pool / Mots saisis, filtre, alerte) et le bloc Passes (coût, 1 passe / illimitées, fin de pool) ; enregistrer puis rouvrir ; essayer coût 101, mot invalide, source Pool sans catégorie | Type « Rafale Picto » 🖍️ (Picto reste « Picto » ✏️) ; défauts : source Pool Picto, coût 0, passes illimitées, fin de pool = reboucler ; points = par mot trouvé ; refus lisibles (coût > 100, mot invalide, filtre vide) ; enregistrement fidèle des 7 champs ; une question Picto v13.0.0 n'est pas modifiée ; cycle 5 : catégories et difficultés en sélection multiple comme Rafale Speedy, un seul sélecteur de catégories (jamais en double), « Ajouter » grisé tant qu'aucune catégorie n'est choisie, crédit = 1re catégorie du filtre, cartes Mots / Passes / Temps et points empilées (Picto un mot garde ses 2 colonnes et son double sélecteur, inchangé) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Préparation, avertissement de pool et lancement

**Validation** : HUMAIN — **Issues** : #318

**Objectif** : Préparation, avertissement de pool et lancement.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Question Rafale Picto source Pool (peu de mots disponibles) : choisir l'équipe et le dessinateur, observer l'avertissement sur /admin et /anim, lancer le décompte puis la manche | Avertissement « N mots disponibles, environ M nécessaires » (M = durée/10 arrondi au-dessus) ; READY refusé si aucun mot disponible (POOL_EMPTY) ; sans dessinateur, START refusé ; au départ le dessinateur reçoit le 1er mot sur sa tablette, l'admin et l'anim aussi, la TV jamais | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Manche complète : Trouvé par l'animateur et par l'admin

**Validation** : HUMAIN — **Issues** : #318

**Objectif** : Manche complète : Trouvé par l'animateur et par l'admin.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer une manche (TIME 120). Le dessinateur dessine ; l'équipe devine à voix haute ; l'animateur appuie sur « Trouvé » (anim), puis l'admin sur « Trouvé » (admin) pour le mot suivant ; essayer d'appuyer en même temps sur les deux | Chaque « Trouvé » donne le mot suivant dans la même action, la feuille repart vierge (nouvelle époque), le compteur grandit sur la TV, l'anim et l'admin ; deux appuis simultanés n'avancent que d'un mot (l'autre est ignoré avec un message) ; le fil des mots montre le mot en cours et les mots trouvés, jamais de mots à venir ; aucune action « Trouvé » ni « Passer » sur les tablettes ; fil des mots (admin et anim, cycle 5) : rangée compacte (retour à la ligne), fond de la couleur de l'équipe qui joue, mot en cours en gras et plus grand, mots passés barrés, jamais de mot à venir ; zone de jeu de l'admin plus grande (cycle 5) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Passer : coût et limite de passes

**Validation** : HUMAIN — **Issues** : #318

**Objectif** : Passer : coût et limite de passes.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Question avec coût 5 et « 1 passe » : le dessinateur annonce « je passe », l'animateur appuie sur « Passer » ; essayer une seconde passe. Refaire avec « illimitées » | Passer retire des points (coût), le mot suivant arrive, « passes restantes » baisse ; avec 1 passe, le bouton est grisé après la première (message « plus de passe » si on force) ; en illimité, jamais grisé ; le mot passé n'est jamais montré aux équipes ni à la TV ; « Passer » absent en pause | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Fin de pool : reboucler ou finir

**Validation** : HUMAIN — **Issues** : #318

**Objectif** : Fin de pool : reboucler ou finir.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Question avec 3 mots saisis : (a) fin de pool = reboucler : trouver 1 mot, passer les 2 autres ; (b) fin de pool = fin de manche : trouver les 3 mots | (a) le plus ancien mot passé revient (FIFO), jamais un mot trouvé, la manche continue ; (b) à la fin des mots la manche s'arrête avec l'issue « fin de pool » et le son de fin de lot, le récapitulatif reste affiché | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — Écran TV : mot trouvé 1 s, compteur, passe

**Validation** : HUMAIN — **Issues** : #318

**Objectif** : Écran TV : mot trouvé 1 s, compteur, passe.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Regarder la TV pendant une manche : « Trouvé » sur un mot avec le dessin tracé, puis « Passer » | Au « Trouvé » le mot s'affiche 1 s en surimpression (« ✅ mot ») sur le dessin, sans « Trouvé par » ni points, puis feuille vierge ; les traits du mot suivant tracés pendant cette seconde ne sont pas perdus ; le grand compteur s'incrémente ; une passe vide la feuille avec un repère « ⏭ » discret, jamais le mot ; aucune animation de points à chaque mot ; TV sans défilement ; fil des mots sur l'écran de jeu (cycle 5) : rangée compacte (retour à la ligne), fond de la couleur de l'équipe qui joue, mot en cours en gras et plus grand, mots passés barrés, jamais de mot à venir | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 7 — Tablettes : dessinateur sans bouton, équipes

**Validation** : HUMAIN — **Issues** : #318

**Objectif** : Tablettes : dessinateur sans bouton, équipes.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Observer la tablette du dessinateur, celle d'un joueur de l'équipe qui devine et celle d'une autre équipe ; tenter de passer ou valider depuis une tablette | Dessinateur : le mot en grand, « Ton mot · catégorie », compteur de mots trouvés, AUCUN bouton Passer/Trouvé ; équipe qui devine : « Devinez ! » avec mots trouvés et passes ; autres équipes : l'équipe qui devine et le compteur seulement ; aucune action n'est possible depuis une tablette | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 8 — Fin de manche et crédit pré-rempli

**Validation** : HUMAIN — **Issues** : #318

**Objectif** : Fin de manche et crédit pré-rempli.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Laisser le temps s'écouler (ou STOP), révéler, observer la page Jeu admin et l'anim ; refaire avec beaucoup de passes à coût élevé ; valider le crédit | Crédit manuel pré-rempli à (trouvés × points − passes × coût) avec plancher 0, ajustable ; l'équipe qui dessinait est présélectionnée sur l'admin ; une manche à 0 point ne coche rien ; l'animation des points ne joue qu'au crédit ; les mots passés ne sont jamais publiés, le récapitulatif des trouvés est public | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 9 — Trouvé sur le gong

**Validation** : HUMAIN — **Issues** : #318

**Objectif** : Trouvé sur le gong.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Laisser le chrono finir au moment où l'équipe vient de trouver ; appuyer sur « Trouvé » après le gong | Le mot en cours est compté une seule fois puis la manche est révélée dans la même action ; un second appui ne recompte pas | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 10 — Relance et déconnexion du dessinateur

**Validation** : HUMAIN — **Issues** : #318

**Objectif** : Relance et déconnexion du dessinateur.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Relancer la même question après la fin ; en manche, fermer la tablette du dessinateur puis la rouvrir ; recharger /admin pendant la manche | Relance : compteurs, passes et mots trouvés remis à zéro, équipe et dessinateur conservés, limite de passes rendue ; dessinateur déconnecté : la manche continue, l'admin et l'anim jouent Trouvé / Passer, alerte « déconnecté(e) » ; à la reconnexion (tablette ou admin) le mot en cours et la feuille sont renvoyés ; fil des mots (admin et anim, cycle 5) : rangée compacte (retour à la ligne), fond de la couleur de l'équipe qui joue, mot en cours en gras et plus grand, mots passés barrés, jamais de mot à venir | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 11 — Non-régression : Picto un mot, Rafale Speedy, libellés

**Validation** : HUMAIN — **Issues** : #318

**Objectif** : Non-régression : Picto un mot, Rafale Speedy, libellés.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Jouer une question Picto v13.0.0 (mots saisis), une question Picto source Pool, puis une manche Rafale Speedy ; vérifier les libellés dans le sélecteur de type, la page Jeu, l'anim et la TV | Picto un mot : Trouvé arrête et révèle comme avant, pas de compteur ni de passes ; Rafale Speedy inchangée ; libellés « Picto », « Rafale Picto », « Rafale Speedy » corrects partout | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Criteres de Validation

- [ ] Scénario 0 PASS
- [ ] Scénarios 1 à 11 PASS (manche complète sur TV et tablettes réelles)
- [ ] Les questions Picto v13.0.0 et Rafale Speedy se comportent comme avant

## Notes QA

