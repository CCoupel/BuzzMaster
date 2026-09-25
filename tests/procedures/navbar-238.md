# Procédure de Test — Navbar : groupe Préparation, seuils responsive, badge (#238)

**Version** : 11.1.0.x (build QUALIF)
**Date** : 2026-09-25
**Testeur** : Utilisateur (validation manuelle — navigateur réel requis)
**Issue** : #238 (inclut #241) — **Plan** : `_work/handoff/plan-239-v7-20260925-160000.md` (parties B et C)
**Maquette** : https://claude.ai/artifact/QEcZsSbxZc6fNWnYisPqVS (version 16)

## Prérequis

- [ ] Environnement : QUALIF (binaire Windows), `http://localhost/admin`
- [ ] Chrome ET Edge (Windows)
- [ ] Outils développeur (F12, mode responsive) ou fenêtre redimensionnée aux largeurs voulues
- [ ] Éclairage Hue configuré si possible (pour voir le bouton Éclairage) ; sinon noter « non testable »
- [ ] Idéalement quelques joueurs/buzzers connectés pour voir les compteurs

## Tableau des paliers attendus (plan C.4)

| Largeur | Préparation | Reste de la barre | « Connecte » |
|---|---|---|---|
| ≥ 1815 | dépliée avec libellés (Joueurs, Quiz, Backstage) + bouton « 🖥️ Interface ▾ » avec libellé | tout avec libellés, titre vertical JEU | texte |
| 1710–1814 | dépliée avec libellés ; Interface ▾ en icône | titre JEU masqué | texte |
| 1500–1709 | dépliée en **icônes seules** + Interface ▾ en icône | Jeu garde ses libellés | texte |
| 1390–1499 | **menu unique** « 🛠️ Préparation ▾ » (icône), Interface = section du menu | Jeu avec libellés | texte |
| 1100–1389 | menu unique en icône | liens Jeu en icône seule | texte |
| 950–1099 | menu unique en icône | espacements compacts, logo réduit ; ENTRACTE/Éclairage avec texte | **pastille seule** |
| 810–949 | menu unique en icône | ENTRACTE 🍿 / Éclairage en icône seule | pastille seule |
| < 810 | menu unique en icône | 5 compteurs → badge 👥 | pastille seule |

## Scénarios

### Scénario 1 — Aspect aux 5 largeurs de référence (1920 / 1600 / 1280 / 1024 / 768)

**Objectif** : la barre correspond au tableau ci-dessus et à la maquette v16, sur **une seule rangée**.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | 1920 px : ouvrir `/admin/scoreboard` | Joueurs/Quiz/Backstage visibles avec libellés, bouton « Interface ▾ » avec libellé, titre vertical JEU, « Connecte » en texte | | |
| 2 | 1920 px : vérifier la hauteur | Barre sur **une seule rangée** (~70 px), aucun retour à la ligne | | |
| 3 | 1600 px | Joueurs/Quiz/Backstage en **icônes** (libellé en infobulle), Interface ▾ en icône, liens Jeu avec libellés | | |
| 4 | 1280 px | Un seul bouton « 🛠️ Préparation ▾ » en icône, liens Jeu en icône seule | | |
| 5 | 1024 px | Compact, logo réduit, ENTRACTE et Éclairage avec texte, pastille de connexion **sans texte** | | |
| 6 | 768 px | ENTRACTE/Éclairage en icône seule, compteurs remplacés par le badge 👥 | | |
| 7 | À chaque largeur : la pastille de connexion (point coloré) | **Toujours visible** (vert connecté / orange connexion / rouge déconnecté) | | |
| 8 | À chaque largeur : défilement horizontal de la barre ? | **Aucun** défilement horizontal, aucun élément coupé | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 2 — Bornes des paliers (transitions)

