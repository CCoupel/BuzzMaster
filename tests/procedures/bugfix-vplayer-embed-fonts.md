# Procédure de Test — Polices embarquées localement (#115, air-gapped)

**Version** : 5.7.24 (branche `bugfix/vplayer-embed-fonts`)
**Date** : 2026-07-26
**Branche** : bugfix/vplayer-embed-fonts
**Testeur** : QA

---

## Contexte du Bug

Le frontend (`index.html`) chargeait les polices Fredoka (titres) et Inter (texte courant)
via Google Fonts en ligne (`<link rel="preconnect">` vers `fonts.googleapis.com`/
`fonts.gstatic.com` + `<link rel="stylesheet">` vers l'API CSS2 de Google). Le réseau de
déploiement de BuzzControl étant **air-gapped** (aucun accès Internet), ce CSS externe ne se
chargeait **jamais** en production — dégradation silencieuse et invisible en dev (où Internet
est disponible) vers les polices système (Arial/Helvetica/sans-serif par défaut du navigateur).

**Fix** : les 2 fichiers woff2 (`fredoka-latin.woff2`, `inter-latin.woff2`, licence SIL OFL)
sont désormais embarqués dans `server-go/web/public/fonts/` → copiés tels quels dans `dist/`
par Vite → intégrés au binaire serveur par le mécanisme `//go:embed all:dist` existant (aucun
changement Go nécessaire). `src/styles/index.css` déclare 2 règles `@font-face` locales
(`url(/fonts/*.woff2)`) ; les `<link>` externes ont été retirés de `index.html`.

**Portée du fix** : toutes les pages utilisant les polices Fredoka/Inter (variables CSS
`--font-display`/`--font-body`), pas seulement VPlayer — l'admin et la TV sont donc concernées
par la même vérification visuelle.

---

## Prérequis

- [ ] Environnement : QUALIF déployé **réellement air-gapped** (ou, à défaut, un poste dont on
      coupe explicitement l'accès réseau externe — voir Scénario 2 pour la méthode)
- [ ] Build de la branche `bugfix/vplayer-embed-fonts` (`npm run build` avant embed Go, ou
      binaire QUALIF déjà buildé avec ce fix)
- [ ] Un navigateur avec les DevTools (onglet Réseau) pour vérifier l'absence de requête externe
- [ ] Accès à l'écran VJoueur (`/player`), à l'admin (`/game`, `/teams`) et à la TV (`/tv`)

---

## Scénario 1 — Aucune requête réseau externe (DevTools)

**Objectif** : Vérifier qu'aucune requête vers `fonts.googleapis.com`/`fonts.gstatic.com` (ou
tout autre domaine externe) n'est émise par le navigateur, même quand Internet est disponible.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir les DevTools (onglet Réseau/Network), vider le cache et le journal | Journal réseau vide | | |
| 2 | Charger l'écran VJoueur (`/player` ou `/enroll`) | Page chargée | | |
| 3 | Filtrer le journal réseau sur "font" ou "google" | **Aucune requête** vers `fonts.googleapis.com` ni `fonts.gstatic.com` | | |
| 4 | Observer les requêtes vers des fichiers `.woff2` | 2 requêtes locales : `/fonts/fredoka-latin.woff2` et `/fonts/inter-latin.woff2`, statut 200, même origine que le serveur | | |
| 5 | Répéter aux étapes 2-4 sur l'admin (`/game`, `/teams`) et la TV (`/tv`) | Même résultat : aucune requête Google Fonts, polices chargées localement | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Rendu visuel en air-gapped réel (ou simulé)

**Objectif** : Vérifier que les polices Fredoka/Inter s'affichent correctement même sans aucun
accès réseau externe — c'est le scénario réel de déploiement en salle de jeu.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Couper l'accès réseau externe du poste/réseau de test (débrancher l'accès Internet en amont du réseau local BuzzControl, ou utiliser un réseau QUALIF réellement air-gapped) — **ne pas seulement bloquer `googleapis.com` dans les DevTools**, l'enjeu est l'absence totale de sortie Internet | Poste isolé, seul le serveur BuzzControl local reste joignable | | |
| 2 | Vider le cache navigateur (Ctrl+Maj+Suppr ou équivalent) pour exclure tout résidu d'un chargement Google Fonts antérieur | Cache vidé | | |
| 3 | Charger l'écran VJoueur (`/player`) | Page chargée normalement, sans erreur bloquante | | |
| 4 | Observer la police des titres/gros textes | **Fredoka** visible (police arrondie/ludique caractéristique), pas de police système générique (Arial/Times/serif par défaut) | | |
| 5 | Observer la police du texte courant | **Inter** visible (sans-serif géométrique), pas de fallback système visible | | |
| 6 | Répéter aux étapes 3-5 sur l'admin (`GamePage`, `TeamsPage`) et la TV (`/tv`) | Même rendu Fredoka/Inter partout, cohérent avec un environnement connecté | | |
| 7 | Comparer avec une capture de référence prise en environnement connecté (avant le fix, ou sur une autre install avec Internet) | Rendu visuellement identique — aucune différence perceptible liée à la coupure réseau | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Non-régression : rendu inchangé en environnement connecté

**Objectif** : Vérifier que le fix n'a pas dégradé le rendu quand Internet **est** disponible
(l'ancien comportement chargeait aussi Fredoka/Inter, juste via le réseau).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur un poste avec accès Internet normal, charger VPlayer/admin/TV | Fredoka/Inter s'affichent normalement (comme avant le fix) | | |
| 2 | Vérifier les poids de police utilisés (titres en semi-gras/gras, texte courant en normal/medium) | Rendu des graisses (400/500/600/700 Fredoka, 400/500/600 Inter) inchangé — la règle `@font-face` locale couvre bien toute la plage via `font-weight: <min> <max>` | | |
| 3 | Vérifier les caractères accentués français (é, è, à, ç, œ) sur une page avec du contenu (ex. une question de quiz) | Tous les accents s'affichent correctement (sous-ensemble `latin` couvre les diacritiques français) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] Scénario 1 : aucune requête réseau vers Google Fonts, DevTools confirment le chargement local
- [ ] Scénario 2 : rendu Fredoka/Inter correct en air-gapped réel, aucun fallback système visible
- [ ] Scénario 3 : non-régression du rendu (graisses, accents français) en environnement connecté

## Notes QA

[Espace pour observations, captures d'écran comparatives (connecté vs air-gapped), méthode
exacte utilisée pour couper le réseau, version du binaire testé, date de test]
