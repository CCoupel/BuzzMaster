# Procédure de Test — Scrollbar parasite sur les pages admin (#177)

**Version** : v6.4.x (branche `feature/anim-communication`)
**Date** : 2026-08-18
**Testeur** : QA / Utilisateur
**Issue** : #177 — scrollbar verticale permanente sur les pages `/admin/*`, effet de bord de #167
**Référence** : Plan `_work/reports/plan-20260818-174801.md`. Complète la procédure du milestone
`tests/procedures/anim-communication-167-168.md` (#167/#168/#175/#176) — même environnement, mêmes
prérequis de session.

> ⚠️ **Ceci est une recette purement visuelle.** L'absence de scrollbar ne peut pas être vérifiée par
> un test automatique : jsdom (le moteur des tests Vitest) ne fait aucun calcul de mise en page réel.
> La suite automatisée (`RegieMessageBar.test.jsx`, bloc "#177, T1") vérifie uniquement le
> **mécanisme** (la variable CSS `--regie-bar-h` posée/mise à jour/nettoyée), jamais le **résultat**
> visuel. Ce document est donc le seul endroit où #177 est réellement validé.

---

## Prérequis

- [ ] Environnement : QUALIF ou LOCAL, `feature/anim-communication` (ou la branche qui l'a mergée)
- [ ] Un navigateur redimensionnable librement (desktop) — pas besoin d'un appareil tactile pour ce
      document (voir `anim-communication-167-168.md` Scénario 13 pour le tactile)
- [ ] Un compte `/admin` avec au moins quelques questions, équipes et un peu d'historique — pour que
      les pages « Questions »/« Équipes »/« Historique » ne soient pas vides (une page vide masque
      certains cas de débordement)
- [ ] Suite automatisée verte avant de démarrer (voir section Non-Régression en fin de document)

---

## Ce qu'il faut savoir avant de commencer

Le bandeau régie (`/admin/*`, bas d'écran) réserve désormais un espace **mesuré dynamiquement**
(`--regie-bar-h`), au lieu d'une constante figée à 44 px. Sa hauteur varie selon :
- **la largeur de la fenêtre** (le contenu du bandeau retourne à la ligne sur fenêtre étroite),
- **l'état du message** (champ seul ≈ 44 px ; champ + « Effacer » + indicateur « Vu par l'animateur »
  peuvent pousser la hauteur au-delà, surtout en fenêtre étroite).

Chaque page ci-dessous doit s'adapter à cette hauteur **quelle qu'elle soit** — c'est tout l'objet de
la correction : une seule mesure réelle, partagée par toutes les pages, au lieu de huit constantes en
dur qui pouvaient dériver.

---

## Scénario 1 — Les huit pages à hauteur fixe/minimale, fenêtre large, sans message actif (AC1)

**Objectif** : Le cas nominal — aucune scrollbar ne doit apparaître sur une page dont le contenu tient
naturellement dans le viewport.

Pour **chacune** des huit pages ci-dessous, avec la fenêtre du navigateur **maximisée** (large) et
**aucun message régie actif** :

| # | Page | Aucune scrollbar verticale ? | Notes |
|---|------|:---:|---|
| 1 | `/admin` (GamePage) | [ ] | Le pire cas du lot (`height` fixe + `overflow: hidden`) |
| 2 | `/admin/quiz` (Questions) | [ ] | |
| 3 | `/admin/teams` (Équipes) | [ ] | |
| 4 | `/admin/settings` (Config) | [ ] | |
| 5 | `/admin/backup` (Backup/Restaure) | [ ] | |
| 6 | `/admin/scoreboard` (Scores) | [ ] | |
| 7 | `/admin/history` (Historique) | [ ] | |
| 8 | `/admin/palmares` (Palmarès catégorie) | [ ] | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Les huit pages, fenêtre ÉTROITE, sans message actif (AC1, AC5)

**Objectif** : Vérifier que le redimensionnement ne fait pas apparaître de scrollbar transitoire ni
permanente, même quand la fenêtre est réduite (le bandeau lui-même peut changer de hauteur sur une
largeur réduite selon son propre contenu).

Rétrécir la fenêtre à une largeur d'environ 500-600 px (ou utiliser le mode responsive du navigateur),
puis répéter le tableau du Scénario 1 :

| # | Page | Aucune scrollbar verticale ? | Notes |
|---|------|:---:|---|
| 1 | `/admin` (GamePage) | [ ] | |
| 2 | `/admin/quiz` (Questions) | [ ] | |
| 3 | `/admin/teams` (Équipes) | [ ] | |
| 4 | `/admin/settings` (Config) | [ ] | |
| 5 | `/admin/backup` (Backup/Restaure) | [ ] | |
| 6 | `/admin/scoreboard` (Scores) | [ ] | |
| 7 | `/admin/history` (Historique) | [ ] | |
| 8 | `/admin/palmares` (Palmarès catégorie) | [ ] | |

Puis, **en gardant la fenêtre étroite** :

| Étape | Action | Résultat Attendu | OK ? |
|-------|--------|-----------------|------|
| 9 | Redimensionner LENTEMENT la fenêtre (large → étroite → large) sur `/admin` | Aucune scrollbar n'apparaît de façon transitoire pendant le redimensionnement (AC5) | [ ] |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Les huit pages, AVEC un message régie actif (AC1, AC4)

**Objectif** : Le cas qui a introduit le bug — le bandeau occupe plus que ses 44 px nominaux une fois
un message actif affiché (texte + « Effacer » + éventuellement l'indicateur d'acquittement).

| Étape | Action | Résultat Attendu | OK ? |
|-------|--------|-----------------|------|
| 1 | Depuis `/admin`, envoyer une consigne régie (fenêtre large) | Le bandeau affiche le message + « Effacer » | [ ] |

Puis, **le message restant actif**, répéter le tableau du Scénario 1 sur les huit pages :

| # | Page | Aucune scrollbar verticale ? | Le bandeau ne recouvre PAS le bas du contenu ? |
|---|------|:---:|:---:|
| 1 | `/admin` (GamePage) | [ ] | [ ] |
| 2 | `/admin/quiz` (Questions) | [ ] | [ ] |
| 3 | `/admin/teams` (Équipes) | [ ] | [ ] |
| 4 | `/admin/settings` (Config) | [ ] | [ ] |
| 5 | `/admin/backup` (Backup/Restaure) | [ ] | [ ] |
| 6 | `/admin/scoreboard` (Scores) | [ ] | [ ] |
| 7 | `/admin/history` (Historique) | [ ] | [ ] |
| 8 | `/admin/palmares` (Palmarès catégorie) | [ ] | [ ] |

Puis, **rétrécir la fenêtre** (message toujours actif — le bandeau devrait grandir davantage, retour à
la ligne) et répéter à nouveau :

| # | Page | Aucune scrollbar verticale ? | Le bandeau ne recouvre PAS le bas du contenu ? |
|---|------|:---:|:---:|
| 1 | `/admin` (GamePage) | [ ] | [ ] |
| 2 | `/admin/quiz` (Questions) | [ ] | [ ] |
| 3 | `/admin/teams` (Équipes) | [ ] | [ ] |
| 4 | `/admin/settings` (Config) | [ ] | [ ] |
| 5 | `/admin/backup` (Backup/Restaure) | [ ] | [ ] |
| 6 | `/admin/scoreboard` (Scores) | [ ] | [ ] |
| 7 | `/admin/history` (Historique) | [ ] | [ ] |
| 8 | `/admin/palmares` (Palmarès catégorie) | [ ] | [ ] |

Enfin, effacer le message (« Effacer ») et vérifier que la réservation redevient normale (44 px) sans
saut visuel brutal ni scrollbar résiduelle.

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — `/admin/logs` et `/admin/updates` (AC2)

**Objectif** : Ces deux pages n'utilisent pas la constante de 120 px corrigée par #177 — vérifier
qu'elles n'ont pas la scrollbar permanente (elles ne devraient pas l'avoir avant #177 non plus) et
qu'elles ne sont pas recouvertes par le bandeau quand il grandit.

| Étape | Action | Résultat Attendu | OK ? |
|-------|--------|-----------------|------|
| 1 | Ouvrir `/admin/logs`, fenêtre large, sans message actif | Pas de scrollbar de page (le défilement des logs, s'il existe, reste interne à la liste) | [ ] |
| 2 | Envoyer un message régie actif, observer `/admin/logs` | Le bas de la liste de logs reste visible — non recouvert par le bandeau | [ ] |
| 3 | Répéter en fenêtre étroite | Même constat | [ ] |
| 4 | Ouvrir `/admin/updates`, mêmes vérifications (large/étroite, avec/sans message) | Pas de scrollbar parasite, pas de recouvrement du bas de page par le bandeau | [ ] |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Contenu qui dépasse réellement le viewport (AC3)

**Objectif** : #177 ne doit PAS supprimer le défilement légitime d'une page dont le contenu est
vraiment plus long que l'écran — seulement la scrollbar parasite d'une page qui, elle, devrait tenir.

| Étape | Action | Résultat Attendu | OK ? |
|-------|--------|-----------------|------|
| 1 | Sur `/admin/quiz`, avec un quiz de nombreuses questions (ou réduire la fenêtre en hauteur pour forcer le débordement) | La page défile normalement | [ ] |
| 2 | Faire défiler jusqu'en bas | Le **dernier élément** de la liste est entièrement visible **au-dessus** du bandeau régie — pas caché dessous | [ ] |
| 3 | Envoyer un message régie actif pendant que la page est en bas de son défilement | Le dernier élément reste visible au-dessus du bandeau (qui vient de grandir) — vérifier qu'il n'est pas repoussé sous le bandeau sans que la page ne s'ajuste | [ ] |
| 4 | Répéter sur `/admin/history` (historique long) et `/admin/backup` si le contenu le permet | Même constat | [ ] |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — Pages plein écran strictement inchangées (AC6)

**Objectif** : #177 ne doit avoir **aucun** effet sur les pages hors `/admin/*` — elles ne montent
jamais le bandeau régie.

| Étape | Action | Résultat Attendu | OK ? |
|-------|--------|-----------------|------|
| 1 | Ouvrir `/tv` (pendant qu'un message régie est actif sur un autre onglet `/admin`) | Aucune trace du bandeau, aucun changement de mise en page, affichage **strictement statique** (contrainte projet — jamais de scroll sur `/tv`) | [ ] |
| 2 | Ouvrir `/player` (VJoueur) | Aucune trace du bandeau, mise en page inchangée | [ ] |
| 3 | Ouvrir `/anim` | Aucune trace du bandeau régie (bas d'écran) — seule la bande de réception du message (Scénario du milestone #167) est présente, ailleurs sur la page | [ ] |
| 4 | Ouvrir `/enroll` (ou `/`, page d'inscription) | Aucune trace du bandeau, mise en page inchangée | [ ] |
| 5 | Redimensionner la fenêtre sur chacune de ces 4 pages | Aucun changement de comportement lié au bandeau régie (il n'existe pas sur ces pages) | [ ] |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation Globale

- [ ] Aucune scrollbar sur les 8 pages admin à contenu court, fenêtre large ET étroite, avec ET sans
      message actif (Scénarios 1-3)
- [ ] `/admin/logs` et `/admin/updates` sans régression, ni scrollbar ni recouvrement (Scénario 4)
- [ ] Le défilement légitime d'un contenu long reste intact, dernier élément visible au-dessus du
      bandeau (Scénario 5)
- [ ] Les 4 pages plein écran (`/tv`, `/player`, `/anim`, `/enroll`) strictement inchangées
      (Scénario 6) — en particulier `/tv` reste statique, sans scroll

---

## Non-Régression (suite automatisée, à exécuter avant validation manuelle)

| Étape | Action | Résultat Attendu | OK ? |
|-------|--------|-----------------|------|
| 1 | `cd server-go/web && npm test -- RegieMessageBar` | PASS, y compris le bloc « mesure de hauteur via --regie-bar-h (#177, T1) » : observation au montage, mise à jour sur nouvelle mesure, arrondi, déduplication, remise à 0px au démontage, disconnect | [ ] |
| 2 | `cd server-go/web && npm test` (suite complète) | Aucune régression sur le reste de la suite (les autres tests utilisant `RegieMessageBar` héritent désormais d'un mock global `ResizeObserver`) | [ ] |
| 3 | `grep -rn "calc(100vh - 120px)" server-go/web/src/pages/` | **Aucune occurrence** (AC7) | [ ] |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Notes QA

[Espace pour observations]
