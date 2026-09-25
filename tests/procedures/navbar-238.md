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
- [ ] Poste **Windows** (police d'émojis Segoe UI Emoji) — obligatoire pour le contrôle de débordement
- [ ] Idéalement quelques joueurs/buzzers connectés pour voir les compteurs

## Tableau des paliers attendus (seuils mesurés sous Windows, v11.1.0.12)

| Largeur | Préparation | Reste de la barre | « Connecte » |
|---|---|---|---|
| ≥ 1945 | dépliée avec libellés (Joueurs, Quiz, Backstage) + bouton « 🖥️ Interface ▾ » **avec libellé** | tout avec libellés, titre vertical JEU | texte |
| 1855–1944 | dépliée avec libellés ; Interface ▾ en icône | titre JEU masqué | texte |
| 1685–1854 | dépliée en **icônes seules** + Interface ▾ en icône | Jeu garde ses libellés | texte |
| 1495–1684 | **menu unique** « 🛠️ Préparation ▾ » (icône), Interface = section du menu | Jeu avec libellés | texte |
| 1275–1494 | menu unique en icône | liens Jeu en icône seule | texte |
| 1095–1274 | menu unique en icône | espacements compacts, logo réduit ; ENTRACTE/Éclairage avec texte | **pastille seule** |
| 955–1094 | menu unique en icône | ENTRACTE 🍿 / Éclairage en icône seule | pastille seule |
| < 955 | menu unique en icône | 5 compteurs → badge 👥 | pastille seule |

Conséquences aux largeurs de référence : **1920** → titre JEU masqué et libellé « Interface » en icône ;
**1600** → menu unique « Préparation ▾ » ; **1280** → liens Jeu en icône seule ; **1024** → ENTRACTE/Éclairage en icône ;
**768** → badge 👥.

> **Important — plateforme de mesure** : la vérification du non-débordement doit se faire **sous Windows**
> (Chrome/Edge avec la police d'émojis **Segoe UI Emoji**), **jamais** avec une police de substitution
> (Chrome Linux/headless sans police emoji : les émojis y sont plus étroits et les mesures faussement optimistes).
> Largeurs minimales réelles mesurées sous Windows par palier : 1899 / 1809 / 1640 / 1450 / 1229 / 1049 / 911 / 696.

## Scénarios

### Scénario 1 — Aspect aux 5 largeurs de référence (1920 / 1600 / 1280 / 1024 / 768)

**Objectif** : la barre correspond au tableau ci-dessus et à la maquette v16, sur **une seule rangée**.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | 1920 px : ouvrir `/admin/scoreboard` | Joueurs/Quiz/Backstage visibles avec libellés, bouton « Interface ▾ » **en icône**, titre vertical JEU masqué, « Connecte » en texte | | |
| 2 | 1920 px : vérifier la hauteur | Barre sur **une seule rangée** (~70 px), aucun retour à la ligne | | |
| 3 | 1600 px | **Menu unique** « 🛠️ Préparation ▾ » (icône), liens Jeu avec libellés, « Connecte » en texte | | |
| 4 | 1280 px | Menu unique « 🛠️ Préparation ▾ », liens Jeu en icône seule | | |
| 5 | 1024 px | Compact, logo réduit, ENTRACTE et Éclairage en **icône seule**, pastille de connexion **sans texte** | | |
| 6 | 768 px | ENTRACTE/Éclairage en icône seule, compteurs remplacés par le badge 👥 | | |
| 7 | À chaque largeur : la pastille de connexion (point coloré) | **Toujours visible** (vert connecté / orange connexion / rouge déconnecté) | | |
| 8 | À chaque largeur : défilement horizontal de la barre ? | **Aucun** défilement horizontal, aucun élément coupé | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 2 — Bornes des paliers (transitions)

**Objectif** : aucun débordement ni pastille perdue à la frontière de chaque palier.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Régler successivement 1945, 1944, 1855, 1854, 1685, 1684, 1495, 1494, 1275, 1274 px | À chaque valeur : une rangée, pastille présente, pas de défilement horizontal, présentation conforme au tableau | | |
| 2 | Régler 1095, 1094, 955, 954 px | Idem | | |
| 3 | Régler 620 et 400 px | Une rangée, pastille présente, tout reste utilisable | | |
| 4 | Redimensionner lentement de 1920 à 400 px puis retour | Transitions fluides, pas d'élément qui saute sur une 2e ligne, pas de menu resté « coincé » | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 3 — Menu unique « Préparation » (< 1685 px, ex. 1600 et 1280)

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Survoler le bouton 🛠️ | Le menu s'ouvre : 👥 Joueurs, ❓ Quiz, 🎭 Backstage, puis en-tête **INTERFACE**, puis 📺 TV ↗, 📱 Joueur ↗, 🎤 Animateur ↗ | | |
| 2 | Éloigner la souris du menu | Il se referme après un très court délai (~150 ms) ; pas de fermeture parasite en descendant vers la liste en diagonale | | |
| 3 | Sortir de la zone, puis cliquer sur le bouton | Le menu s'ouvre ; un second clic le referme (après un survol, le premier clic garde le menu ouvert — comportement voulu pour le tactile) | | |
| 4 | Menu ouvert : Échap | Se ferme | | |
| 5 | Menu ouvert : clic dans une zone vide | Se ferme | | |
| 6 | Cliquer « Quiz » | Page Quiz ouverte dans le même onglet, menu fermé, bouton Préparation **mis en évidence** | | |
| 7 | Cliquer « TV », « Joueur », « Animateur » | Chacun s'ouvre dans un **nouvel onglet** (`/tv`, `/player`, `/anim`) | | |
| 8 | Sur une page hors groupe (ex. Scores) | Le bouton Préparation n'est pas mis en évidence | | |
| 9 | Clavier : Tab jusqu'au bouton, Entrée / Espace | Ouvre/ferme ; les entrées sont atteignables au clavier, Échap ferme | | |
| 10 | Tactile (mode tactile F12) : tap sur le bouton, tap ailleurs | Ouvre puis ferme, sans dépendre du survol | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 4 — Préparation en ligne + bouton Interface (≥ 1685 px, ex. 1920 puis 1700)

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | 1920 px (Préparation avec libellés) : cliquer « Joueurs », « Quiz », « Backstage » | Navigation dans le même onglet, lien actif mis en évidence | | |
| 2 | Survoler / cliquer « Interface ▾ » | Liste TV ↗, Joueur ↗, Animateur ↗ (nouveaux onglets) | | |
| 3 | Fermer : sortie de souris, Échap, clic extérieur | Se ferme dans les trois cas | | |
| 4 | 1700 px (icônes seules) : survoler une icône de Préparation | Infobulle avec le libellé (Joueurs, Quiz, Backstage) | | |
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
| 4 | Réduire sous 1095 px | Icône seule (🍿 / 🎬) avec infobulle ; toujours cliquable | | |
| 5 | Éclairage (si configuré) : sous 1095 px | Icône 💡 seule avec infobulle ; le popover s'ouvre au clic | | |
| 6 | Pendant une question en cours | Bouton ENTRACTE désactivé avec l'infobulle habituelle | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 8 — Badge compteurs 👥 (< 955 px)

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | À 768 px (ou 900 px) avec des joueurs/buzzers connectés | Badge « 👥 connectés/participants » (VJoueurs + Buzzers) ; les 5 compteurs ne sont plus affichés | | |
| 2 | Survoler le badge | Détail : les 5 compteurs avec libellés (admin, TV, animateur, VJoueurs X/Y, Buzzers X/Y) | | |
| 3 | Cliquer le badge (souris puis **tactile**, mode tactile F12) | Le détail s'ouvre et **reste ouvert** après le tap ; second clic/Échap/clic extérieur le ferme (point de vigilance : un tap émule survol puis clic, le détail ne doit pas s'ouvrir puis se refermer aussitôt) | | |
| 4 | Débrancher un buzzer / déconnecter un VJoueur | Badge passe à orange puis rouge (sévérité la plus grave) | | |
| 5 | Repasser à ≥ 955 px | Retour aux 5 compteurs, badge absent | | |

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
- [ ] Préparation : en ligne ≥ 1685, menu unique < 1685 ; survol ET clic/tap ; Échap, clic extérieur, navigation ferment
- [ ] Un seul menu ouvert à la fois
- [ ] « Réglages » remplace « Config » (même route)
- [ ] ENTRACTE 🍿/🎬, Éclairage, badge 👥 conformes
- [ ] Chrome et Edge OK, mesures faites sous Windows (Segoe UI Emoji)

## Notes QA

[Espace pour observations]
