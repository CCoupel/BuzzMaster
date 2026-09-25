# Procédure de Test — Nouveau logo BuzzControl (#239)

**Version** : 11.1.0.x (build QUALIF)
**Date** : 2026-09-25
**Testeur** : Utilisateur (validation manuelle — navigateur réel requis)
**Issue** : #239 — logo A1 remplaçant l'abeille du bouton de menu de la Navbar
**Plan** : `_work/handoff/plan-239-v7-20260925-160000.md` (partie A)

## Prérequis

- [ ] Environnement : QUALIF (binaire Windows `buzzcontrol-v11.1.0.x-windows-amd64.exe`)
- [ ] Serveur démarré, navigateur sur `http://localhost/admin`
- [ ] Navigateurs : Chrome ET Edge (Windows)
- [ ] Maquette de référence : artboard « #239 — logo A1, menu ouvert » (https://claude.ai/artifact/QEcZsSbxZc6fNWnYisPqVS)
- [ ] Outils développeur (F12 → mode responsive) pour les largeurs 768 et 400 px

> **Note testeur** : le défilement horizontal de la Navbar sous ~2100 px de large est **antérieur à #239**
> (traité par #238/#241). Ce n'est **pas** un défaut de #239 — ne pas le reporter comme échec.

## Ce qui change / ne change pas

- Change : le bouton de menu affiche le mot-symbole « Buzz » / « Control » (incliné) + ⚡ + ▼ au lieu de 🐝 ; le texte « BuzzControl » à côté disparaît ; la rotation de l'abeille disparaît.
- Ne change pas : entrées du menu, ancrage, fermeture, libellé accessible « Menu de navigation », badge version, effet de survol (léger agrandissement).

## Scénarios

### Scénario 1 — Rendu du logo à 1920 et 1280 px (AC1, AC4, AC7)

**Objectif** : le logo est conforme à la maquette, lisible sur fond blanc.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `/admin/quiz` en fenêtre 1920 px de large | Navbar affichée, bouton de menu en haut à gauche | | |
| 2 | Observer le bouton de menu | « Buzz » (indigo foncé) au-dessus, « Control » (rose) plus petit, décalé à droite et incliné vers le haut, ⚡ dans le coin haut-droit, ▼ à côté | | |
| 3 | Comparer à l'artboard « A1 — menu ouvert » | Rendu conforme (police arrondie Fredoka, couleurs, inclinaison) | | |
| 4 | Vérifier le fond | Logo bien lisible sur le fond blanc de la Navbar, aucune couleur illisible | | |
| 5 | Vérifier l'absence de l'ancien contenu | Plus d'abeille 🐝, plus de texte « BuzzControl » à côté du bouton | | |
| 6 | Vérifier le badge version | Le badge `vX.Y.Z` suit directement le bouton, sans chevauchement avec « Control » ni ⚡ | | |
| 7 | Passer la fenêtre à 1280 px | Même rendu, même conformité, pas de chevauchement | | |
| 8 | Vérifier que la police est bien chargée | Aucun rendu en police système de secours (lettres arrondies grasses) ; onglet Réseau (F12) : aucune requête vers un domaine externe (ex. Google Fonts) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 2 — Menu : ouverture/fermeture inchangées (AC2, AC9, AC10)

**Objectif** : le menu se comporte exactement comme avant.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Cliquer sur le logo | Le menu déroulant s'ouvre, mêmes entrées qu'avant (Config, Logs, Quitter, etc.), ancré sous le bouton | | |
| 2 | Cliquer à nouveau sur le logo | Le menu se ferme | | |
| 3 | Rouvrir le menu, cliquer dans une zone vide de la page | Le menu se ferme (clic extérieur) | | |
| 4 | Rouvrir le menu, cliquer sur une entrée de navigation | La page change et le menu se ferme | | |
| 5 | Clavier : Tab jusqu'au bouton de menu | Focus visible sur le bouton | | |
| 6 | Appuyer sur Entrée (puis Espace) | Le menu s'ouvre / se ferme | | |
| 7 | Survoler le bouton | Léger agrandissement (scale) et fond gris clair, comme avant | | |
| 8 | Observer le logo 10 secondes sans interaction | **Aucune rotation / animation** (l'abeille oscillait avant) | | |
| 9 | Survoler le bouton : infobulle | Infobulle « Menu » | | |
| 10 | (Optionnel) Lecteur d'écran / inspecteur d'accessibilité | Le bouton est annoncé « Menu de navigation » ; le contenu du logo n'est pas lu séparément | | |
| 11 | Cliquer sur le badge version | Navigation vers `/admin/updates` comme avant | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 3 — Petits formats ≤ 768 px (AC6, AC7)

**Objectif** : le logo réduit reste visible et cliquable.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Passer la fenêtre (ou mode responsive) à 768 px | Logo visible, **plus petit** qu'à 1280 px | | |
| 2 | Cliquer sur le logo | Le menu s'ouvre normalement, entrées lisibles | | |
| 3 | Vérifier ⚡ et « Control » | Restent dans la zone du bouton, pas de chevauchement avec le badge version | | |
| 4 | Passer à 400 px | Logo toujours visible et cliquable, menu utilisable | | |
| 5 | Ouvrir puis fermer le menu à 400 px | Comportement identique à 1280 px | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 4 — Non-régression des autres pages (hors périmètre)

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `/tv` | Affichage inchangé (aucun logo A1) | | |
| 2 | Ouvrir `/anim` | Inchangé | | |
| 3 | Ouvrir `/player` | Inchangé | | |
| 4 | Ouvrir `/` | Inchangé | | |
| 5 | Ouvrir `/admin/game`, `/admin/players`, `/admin/settings` | Navbar avec logo A1, reste de la barre inchangé (paliers, ENTRACTE, Éclairage, compteurs) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 5 — Chrome et Edge (Windows)

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Rejouer les scénarios 1 à 3 sous Chrome | PASS | | |
| 2 | Rejouer les scénarios 1 à 3 sous Edge | PASS | | |
| 3 | Observer le rendu de ⚡ | Emoji affiché (l'aspect peut varier selon l'OS — toléré) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Criteres de Validation

- [ ] Logo A1 conforme à la maquette à 1920 et 1280 px, lisible sur fond blanc
- [ ] Menu s'ouvre et se ferme comme avant (clic, clic extérieur, navigation, clavier)
- [ ] Aucune animation de rotation ; hover conservé
- [ ] « BuzzControl » textuel et 🐝 absents
- [ ] Logo réduit cliquable à 768 et 400 px, sans chevauchement du badge version
- [ ] `/tv`, `/anim`, `/player`, `/` inchangés
- [ ] Chrome et Edge OK
- [ ] Le défilement horizontal préexistant n'est pas compté comme échec

## Notes QA

[Espace pour observations]
