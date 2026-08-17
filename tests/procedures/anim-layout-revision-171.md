# Procédure de Test — Révision de mise en page `/anim` (#171)

**Version** : v6.2.x (branche `feature/anim-question-display`)
**Date** : 2026-08-16
**Testeur** : QA
**Issue** : #171 — `#ID` promu titre, L5 ancré en bas, crédit universel
**Référence** : Plan `_work/reports/plan-20260816-192400.md` (révision 2), maquette
https://claude.ai/code/artifact/f0d94785-5f20-4397-a538-435bcc4600b2

---

## Prérequis

- [ ] Environnement : QUALIF ou LOCAL
- [ ] `/anim` sur tablette (ou navigateur), `/admin` et `/tv` sur postes séparés pour la
      non-régression du chronomètre
- [ ] Un quiz avec au moins une question SPEEDY, une QCM et une ARDOISE
- [ ] Au moins une équipe qui ne buzz/répond pas, pour vérifier "0 pt" sans tentative
- [ ] Deux résolutions à couvrir : **1280×800** et **1024×768**, plus un contrôle en **1024×600**

---

## Scénario 1 — Lisibilité de la ligne méta (une manche par mode)

**Objectif** : Vérifier l'ordre, la taille et l'absence de doublon sur `#ID`.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Charger une question SPEEDY, observer la ligne méta à distance de lecture normale (bras tendu) | Ordre lisible : progression (n/total) · catégorie · type · **`#ID`** · options — tous à la même taille que le bouton "à suivre" | | |
| 2 | Vérifier `#ID` | Apparaît **une seule fois** sur toute la page, à la même taille que les autres éléments (pas de petit texte en retrait ailleurs) | | |
| 3 | Vérifier les points de la question | Toujours ancrés à droite de la ligne | | |
| 4 | Vérifier le statut de connexion | Toujours présent, à sa taille habituelle (pas agrandi comme les 5 autres éléments) | | |
| 5 | Répéter pour une question QCM puis ARDOISE | Même ordre, même lisibilité | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Statut de partie sur la ligne réponse

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Charger une question, observer la colonne chronomètre | Le chronomètre affiche les chiffres et la barre, **sans** pastille de statut (ARRET/PAUSE/EN COURS/...) | | |
| 2 | Observer la ligne juste au-dessus de la zone réponse | La pastille de statut y est affichée, juste avant la zone réponse | | |
| 3 | Dérouler les phases (PREPARE → READY → STARTED → PAUSE → STOP → REVEALED) | La pastille change de libellé/couleur en cohérence avec la phase à chaque étape | | |
| 4 | Ouvrir `/admin` sur la même partie | Le chronomètre `/admin` garde SA pastille de statut intégrée, comme avant #171 | | |
| 5 | Ouvrir `/tv` sur la même partie | Idem — chronomètre `/tv` inchangé | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Conduite : contenu et gain de hauteur

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Charger une question SPEEDY | L2 affiche un texte réservé nommant le mode ; L3 affiche un emplacement réservé générique (pas de grille) | | |
| 2 | Charger une question QCM | L3 affiche la grille des 4 propositions ; L2 reste réservée au mode | | |
| 3 | Comparer la hauteur de conduite disponible avec avant #171 (si connu) | Gain de hauteur perceptible (L5 ne pousse plus le contenu, structure ancrée) | | |
| 4 | Observer "à suivre" | Toujours fonctionnel dans ses trois états (vert/bleu/inerte), format inchangé | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Ancrage de "à suivre" (position identique, tous modes/questions)

