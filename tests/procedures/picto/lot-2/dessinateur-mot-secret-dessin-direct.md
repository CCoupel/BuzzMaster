# Procedure de Test — Picto : dessinateur, mot secret, dessin en direct (LOT-2.1, #192 #193)

**Version** : 13.0.0.x (QUALIF)
**Date** : 2026-11-23
**Testeur** : utilisateur (téléphone réel + TV réelle) — QA ne rejoue pas ces scénarios (pas de navigateur fiable)
**Références** : `contracts/picto.md` §6.1 à §6.3, §6.7, §8, §10, §15 — maquettes `docs/mockup/v13.0.0/` (M1 anim, M2 admin, M3 TV, M4 tablettes, M8, M9)

## Prerequis

- [ ] Environnement : QUALIF, serveur Windows lancé
- [ ] Matériel : **1 vrai téléphone** (VJoueur, dessinateur), **1 TV réelle** (`/tv`), 1 PC admin (`/`), 1 tablette animateur (`/anim`), au moins 2 autres VJoueurs (téléphone ou onglets)
- [ ] Données : équipes « Rouges » (≥ 2 joueurs VJoueur connectés) et « Bleus » (≥ 1) ; une question Picto de **3 mots** (durée 90 s) et une question Picto à **1 mot**
- [ ] Rien d'autre ne tourne sur la TV : écran statique, sans scroll

## Scenarios

### Scenario 1 — Préparation : type et catégorie affichés (retour GATE LOT-1)

**Validation** : HUMAIN — **Issues** : #192

**Objectif** : en préparation / READY, la question Picto affiche son type et sa catégorie comme les autres types, jamais « Nouvelle question ».

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Admin : sélectionner la question Picto | En-tête de la page Jeu : « ✏️ Picto » + la catégorie | | |
| 2 | Regarder le bandeau haut de la TV | Type et catégorie visibles (indice), pas « Nouvelle question » | | |
| 3 | Regarder la tablette animateur | Même information | | |
| 4 | Passer en READY | L'affichage ne régresse pas | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 2 — Choisir l'équipe et le dessinateur (choix direct puis tirage)

**Validation** : HUMAIN — **Issues** : #192

**Objectif** : l'animateur ou l'admin désigne l'équipe et le dessinateur ; seuls les joueurs connectés sont proposés.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Choisir l'équipe « Rouges » (sur l'anim, puis vérifier sur l'admin) | L'équipe est la même sur les deux écrans ; les joueurs de « Rouges » sont listés | | |
| 2 | Couper le Wi-Fi d'un joueur Rouge | Il apparaît non sélectionnable (déconnecté) | | |
| 3 | Choisir directement un joueur connecté | Il est désigné ; son téléphone affiche « C'est toi qui dessines ! » **sans le mot** | | |
| 4 | Cliquer « tirage au hasard » 5 fois | À chaque fois un joueur **connecté de Rouges** est désigné ; le téléphone concerné change ; jamais un joueur déconnecté | | |
| 5 | Tirer sans aucun joueur connecté dans l'équipe | Message clair (aucun joueur connecté), aucun dessinateur désigné | | |
| 6 | Changer d'équipe pour « Bleus » | Le dessinateur de « Rouges » est effacé | | |
| 7 | Sans dessinateur, regarder les boutons « Lancer » (admin et anim) | Grisés tant qu'il n'y a pas de dessinateur connecté ; un `START` forcé est refusé avec un message clair | | |
| 8 | Écrans des autres joueurs | Équipe qui dessine : « Devinez ! … regardez la TV » ; autres équipes : « *Équipe* devine le dessin de *Nom* » ; aucune feuille | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 3 — Le mot reste secret (téléphone du dessinateur, admin, anim seulement)

**Validation** : HUMAIN — **Issues** : #192

