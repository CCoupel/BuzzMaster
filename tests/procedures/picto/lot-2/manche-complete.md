# Procedure de Test — Picto : manche complète, quatre issues, relance (LOT-2.3, #194)

**Version** : 13.0.0.x (QUALIF)
**Date** : 2026-11-23
**Testeur** : utilisateur (téléphone réel + TV réelle) — QA ne rejoue pas ces scénarios (pas de navigateur fiable)
**Références** : `contracts/picto.md` §6.3 à §6.7, §7, §15 — maquettes `docs/mockup/v13.0.0/` (M1 anim, M2 admin, M3 TV, M4 tablettes, M8 manche)
**Complète** : `dessinateur-mot-secret-dessin-direct.md` (LOT-2.1 : préparation, mot secret, dessin en direct)

## Prerequis

- [ ] Environnement : QUALIF, serveur Windows lancé
- [ ] Matériel : **1 vrai téléphone** (dessinateur), 1 ou 2 autres VJoueurs, **1 TV réelle** (`/tv`), PC admin, tablette animateur (`/anim`), idéalement 1 buzzer physique
- [ ] Données : équipes « Rouges » (≥ 2 VJoueurs connectés) et « Bleus » ; une question Picto de **3 mots** (durée 90 s) et une question Picto à **1 mot** ; points de la question : 10
- [ ] Rien d'autre sur la TV : écran statique, sans scroll

## Scenarios

### Scenario 1 — Issue « Trouvé » (manche gagnée) et crédit des points

**Validation** : HUMAIN — **Issues** : #194

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Choisir l'équipe « Rouges », un dessinateur, lancer ; le dessinateur trace pendant ~20 s | Dessin en direct sur la TV ; le mot n'apparaît que sur le téléphone du dessinateur, l'admin et l'anim | | |
| 2 | L'équipe devine **à l'oral** ; l'animateur appuie sur « Trouvé ! » | La manche s'arrête **et** la réponse est révélée en une seule fois, **sans aucun point attribué** : le chronomètre s'arrête, la TV montre le dessin final et **le mot seul** en zone réponse | | |
| 3 | TV | **Aucun** « Trouvé par … », aucun « +10 pts », aucun nom d'équipe dans la zone réponse : le mot, comme la réponse des autres types | | |
| 4 | Téléphones (dessinateur et autres) | Dessinateur : « Mot révélé », le mot et sa feuille finale ; les autres : « Mot révélé » et le mot ; aucun « Trouvé par … » ni points | | |
| 5 | Anim / admin | Issue « ✅ Trouvé » **sans le nom de l'équipe** ; ligne de crédit « +10 pts » **pré-remplie** pour « Rouges » seulement (« 0 pt » pour les autres équipes) | | |
| 6 | Valider le crédit « +10 pts » | **L'animation commune des points** se joue sur la TV (équipe « Rouges », +10), et sur les **seuls** téléphones de « Rouges » ; la ligne se verrouille ; score de l'équipe +10 ; aucun score individuel pour le dessinateur | | |
| 7 | Regarder les téléphones des autres équipes | Aucune animation de points | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 2 — Issue « Temps écoulé », puis « Trouvé » sur le gong