**Objectif** : Vérifier visuellement que L5 ne bouge jamais, quel que soit le contenu du bloc
central.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Noter la position verticale (en pixels ou repère visuel) de "à suivre" sur une question SPEEDY (bloc central minimal) | Position notée | | |
| 2 | Charger une question QCM (bloc central plus chargé : grille + indices) | Position de "à suivre" **identique** à l'étape 1 | | |
| 3 | Charger une question ARDOISE | Position de "à suivre" **identique** | | |
| 4 | Passer d'une question à l'autre plusieurs fois de suite | Aucun sursaut visuel de "à suivre" à aucun moment | | |
| 5 | Vérifier que "à suivre" reste juste au-dessus de la bande régie | Toujours vrai, quel que soit le mode | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Tenue sans scroll (1280×800, 1024×768, 1024×600)

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | En 1280×800, charger une question QCM avec indices activés (cas le plus haut) | Toute la page tient, `/anim` reste en `overflow: hidden`, aucune barre de défilement sur la PAGE | | |
| 2 | Répéter en 1024×768 | Idem | | |
| 3 | Répéter en 1024×600 (barres navigateur) | La page tient toujours ; si le contenu central (L2/L3/L4) déborde, **c'est lui qui défile en interne** — jamais L1 ni "à suivre" qui disparaissent de l'écran | | |
| 4 | Vérifier les cibles tactiles des boutons L1 et "à suivre" | Toutes ≥ 62 px, dans les trois résolutions | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — Crédit universel (SPEEDY, QCM, ARDOISE)

**Objectif** : Vérifier que "0 pt" est systématiquement proposé, et que le verrouillage suit
toujours le crédit réel plutôt que la tentative.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Manche SPEEDY, laisser une équipe NE PAS buzzer | Une fois arrêtée : cette équipe voit "0 pt" (pas de "+N pts"), avec un motif discret ("pas de buzz"), **le bouton "0 pt" est aligné avec ceux des autres équipes** | | |
| 2 | Créditer/refuser les autres équipes normalement | Comportement inchangé (#156/#157) | | |
| 3 | Manche QCM, laisser une équipe ne pas répondre | Même comportement : "0 pt" seul, motif "pas de réponse" | | |
| 4 | Manche ARDOISE | Liste ARDOISE inchangée (#158) — "0 pt" toujours proposé aux copies vides comme avant | | |
| 5 | Depuis `/admin`, créditer une équipe SPEEDY qui n'a jamais buzzé (la régie peut le faire librement) | Sur `/anim`, cette équipe apparaît **verrouillée** avec le montant crédité, malgré l'absence de tentative | | |
| 6 | Vérifier l'alignement visuel de tous les gestes de crédit dans la colonne équipes | Tous à la même position, avec ou sans médaille/temps de réaction affichés à côté | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 7 — Carte d'équipe : médaille

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Manche SPEEDY, observer l'équipe la plus rapide en STARTED | Médaille (🏆) affichée **avant** le nom de l'équipe | | |
| 2 | Comparer la position du bouton de crédit entre l'équipe médaillée et une équipe sans médaille | **Position identique** (aligné à droite dans les deux cas) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 8 — Non-régression `/admin` et `/tv`

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Dérouler une manche complète sur `/admin` (SPEEDY, QCM, ARDOISE) | Comportement strictement inchangé par #171 | | |
| 2 | Observer `/tv` sur la même manche | Comportement strictement inchangé | | |
| 3 | `go build ./...` (si applicable), `npm test` | Tous les tests PASS | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation Globale

- [ ] Ligne méta lisible à bout de bras, `#ID` unique, statut connexion non agrandi (Scénario 1)
- [ ] Pastille de statut sur la ligne réponse, `/admin`/`/tv` non régressés (Scénario 2)
- [ ] Contenu L2/L3 correct par mode, gain de hauteur perçu (Scénario 3)
- [ ] "À suivre" toujours à la même position, tous modes/questions confondus (Scénario 4)
- [ ] Aucun débordement de page en 1280×800/1024×768/1024×600 (Scénario 5)
- [ ] "0 pt" toujours proposé, verrouillage indépendant de la tentative (Scénario 6)
- [ ] Médaille avant le nom, gestes alignés (Scénario 7)
- [ ] Aucune régression `/admin`/`/tv` (Scénario 8)

---

## Notes QA

[Espace pour observations]