**Objectif** : aucun débordement ni pastille perdue à la frontière de chaque palier.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Régler successivement 1815, 1814, 1710, 1709, 1500, 1499, 1390, 1389 px | À chaque valeur : une rangée, pastille présente, pas de défilement horizontal, présentation conforme au tableau | | |
| 2 | Régler 1100, 1099, 950, 949, 810, 809 px | Idem | | |
| 3 | Régler 620 et 400 px | Une rangée, pastille présente, tout reste utilisable | | |
| 4 | Redimensionner lentement de 1920 à 400 px puis retour | Transitions fluides, pas d'élément qui saute sur une 2e ligne, pas de menu resté « coincé » | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 3 — Menu unique « Préparation » (< 1500 px, ex. 1280)

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Survoler le bouton 🛠️ | Le menu s'ouvre : 👥 Joueurs, ❓ Quiz, 🎭 Backstage, puis en-tête **INTERFACE**, puis 📺 TV ↗, 📱 Joueur ↗, 🎤 Animateur ↗ | | |
| 2 | Éloigner la souris du menu | Il se referme après un très court délai (~150 ms) ; pas de fermeture parasite en descendant vers la liste en diagonale | | |
| 3 | Cliquer sur le bouton | Le menu s'ouvre ; un second clic le referme | | |
| 4 | Menu ouvert : Échap | Se ferme | | |
| 5 | Menu ouvert : clic dans une zone vide | Se ferme | | |
| 6 | Cliquer « Quiz » | Page Quiz ouverte dans le même onglet, menu fermé, bouton Préparation **mis en évidence** | | |
| 7 | Cliquer « TV », « Joueur », « Animateur » | Chacun s'ouvre dans un **nouvel onglet** (`/tv`, `/player`, `/anim`) | | |
| 8 | Sur une page hors groupe (ex. Scores) | Le bouton Préparation n'est pas mis en évidence | | |
| 9 | Clavier : Tab jusqu'au bouton, Entrée / Espace | Ouvre/ferme ; les entrées sont atteignables au clavier, Échap ferme | | |
| 10 | Tactile (mode tactile F12) : tap sur le bouton, tap ailleurs | Ouvre puis ferme, sans dépendre du survol | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 4 — Préparation en ligne + bouton Interface (≥ 1500 px, ex. 1920 puis 1600)

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | 1920 px : cliquer « Joueurs », « Quiz », « Backstage » | Navigation dans le même onglet, lien actif mis en évidence | | |
| 2 | Survoler / cliquer « Interface ▾ » | Liste TV ↗, Joueur ↗, Animateur ↗ (nouveaux onglets) | | |
| 3 | Fermer : sortie de souris, Échap, clic extérieur | Se ferme dans les trois cas | | |
| 4 | 1600 px : survoler une icône de Préparation | Infobulle avec le libellé (Joueurs, Quiz, Backstage) | | |
| 5 | Pas de titre vertical « PRÉPARATION » | Effectivement absent (barre restée sur une rangée) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 5 — Un seul menu ouvert à la fois

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir le menu du logo, puis Préparation | Le menu du logo se ferme | | |
| 2 | Ouvrir Préparation, puis le menu du logo | Préparation se ferme | | |
| 3 | Ouvrir le popover Éclairage (si configuré), puis Préparation | Le popover se ferme | | |
| 4 | À 1920 : ouvrir Interface, puis le menu du logo | Interface se ferme | | |
| 5 | À 768 : ouvrir le badge 👥, puis Préparation | Le badge se referme | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 6 — Menu du logo : « Réglages »

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Cliquer sur le logo | Entrées dans l'ordre : ⚙️ **Réglages**, Ambiance, Backup/Restaure, Mises à jour, Logs, Quitter | | |
| 2 | Cliquer « Réglages » | Page `/admin/settings` (identique à l'ancienne « Config ») | | |
| 3 | Chercher « Config » | Plus aucune entrée nommée « Config » dans la Navbar | | |
| 4 | Ambiance, Backup, Mises à jour, Logs | Ouvrent leur page comme avant | | |
| 5 | « Quitter » : cliquer puis **Annuler** | Confirmation affichée, menu fermé, serveur toujours actif (ne PAS confirmer) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 7 — ENTRACTE 🍿 / 🎬 et Éclairage

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | À 1280 px, partie arrêtée | Bouton « 🍿 ENTRACTE » (libellé visible) | | |
| 2 | Cliquer | Passe à « 🎬 FIN D'ENTRACTE », entracte actif | | |
| 3 | Cliquer à nouveau | Retour à 🍿 ENTRACTE | | |
| 4 | Réduire sous 950 px | Icône seule (🍿 / 🎬) avec infobulle ; toujours cliquable | | |
| 5 | Éclairage (si configuré) : sous 950 px | Icône 💡 seule avec infobulle ; le popover s'ouvre au clic | | |
| 6 | Pendant une question en cours | Bouton ENTRACTE désactivé avec l'infobulle habituelle | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 8 — Badge compteurs 👥 (< 810 px)

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | À 768 px avec des joueurs/buzzers connectés | Badge « 👥 connectés/participants » (VJoueurs + Buzzers) ; les 5 compteurs ne sont plus affichés | | |
| 2 | Survoler le badge | Détail : les 5 compteurs avec libellés (admin, TV, animateur, VJoueurs X/Y, Buzzers X/Y) | | |
| 3 | Cliquer le badge (tactile) | Le détail s'ouvre ; second clic/Échap/clic extérieur le ferme | | |
| 4 | Débrancher un buzzer / déconnecter un VJoueur | Badge passe à orange puis rouge (sévérité la plus grave) | | |
| 5 | Repasser à ≥ 810 px | Retour aux 5 compteurs, badge absent | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 9 — Non-régression et navigateurs

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Groupe Jeu : Jeu, Scores, Palmarès, Historique | Mêmes pages qu'avant, lien actif surligné | | |
| 2 | Badge version | Clic → `/admin/updates`, comme avant | | |
| 3 | Logo A1 (#239) | Inchangé, menu s'ouvre/ferme comme avant | | |
| 4 | `/tv`, `/anim`, `/player`, `/` | Inchangés (pas de Navbar admin) | | |
| 5 | Rejouer scénarios 1 à 3 sous Chrome puis Edge | PASS dans les deux | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Criteres de Validation

- [ ] Une seule rangée et pastille visibles à toutes les largeurs testées, aux bornes comprises
- [ ] Aucun défilement horizontal de la Navbar
- [ ] Préparation : en ligne ≥ 1500, menu unique < 1500 ; survol ET clic/tap ; Échap, clic extérieur, navigation ferment
- [ ] Un seul menu ouvert à la fois
- [ ] « Réglages » remplace « Config » (même route)
- [ ] ENTRACTE 🍿/🎬, Éclairage, badge 👥 conformes
- [ ] Chrome et Edge OK

## Notes QA

[Espace pour observations]
