# Procédure de Test — Scrollbar parasite sur les pages admin (#177 + #179)

**Version** : v6.4.x (branche `feature/anim-communication`)
**Date** : 2026-08-18 (révisée 2026-08-19 pour #179)
**Testeur** : QA / Utilisateur
**Issues** : #177 (scrollbar verticale permanente sur `/admin/*`, effet de bord de #167) + #179
(hauteur de la Navbar jamais mesurée, deuxième occurrence de la même faille sur `--navbar-h`)
**Référence** : Plan `_work/reports/plan-20260818-174801.md` (#177), `_work/reports/plan-20260818-212304.md`
(#179). Complète la procédure du milestone `tests/procedures/anim-communication-167-168.md`
(#167/#168/#175/#176) — même environnement, mêmes prérequis de session.

> ⚠️ **Ceci est une recette purement visuelle.** L'absence de scrollbar ne peut pas être vérifiée par
> un test automatique : jsdom (le moteur des tests Vitest) ne fait aucun calcul de mise en page réel.
> La suite automatisée (`useElementHeightVar.test.js`, `Navbar.test.jsx` bloc "#179, T2",
> `RegieMessageBar.test.jsx` bloc "#177, T1") vérifie uniquement le **mécanisme** (les variables CSS
> `--regie-bar-h` et `--navbar-h` posées/mises à jour/nettoyées), jamais le **résultat** visuel. Ce
> document est donc le seul endroit où #177 **et** #179 sont réellement validées.

---

## Prérequis

- [ ] Environnement : QUALIF ou LOCAL, `feature/anim-communication` (ou la branche qui l'a mergée)
- [ ] Un navigateur redimensionnable librement (desktop) — pas besoin d'un appareil tactile pour ce
      document (voir `anim-communication-167-168.md` Scénario 13 pour le tactile)
- [ ] Un compte `/admin` avec au moins quelques questions, équipes et un peu d'historique — pour que
      les pages « Questions »/« Équipes »/« Historique » ne soient pas vides (une page vide masque
      certains cas de débordement)
- [ ] Un panneau/outils de développement permettant de lire ou fixer précisément la largeur de la
      fenêtre (utile pour viser 768 px et 1200 px au Scénario 7)
- [ ] Suite automatisée verte avant de démarrer (voir section Non-Régression en fin de document)

---

## Ce qu'il faut savoir avant de commencer

L'espace réservé en bas (bandeau régie) **et** en haut (Navbar) des pages `/admin/*` est désormais
**mesuré dynamiquement** — plus aucune constante en dur (`--admin-chrome-h` ne contient plus ni
`120px`, ni `72px`, ni `48px`, ni `44px` fixes). Deux sources varient :

- **`--regie-bar-h`** (#177) — le bandeau régie, en bas. Varie selon la largeur de la fenêtre (le
  contenu retourne à la ligne sur fenêtre étroite) et l'état du message (champ seul ≈ 44 px ; champ +
  « Effacer » + indicateur « Vu par l'animateur » peuvent pousser la hauteur au-delà).
- **`--navbar-h`** (#179) — la Navbar, en haut. Varie **par sauts discrets** aux points de rupture
  responsive **768 px** et **1200 px** (paddings et libellés qui changent), pas en continu.

Chaque page ci-dessous doit s'adapter à ces deux hauteurs **quelles qu'elles soient**, séparément et
combinées — c'est tout l'objet des deux corrections : deux mesures réelles, partagées par toutes les
pages, au lieu de constantes en dur qui pouvaient dériver.

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
| 6 | Redimensionner la fenêtre en franchissant 768 px et 1200 px sur `/tv` | Aucun changement — la Navbar n'est **jamais montée** sur les routes plein écran (#179 n'y a aucune prise) | [ ] |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 7 — Franchissement des points de rupture Navbar, 768 px et 1200 px (#179, AC1, AC3)

**Objectif** : La hauteur de la Navbar change **par sauts discrets** à ces deux largeurs précises
(paddings et libellés qui changent) — c'est le cas que #177 seul ne couvrait pas, puisque
`--regie-bar-h` était déjà mesuré mais `--navbar-h` restait une constante figée à ~72 px jusqu'à #179.

| Étape | Action | Résultat Attendu | OK ? |
|-------|--------|-----------------|------|
| 1 | Sur `/admin`, régler la largeur de la fenêtre à **1250 px** (au-dessus du point de rupture 1200 px) | Pas de scrollbar, Navbar dans son affichage "large" | [ ] |
| 2 | Rétrécir **lentement** jusqu'à **1150 px** (franchissant 1200 px) | La Navbar change de hauteur/densité à ce point précis ; **aucune scrollbar n'apparaît** pendant ni après la transition | [ ] |
| 3 | Continuer jusqu'à **820 px** (au-dessus du point de rupture 768 px) | Toujours pas de scrollbar | [ ] |
| 4 | Rétrécir jusqu'à **720 px** (franchissant 768 px) | La Navbar change à nouveau de hauteur ; toujours aucune scrollbar | [ ] |
| 5 | Répéter les étapes 1-4 sur **au moins 3 des huit pages admin** (GamePage recommandée — le pire cas — + deux autres au choix) | Même constat sur chacune : aucune scrollbar en franchissant les deux points de rupture | [ ] |
| 6 | Répéter l'étape 2 (franchissement de 1200 px) **avec un message régie actif** | Aucune scrollbar, même avec les deux sources de variation (Navbar + bandeau) combinées | [ ] |
| 7 | Faire varier le nombre de badges de compteurs clients affichés dans la Navbar si possible (ex. connecter/déconnecter des tablettes animateur pendant l'observation) | La Navbar peut changer de hauteur si sa largeur disponible varie ; toujours aucune scrollbar | [ ] |
| 8 | Naviguer vers `/tv` puis revenir sur `/admin` (démonte puis remonte la Navbar) | Pas de saut visuel anormal au retour, pas de scrollbar résiduelle | [ ] |

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
- [ ] Aucune scrollbar en franchissant les points de rupture 768 px et 1200 px, seuls ou combinés à un
      message régie actif (Scénario 7, #179)

---

## Non-Régression (suite automatisée, à exécuter avant validation manuelle)

| Étape | Action | Résultat Attendu | OK ? |
|-------|--------|-----------------|------|
| 1 | `cd server-go/web && npm test -- useElementHeightVar` | PASS — hook mutualisé (#179) : observation au montage, mise à jour sur nouvelle mesure, priorité `borderBoxSize` sur `contentRect`, arrondi, déduplication, remise à 0px au démontage, disconnect, plusieurs consommateurs indépendants | [ ] |
| 2 | `cd server-go/web && npm test -- Navbar` | PASS, y compris le bloc « mesure de hauteur --navbar-h (#179, T2) » : `--navbar-h` posée au montage, remise à 0px au démontage | [ ] |
| 3 | `cd server-go/web && npm test -- RegieMessageBar` | PASS **sans la moindre modification** de ce fichier depuis #177 (AC9 de #179 — garde-fou de l'extraction du hook `useElementHeightVar`) | [ ] |
| 4 | `cd server-go/web && npm test` (suite complète) | Aucune régression sur le reste de la suite (les tests utilisant `RegieMessageBar`/`Navbar` héritent désormais d'un mock global `ResizeObserver`) | [ ] |
| 5 | `grep -rn "calc(100vh - 120px)" server-go/web/src/pages/` | **Aucune occurrence** (AC7 de #177) | [ ] |
| 6 | `grep -n "72px\|48px\|120px" server-go/web/src/App.css` | Aucune occurrence dans la formule `--admin-chrome-h` — seul un repli `--navbar-h: 72px` en `:root` est légitime (valeur nominale avant première mesure, AC8 de #179) | [ ] |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Notes QA

[Espace pour observations]
