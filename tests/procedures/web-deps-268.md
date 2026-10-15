# Procédure de Test — #268 : montée de react-router-dom 6.30.6 et porte `npm audit`

**Version** : 11.2.2
**Date** : 2026-10-13
**Testeur** : utilisateur (QA n'exécute pas les procédures manuelles — pas de navigateur en session agent)
**Automatique associé** : `server-go/web/src/App.routes.268.test.jsx` (routes, redirection, endpoints WebSocket)

## Prérequis
- [ ] Environnement : QUALIF, binaire du lot LOT-1 déployé (react-router-dom 6.30.6)
- [ ] Un navigateur de bureau (Chrome ou Edge) **et** une tablette/téléphone pour `/player` et `/anim`
- [ ] Un écran ou un second onglet plein écran pour `/tv`
- [ ] Console du navigateur (F12) ouverte sur l'onglet Console : noter toute erreur rouge
- [ ] Au moins une équipe et un buzzer déclarés (pour que les pages ne soient pas vides)
- Notation : `<QUALIF>` = URL de base de l'instance QUALIF (ex. `http://<hôte-qualif>`)

## Scénario 1 — Chaque route s'affiche (ouverture directe de l'URL)
**Objectif** : aucune route n'a été cassée par le bump ; chaque URL rend sa page, jamais un écran blanc.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `<QUALIF>/` | Page d'inscription des joueurs (EnrollPage), sans barre de navigation admin | | |
| 2 | Ouvrir `<QUALIF>/player` | Page VJoueur, plein écran, sans barre admin | | |
| 3 | Ouvrir `<QUALIF>/tv` | Affichage TV statique (pas de scroll), sans barre admin | | |
| 4 | Ouvrir `<QUALIF>/anim` | Interface animateur, plein écran, sans barre admin | | |
| 5 | Ouvrir `<QUALIF>/admin` | Page de jeu admin avec Navbar, bandeau bas régie présent | | |
| 6 | Ouvrir `<QUALIF>/admin/` (slash final) | Même page de jeu que l'étape 5 | | |
| 7 | Ouvrir successivement `/admin/scoreboard`, `/admin/teams`, `/admin/quiz`, `/admin/backstage`, `/admin/history`, `/admin/palmares`, `/admin/settings`, `/admin/ambiance`, `/admin/backup`, `/admin/updates`, `/admin/logs` | Chaque URL affiche sa page (Scores, Équipes, Questions, Backstage, Historique, Palmarès, Configuration, Ambiance, Sauvegarde, Mises à jour, Logs), avec Navbar | | |
| 8 | Console du navigateur après chaque page | Aucune erreur rouge (les avertissements « React Router Future Flag » orange sont tolérés) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Navigation complète par les menus (sans rechargement)
**Objectif** : la navigation client (liens de la Navbar / menu abeille) fonctionne et conserve l'état de connexion.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur `/admin`, cliquer chaque entrée de la Navbar et du menu abeille, l'une après l'autre (Jeu, Scores, Équipes, Questions, Backstage, Historique, Palmarès, Logs, Configuration, Ambiance, Sauvegarde, Mises à jour) | L'URL change à chaque clic, la page correspondante s'affiche, **aucun rechargement complet** (l'onglet ne clignote pas, l'indicateur de connexion reste vert) | | |
| 2 | Observer l'indicateur de connexion de la Navbar pendant la navigation | Reste « connecté » ; pas de déconnexion/reconnexion WebSocket à chaque page | | |
| 3 | Cliquer le logo / lien d'accueil si présent | Retour sur la page admin attendue, aucune erreur console | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Rafraîchissement sur une route profonde
**Objectif** : un F5 sur une route autre que la racine sert bien l'application (fallback SPA du serveur Go) et rend la bonne page.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Aller sur `/admin/palmares` par la Navbar, puis F5 | La page Palmarès se recharge (pas de 404 serveur, pas d'écran blanc) | | |
| 2 | Aller sur `/admin/quiz`, puis F5 | La page Questions se recharge | | |
| 3 | Aller sur `/admin/settings`, puis F5 | La page Configuration se recharge | | |
| 4 | Sur `/tv`, F5 | L'affichage TV se recharge, se reconnecte et affiche l'état courant | | |
| 5 | Sur `/anim`, F5 | L'interface animateur se recharge et se reconnecte | | |
| 6 | Sur `/player`, F5 | La page VJoueur se recharge (selon l'état : inscription ou jeu) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Retour / avance du navigateur
**Objectif** : l'historique du navigateur est cohérent après le bump.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Partir de `/admin`, naviguer par la Navbar : Équipes → Questions → Historique | Trois pages successives | | |
| 2 | Bouton « Précédent » du navigateur (×3) | Retour Questions, puis Équipes, puis Jeu — dans l'ordre inverse, chaque page s'affiche | | |
| 3 | Bouton « Suivant » du navigateur (×3) | Retour Équipes, Questions, Historique dans l'ordre | | |
| 4 | Raccourci clavier Alt+← puis Alt+→ | Même comportement | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Redirection `/admin/rafale` (ancien favori)
**Objectif** : l'ancienne route RAFALE redirige vers l'onglet Rafale des Questions, sans piéger le bouton « Précédent ».

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Aller sur `/admin/teams`, puis taper à la main `<QUALIF>/admin/rafale` dans la barre d'adresse | L'URL devient `/admin/quiz?tab=rafale` et l'onglet Rafale de la page Questions est actif | | |
| 2 | Bouton « Précédent » du navigateur | Retour sur la page précédente (une seule fois), **pas** de retour sur `/admin/rafale` ni de boucle de redirection | | |
| 3 | Ouvrir `<QUALIF>/admin/quiz?tab=rafale` directement | Onglet Rafale actif, même rendu qu'à l'étape 1 | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — URL inconnue (comportement actuel conservé)
**Objectif** : constater que le comportement sur URL inconnue n'a pas changé (aucune page 404 dédiée dans l'application).

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `<QUALIF>/admin/inconnue` | Coque admin (Navbar) visible, zone centrale vide, pas d'erreur bloquante | | |
| 2 | Depuis cet état, cliquer une entrée de la Navbar | La navigation fonctionne normalement | | |
| 3 | Ouvrir `<QUALIF>/inconnu` | Zone centrale vide (pas de Navbar), pas d'écran d'erreur React | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 7 — Endpoints WebSocket par page (non-régression du choix d'endpoint)
**Objectif** : chaque page se connecte au bon endpoint WebSocket (le routeur pilote ce choix).

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Onglet réseau (F12 → Réseau → WS), ouvrir `/admin` | Connexion `/ws/admin` | | |
| 2 | Ouvrir `/tv` | Connexion `/ws/tv` | | |
| 3 | Ouvrir `/anim` | Connexion `/ws/anim` | | |
| 4 | Ouvrir `/` et `/player` | Connexion `/ws/player` | | |
| 5 | Depuis `/admin`, naviguer vers `/admin/logs` | Une connexion supplémentaire `/ws/logs` (page Logs) en plus de `/ws/admin` | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 8 — Audit des dépendances de production
**Objectif** : plus aucune vulnérabilité « high » ou plus sévère dans les dépendances livrées (porte provisoire du lot ; la porte complète arrive avec LOT-2).

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `cd server-go/web && npm ci` | Installation propre, aucune erreur | | |
| 2 | `npm ls react-router react-router-dom @remix-run/router` | `react-router-dom@6.30.6`, `react-router@6.30.6`, `@remix-run/router` ≥ 1.23.3 | | |
| 3 | `npm audit --omit=dev --audit-level=high` | Code retour 0, aucune vulnérabilité listée (« found 0 vulnerabilities ») | | |
| 4 | `npm run build` | Build réussi, `dist/` généré | | |
| 5 | Vérifier que `scripts/ci.sh` étape 6 contient `npm audit --omit=dev --audit-level=high` et que `.claude/project-config.json` `commands.audit` l'enchaîne après `govulncheck` | Les deux présents | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Criteres de Validation

- [ ] Les 16 routes (scénario 1) rendent leur page, sans erreur console
- [ ] Navigation par menus, F5 sur route profonde et retour/avance du navigateur sans anomalie (scénarios 2 à 4)
- [ ] `/admin/rafale` redirige vers `/admin/quiz?tab=rafale` sans piéger « Précédent » (scénario 5)
- [ ] Comportement sur URL inconnue inchangé (scénario 6)
- [ ] Endpoints WebSocket corrects par page (scénario 7)
- [ ] `npm audit --omit=dev --audit-level=high` vide et build vert (scénario 8)

## Notes QA

(observations : navigateur et version, URL de la QUALIF, anomalies visuelles)