**Validation** : HUMAIN — **Issues** : #194

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer une manche et laisser le chronomètre aller à zéro (ou régler une durée courte) | Son et éclairage « temps écoulé » habituels ; la manche s'arrête ; **le mot n'est pas encore affiché** sur la TV | | |
| 2 | Écrans animateur / admin | Proposent « Trouvé ! » et la révélation | | |
| 3 | « Trouvé ! » (l'équipe a répondu sur le gong) | Révélation immédiate, issue « Trouvé », crédit pré-rempli à 10 pts | | |
| 4 | Refaire un temps écoulé, cette fois « Révéler » sans « Trouvé » | Mot révélé (mot seul sur la TV), **aucun crédit** proposé (seulement « 0 pt » côté anim) | | |
| 4b | Valider « 0 pt » | **Aucune animation** sur la TV ni les téléphones, aucun texte de points ; la ligne se verrouille sur « 0 pt » | | |
| 5 | Après la révélation, essayer « Trouvé ! » | Plus proposé / refusé | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 3 — Issue « Arrêt » (abandon) et issue « sans mot »

**Validation** : HUMAIN — **Issues** : #194

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Pendant une manche, l'admin appuie sur **Stop** | Manche arrêtée, mot **pas encore public** ; « Trouvé ! » n'est plus proposé (abandon) | | |
| 2 | « Révéler » | Mot affiché sur la TV, aucun crédit proposé | | |
| 3 | Alternative : relancer la question (scénario 5) | Possible depuis l'arrêt | | |
| 4 | Question « sans mot » : éditer une question Picto, vider la liste de mots pendant que la question est en READY (deux onglets admin), puis lancer | La manche s'arrête immédiatement, message clair « sans mot », aucune erreur bloquante ; « Révéler » affiche une réponse vide | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 4 — Buzzers et appuis ignorés, éclairage des équipes

**Validation** : HUMAIN — **Issues** : #194

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Dès que l'équipe est choisie (préparation) : regarder les buzzers physiques | Ceux de l'équipe qui dessine : couleur d'équipe **pleine** ; ceux des autres équipes : **atténués** | | |
| 2 | En manche, appuyer sur un buzzer physique de chaque équipe, et sur le bouton d'un VJoueur | **Aucun effet**, aucun message d'erreur, aucune pause | | |
| 3 | Pause admin | Tous les buzzers atténués ; reprise : l'équipe qui dessine retrouve son éclairage plein | | |
| 4 | Éclairage d'ambiance (si une ampoule est configurée) | Scène « tour d'équipe » aux couleurs de l'équipe qui dessine | | |
| 5 | Sons | Seulement les bruitages existants (décompte, temps écoulé, gain au crédit), aucun nouveau son | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 5 — Relance de la question (nouveau tirage, feuille vierge)

**Validation** : HUMAIN — **Issues** : #194

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Après une manche (révélée ou arrêtée), l'admin sélectionne de nouveau la **même question** | Retour en préparation ; l'équipe et le dessinateur sont **conservés** ; issue, mot révélé et crédit remis à zéro | | |
| 2 | Téléphone du dessinateur | Retour à « C'est toi qui dessines ! » **sans le mot** | | |
| 3 | Changer le dessinateur (choix direct, puis tirage au hasard), puis lancer | Nouvelle manche, **feuille vierge** sur TV et téléphone | | |
| 4 | Question à **3 mots** : relancer 5 fois | Le mot est tiré librement : il peut changer ou **se répéter** | | |
| 5 | Question à **1 mot** : relancer | C'est toujours **le même mot** | | |
| 6 | Passer à la question suivante (autre question) | Équipe, dessinateur et feuille remis à zéro : on repart de la préparation | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 6 — Reconnexions : dessinateur, TV, admin, anim

**Validation** : HUMAIN — **Issues** : #194

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | En manche, couper le Wi-Fi du dessinateur 20 s | La manche **continue** ; admin et anim voient « *Nom* est déconnecté(e) — elle retrouvera sa feuille en revenant ; sinon, relancez la question » ; **aucun remplacement** n'est proposé en manche | | |
| 2 | Rétablir le Wi-Fi | Le téléphone retrouve **mot et feuille** ; il peut continuer | | |
| 3 | Recharger la TV (F5) en plein dessin | Le dessin déjà tracé réapparaît ; la TV ne montre jamais le mot avant la révélation | | |
| 4 | Recharger l'admin puis l'anim | Feuille complète + mot de la manche | | |
| 5 | Supprimer le bumper du dessinateur depuis l'admin, hors manche | Le dessinateur est effacé de la préparation, l'équipe est conservée ; un nouveau choix est possible | | |
| 6 | Si le dessinateur reste déconnecté, relancer la question | Retour en préparation, nouveau choix/tirage parmi les joueurs connectés | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Criteres de Validation

- [ ] Les quatre issues se terminent proprement (Trouvé, temps écoulé, arrêt, sans mot)
- [ ] « Trouvé » sur le gong fonctionne ; plus après la révélation
- [ ] Le mot n'est public qu'à la révélation, et uniquement le mot de la manche
- [ ] Crédit manuel pré-rempli aux points de la question, équipe seule
- [ ] Relance : feuille vierge, tirage libre, équipe et dessinateur conservés
- [ ] Aucun scroll sur la TV ; aucune régression sur les autres types (lancer un QCM, une Roue, une Rafale)

## Notes QA

Automatique : `server-go/cmd/server/picto_round_194_test.go` (issues, crédit, relance, déconnexions, buzzers),
`server-go/internal/led/picto_led_194_test.go` (éclairage des équipes). Mesure de charge : `tests/perf/picto-load.md`.