**Objectif** : le mot n'apparaît que sur le téléphone du dessinateur, l'admin et l'anim — jamais sur la TV ni les autres téléphones.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Question de 3 mots : désigner un dessinateur, lancer | Décompte puis la manche démarre | | |
| 2 | Téléphone du dessinateur | Le **mot en grand** + la feuille carrée pleine largeur, **aucun bouton** | | |
| 3 | Admin et animateur | Ils voient le mot de la manche | | |
| 4 | TV | Aucun mot ; indice (catégorie) et « ✏️ *Nom* dessine pour *Équipe* » ; la feuille | | |
| 5 | Autres téléphones | Aucun mot, aucune feuille | | |
| 6 | Dans le navigateur de la TV, ouvrir les outils de développement (onglet Réseau/WS) pendant une manche | Aucune trame ne contient le mot ni les autres mots de la question | | |
| 7 | Relancer plusieurs manches (question à 3 mots) | Tirage libre : le mot peut changer ou se répéter ; question à 1 mot : toujours le même mot | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 4 — Dessin en direct sur la TV, doigt réel

**Validation** : HUMAIN — **Issues** : #193

**Objectif** : ce que le dessinateur trace au doigt apparaît en direct sur la TV, sans saccade excessive, trait noir sur fond blanc.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Tracer un trait continu long, des courbes, un point isolé (appui bref) | Le trait suit le doigt sur la TV avec un retard faible (< ~1/4 s), extrémités arrondies ; le point isolé est un disque | | |
| 2 | Dessiner vite pendant 30 s | La TV reste fluide ; aucun morceau de trait manquant | | |
| 3 | Aucun bouton « effacer » sur le téléphone | La feuille ne s'efface jamais, le dessinateur peut continuer par-dessus | | |
| 4 | Faire pivoter le téléphone (portrait puis paysage) | La feuille reste carrée, le dessin est conservé | | |
| 5 | Regarder admin et anim | Ils voient le même dessin en direct | | |
| 6 | Les autres téléphones | Ne reçoivent jamais la feuille | | |
| 7 | TV : vérifier l'écran statique | Pas de scroll, chronomètre commun (mm:ss + barre) à sa place habituelle, feuille carrée centrée | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 5 — TV rechargée pendant le dessin

**Validation** : HUMAIN — **Issues** : #193

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Pendant une manche avec un dessin déjà avancé, recharger la page de la TV (F5) | La TV réaffiche **tout le dessin déjà tracé** après quelques instants | | |
| 2 | Continuer à dessiner | Les nouveaux traits s'ajoutent sans doublon ni trou | | |
| 3 | Recharger aussi l'admin puis l'anim | Idem (feuille complète + mot sur l'admin/l'anim) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 6 — Le dessinateur décroche puis revient

**Validation** : HUMAIN — **Issues** : #192 #193

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | En manche, couper le Wi-Fi du téléphone dessinateur | La manche **continue** (chronomètre inchangé) ; l'admin et l'anim voient l'alerte « *Nom* est déconnecté(e) — … » | | |
| 2 | Rétablir le Wi-Fi, rouvrir la page | Le téléphone retrouve **le mot et sa feuille** avec les traits déjà tracés | | |
| 3 | Continuer à dessiner | La TV reçoit les nouveaux traits, le dessin précédent est conservé | | |
| 4 | Avant le lancement (READY), déconnecter le dessinateur | « Lancer » redevient grisé ; un nouveau choix ou un tirage règle le cas | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 7 — Pause et buzzers

**Validation** : HUMAIN — **Issues** : #192 #193

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Admin : PAUSE pendant le dessin | Chronomètre suspendu ; feuille du dessinateur **verrouillée** (« Jeu en pause ») ; les traits ne partent plus | | |
| 2 | Reprendre | Le dessin reprend, rien n'a été perdu | | |
| 3 | Appuyer sur un buzzer physique (ou le bouton d'un VJoueur) pendant la manche | Aucun effet, aucun message d'erreur ; LED : équipe qui dessine en plein, autres équipes atténuées | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Criteres de Validation

- [ ] Le mot n'a jamais été vu sur la TV ni sur un téléphone autre que le dessinateur
- [ ] Le dessin apparaît en direct sur TV, admin et anim, jamais sur les autres téléphones
- [ ] Un dessinateur déconnecté retrouve mot et feuille
- [ ] Aucun scroll sur la TV
- [ ] Aucune régression sur les autres types (ouvrir une question QCM et une question Roue, les lancer)

## Notes QA

Les règles (garde d'identité, époque, SEQ, tampon plein, fuite) sont couvertes en automatique :
`server-go/cmd/server/picto_wire_192_193_test.go`, `picto_leak_fanout_192_test.go`, `internal/server/picto_relay_193_test.go`.
Mesure de charge : `tests/perf/picto-load.md`.
