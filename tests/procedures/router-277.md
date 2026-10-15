# Procédure de Test — Navigation après la migration react-router 7 (#277)

**Version** : v11.2.3 (`react-router-dom` 6.30.6 → 7.18.4)
**Date** : 2026-10-15
**Testeur** : utilisateur (seul, sur le binaire QUALIF Windows)
**Issue** : #277 — migration à iso-rendu, aucun fichier source modifié : seul le routage change de version
**Référence** : `_work/handoff/plan-v11.2.3.md` §#277 ; routes dans `server-go/web/src/App.jsx`

---

## Prérequis

- [ ] Environnement : QUALIF (binaire Windows v11.2.3, serveur lancé, port 80)
- [ ] Navigateur de bureau récent, **console développeur ouverte (F12 → Console)** pendant tous les scénarios ; vider la console avant chaque scénario
- [ ] Données : un quiz chargé avec au moins une question ; au moins une équipe et un buzzer (ou VJoueur) disponible pour l'enrôlement
- [ ] Pour le scénario 5 : **aucune clé API IA configurée** (ou un fournisseur sans clé), afin que la modale IA propose d'aller aux réglages
- [ ] Accès : aucun compte ; réseau local suffisant

> Règle commune à tous les scénarios : **aucun avertissement ni erreur `React Router`** (ex. « Future Flag Warning », « No routes matched ») ne doit apparaître dans la console.

---

## Scénario 1 — Enrôlement `/` puis arrivée sur `/player`

**Objectif** : `useNavigate` d'EnrollPage envoie vers `/player` après l'enrôlement.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `http://<serveur>/` | Page d'enrôlement affichée | | |
| 2 | S'enrôler (choisir/valider un nom d'équipe ou de joueur selon l'écran) | Le navigateur arrive sur `/player` (URL mise à jour) avec l'écran joueur | | |
| 3 | Dans l'écran joueur, utiliser l'action qui ramène à l'enrôlement (changement de joueur/déconnexion proposé par la page) | Retour sur `/` sans rechargement complet de la page | | |
| 4 | Console | Aucun avertissement react-router | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Affichage TV `/tv`

**Objectif** : la route `/tv` s'affiche et reste statique.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `http://<serveur>/tv` | Affichage TV (sans barre de navigation admin), pas de défilement | | |
| 2 | Lancer une question depuis `/admin` | La TV suit l'état (question affichée) | | |
| 3 | Console | Aucun avertissement react-router | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Barre de navigation `/admin` : `NavLink` et `NavGroupMenu`

**Objectif** : liens actifs corrects, menus de groupe fonctionnels.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `/admin` | Page Jeu ; l'entrée « Jeu » est visuellement active, et elle seule | | |
| 2 | Cliquer successivement : Scores, Palmarès, Historique, Joueurs, Quiz, Backstage, Réglages, Backup/Restaure, Mises à jour, Logs | À chaque clic : l'URL devient `/admin/<page>`, la bonne page s'affiche, **une seule** entrée est active (la courante), « Jeu » n'est plus active | | |
| 3 | Ouvrir un menu de groupe (menu déroulant) puis choisir une entrée | Le menu se ferme, la page change, l'entrée du groupe courante est marquée active | | |
| 4 | Cliquer TV / Joueur / Animateur (liens absolus) | Navigation vers `/tv`, `/player`, `/anim` ; retour à `/admin` possible | | |
| 5 | Console | Aucun avertissement react-router | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Redirection du favori `/admin/rafale`

**Objectif** : `<Navigate replace>` envoie vers `/admin/quiz?tab=rafale`, onglet Rafale actif (`useSearchParams`).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Saisir directement `http://<serveur>/admin/rafale` dans la barre d'adresse | L'URL devient `/admin/quiz?tab=rafale` ; la page Quiz s'ouvre sur l'onglet **Rafale** | | |
| 2 | Cliquer le bouton Retour du navigateur | On revient à la page précédant la saisie (pas de boucle sur `/admin/rafale`, car la redirection remplace l'entrée d'historique) | | |
| 3 | Ouvrir `http://<serveur>/admin/quiz` (sans paramètre) | Onglet **Questions** actif | | |
| 4 | Ouvrir `http://<serveur>/admin/quiz?tab=rafale` | Onglet **Rafale** actif d'emblée | | |
| 5 | Console | Aucun avertissement react-router | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Lien vers les réglages depuis la modale IA

**Objectif** : `useNavigate('/admin/settings')` des modales IA, avec fermeture de la modale.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur `/admin/quiz` (onglet Questions), ouvrir la génération IA par question | Modale « génération IA » ouverte | | |
| 2 | Cliquer le lien/bouton qui renvoie vers les réglages (clé API manquante) | Navigation vers `/admin/settings`, la modale est fermée | | |
| 3 | Revenir sur `/admin/quiz?tab=rafale` et ouvrir la génération IA Rafale | Modale Rafale ouverte | | |
| 4 | Cliquer le même lien vers les réglages | Navigation vers `/admin/settings`, modale fermée | | |
| 5 | Console | Aucun avertissement react-router | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — Bouton Retour / Avancer du navigateur

**Objectif** : l'historique reflète les navigations internes.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Depuis `/admin`, naviguer : Scores → Joueurs → Quiz → Réglages (via la barre) | URL et pages suivent | | |
| 2 | Retour du navigateur ×3 | On repasse par Quiz, Joueurs, Scores, puis `/admin` ; la page et l'entrée active de la barre correspondent à l'URL à chaque pas | | |
| 3 | Avancer du navigateur ×2 | Joueurs puis Quiz, avec la barre synchronisée | | |
| 4 | Console | Aucun avertissement react-router | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 7 — Rechargement F5 sur une sous-route (repli SPA du serveur Go)

**Objectif** : le serveur sert l'application pour toute sous-route, et le routeur la résout.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Aller sur `/admin/quiz`, appuyer F5 | La page Quiz se recharge (pas de 404, pas de page blanche) | | |
| 2 | Aller sur `/admin/settings`, F5 | La page Réglages se recharge | | |
| 3 | Aller sur `/admin/quiz?tab=rafale`, F5 | Rechargement sur l'onglet Rafale | | |
| 4 | Saisir `/admin/rafale`, valider (rechargement complet) | Redirection vers `/admin/quiz?tab=rafale` | | |
| 5 | Console | Aucun avertissement react-router | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 8 — Interface animateur `/anim`

**Objectif** : la route `/anim` s'affiche et sa navigation depuis/vers l'admin fonctionne.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `http://<serveur>/anim` (adresse directe) | Interface animateur affichée | | |
| 2 | F5 | Rechargement sur `/anim` sans erreur | | |
| 3 | Depuis `/admin`, cliquer « Animateur » dans la barre | Arrivée sur `/anim` ; retour navigateur → `/admin` | | |
| 4 | Console | Aucun avertissement react-router | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] Les 8 scénarios passent
- [ ] Aucun avertissement ni erreur react-router en console, sur aucun scénario
- [ ] Aucune différence visuelle avec la version précédente (iso-rendu)

## Notes QA

[Espace pour observations — en cas d'échec : scénario, étape, URL, capture de la console]
