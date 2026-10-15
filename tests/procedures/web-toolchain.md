# Procédure de Test — #270 : migration d'outillage web (vite 8, vitest 5, Node 22) et porte `npm audit` complète

**Version** : 11.2.2
**Date** : 2026-10-13
**Testeur** : utilisateur (QA n'exécute pas les procédures manuelles — pas de navigateur en session agent)
**Automatique associé** : `server-go/internal/server/http_dist_smoke_test.go` (dist servi par Go : routes SPA, assets hachés, MIME, polices) + suite vitest complète (non-régression)
**Pourquoi un test manuel** : les tests automatiques ne voient ni le rendu visuel du build ni la minification CSS (lightningcss avec vite 8). Seul l'œil de l'utilisateur détecte une règle CSS perdue ou déformée.

## Prérequis
- [ ] Environnement : QUALIF, binaire du LOT-2 déployé (le `dist` est celui construit avec vite 8, embarqué dans le binaire)
- [ ] Navigateur de bureau (Chrome ou Edge) + tablette ou téléphone pour `/player` et `/anim` ; un écran TV ou un onglet plein écran pour `/tv`
- [ ] Console du navigateur (F12) ouverte : noter toute erreur rouge ou toute 404 dans l'onglet Réseau
- [ ] Données : au moins 2 équipes, 2 buzzers (ou VJoueurs), un quiz avec quelques questions (dont une QCM) chargé
- [ ] Pour comparer le rendu : une capture de référence d'une instance de la version précédente (PROD v11.2.1) si disponible, sinon les maquettes `docs/mockup/`
- Notation : `<QUALIF>` = URL de base de l'instance QUALIF

## Scénario 1 — Chargement du binaire : aucune ressource en échec
**Objectif** : le binaire sert un build complet, sans 404 ni mauvais type MIME.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `<QUALIF>/admin` avec l'onglet Réseau ouvert (cocher « Désactiver le cache »), recharger | Toutes les requêtes `/assets/*.js`, `/assets/*.css`, `/fonts/*.woff2` en **200** ; aucune ligne rouge | | |
| 2 | Inspecter le type de `index-*.js` et `index-*.css` | `javascript` et `text/css` ; aucune erreur « MIME type » en console | | |
| 3 | Console | Aucune erreur rouge (avertissements React Router orange tolérés) | | |
| 4 | Répéter les étapes 1 à 3 sur `/tv`, `/anim`, `/player` | Mêmes résultats | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Surface ADMIN : CSS appliqué, navigation, WebSocket
**Objectif** : la régie est identique à la version précédente (la minification CSS n'a rien cassé).

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `<QUALIF>/admin` | Mise en page complète : Navbar stylée (couleurs, espacements, icônes), polices Fredoka/Inter (pas la police système), bandeau régie en bas | | |
| 2 | Observer l'indicateur de connexion | Connecté (vert) ; le nombre de buzzers/TV/animateurs connectés s'affiche | | |
| 3 | Parcourir toutes les entrées du menu : Jeu, Scores, Équipes, Questions, Backstage, Historique, Palmarès, Logs, Configuration, Ambiance, Sauvegarde, Mises à jour | Chaque page s'affiche avec son style complet (boutons, tableaux, cartes, modales à l'ouverture) ; aucune page « nue » ou déformée | | |
| 4 | Ouvrir une modale (ex. « Lancer la partie » ou l'édition d'une équipe) puis la fermer | Modale centrée, fond assombri, boutons stylés ; fermeture correcte | | |
| 5 | Mode sombre / éléments colorés des équipes, si présents | Couleurs d'équipe correctes (pas de couleur perdue ou remplacée) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Surface TV : affichage statique, sans scroll
**Objectif** : la contrainte « TV statique » est conservée (`overflow: hidden`, unités viewport).

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `<QUALIF>/tv` en plein écran (F11) sur 16:9 | Affichage plein écran, **aucune barre de défilement**, rien qui déborde | | |
| 2 | Redimensionner la fenêtre (étroite, haute, large) | Le contenu se redimensionne (vh/vw) sans scroll ni chevauchement | | |
| 3 | Depuis la régie, afficher une question, puis les scores | La TV reflète l'état en direct (WebSocket `/ws/tv` connecté), styles corrects (typographie, couleurs, tableau des scores limité au top) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Surface ANIMATEUR (tablette)
**Objectif** : l'interface animateur est intacte.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `<QUALIF>/anim` sur tablette | Interface plein écran (sans Navbar admin), polices et couleurs correctes, boutons à taille tactile | | |
| 2 | Vérifier la connexion | Connectée (`/ws/anim`) ; l'état du jeu s'affiche | | |
| 3 | Lancer une question depuis la régie | La question apparaît sur la tablette, mise en page correcte, réponses visibles | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Surface JOUEUR (`/` et `/player`)
**Objectif** : l'inscription et le VJoueur fonctionnent.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `<QUALIF>/` sur téléphone | Page d'inscription stylée, polices Fredoka/Inter, formulaire utilisable | | |
| 2 | S'inscrire avec un nom et une équipe | Redirection vers le VJoueur, connexion WebSocket `/ws/player` | | |
| 3 | Depuis la régie, lancer une question | Le VJoueur affiche la question, les boutons de réponse sont stylés | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — Surface QUESTIONS / BACKSTAGE
**Objectif** : les deux pages les plus riches en CSS et en code chargé à la demande n'ont pas régressé.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `<QUALIF>/admin/quiz` | Éditeur de questions complet : onglets (Quiz, MEMOTION, Rafale), liste, formulaire, sélecteurs | | |
| 2 | Créer ou ouvrir une question QCM, modifier le texte, enregistrer | Enregistrement réussi, message de confirmation, aucune erreur console | | |
| 3 | Ouvrir l'onglet Rafale puis MEMOTION | Chaque onglet s'affiche avec son style | | |
| 4 | Ouvrir `<QUALIF>/admin/backstage` | Page Backstage complète : métadonnées du quiz, Entracte, fonds d'écran | | |
| 5 | Ouvrir la page **Mises à jour** (`/admin/updates`) et la page **Configuration** | Pages complètes ; la page Mises à jour charge ses ressources de flash firmware sans erreur (chunks `esp32*.js` chargés à la demande uniquement quand on les utilise) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 7 — Parcours de jeu court de bout en bout
**Objectif** : un mini-jeu complet traverse les 4 surfaces ensemble.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir simultanément `/admin`, `/tv`, `/anim` et un VJoueur | Les 4 sont connectés | | |
| 2 | Régie : lancer une question QCM | TV, animateur et VJoueur affichent la question | | |
| 3 | VJoueur : répondre | La réponse apparaît côté régie et animateur ; la TV passe en état « pause » attendu | | |
| 4 | Régie : révéler la réponse et attribuer les points | Scores mis à jour sur la TV et la régie, animations/couleurs correctes | | |
| 5 | Passer à la question suivante, puis ouvrir `/admin/scoreboard` | Classement cohérent avec les points attribués | | |
| 6 | Rechargement (F5) sur chacune des 4 surfaces en cours de jeu | Chacune se reconnecte et retrouve l'état courant | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 8 — Outillage : audit complet et Node 22 (vérifications à la console)
**Objectif** : la porte `npm audit` porte sur TOUTES les dépendances et la CI tourne sur Node 22.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `node --version` (poste de build) | ≥ 22.12 | | |
| 2 | `cd server-go/web && npm ci` | Installation propre, sans erreur ni avertissement d'engine bloquant | | |
| 3 | `npm ls vite vitest @vitejs/plugin-react @vitest/coverage-v8` | `vite@8.x`, `vitest@5.x`, `@vitejs/plugin-react@6.x`, `@vitest/coverage-v8@5.x`, sans `UNMET`/`invalid` | | |
| 4 | `npm audit --audit-level=high` (**sans** `--omit=dev`) | Code retour 0 ; aucune vulnérabilité « high » ou « critical » (les « moderate/low » éventuelles sont listées dans le rapport QA) | | |
| 5 | Si `overrides` présents dans `package.json` | Chacun est documenté (raison + vulnérabilité) ; aucun `overrides` sans justification | | |
| 6 | `grep -n "NODE_VERSION" .github/workflows/ci-branch.yml .github/workflows/release.yml` | `'22'` dans les deux fichiers (3 usages `setup-node` s'appuient dessus) | | |
| 7 | `grep -n "npm audit" scripts/ci.sh` et `commands.audit` dans `.claude/project-config.json` | `npm audit --audit-level=high` sans `--omit=dev` dans les deux | | |
| 8 | `cd server-go/web && npm run build` puis `npx vitest run` | Build réussi ; tous les fichiers de tests passent (aucun `skip` ajouté, voir contrôle « aucun test affaibli » du handoff) | | |
| 9 | `scripts/ci.sh` en entier | Vert de bout en bout | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Criteres de Validation

- [ ] Aucune 404 ni erreur de type MIME sur les 4 surfaces (scénario 1)
- [ ] Rendu identique à la version précédente sur admin, TV, animateur, joueur, questions/backstage (scénarios 2 à 6) — en particulier aucune règle CSS perdue
- [ ] TV toujours statique, sans scroll (scénario 3)
- [ ] Mini-jeu complet réussi sur les 4 surfaces, F5 compris (scénario 7)
- [ ] `npm audit --audit-level=high` vide sur tout le périmètre, Node 22 en CI, porte `ci.sh` définitive (scénario 8)

## Notes QA

(navigateur et version, URL de la QUALIF, écarts visuels avec captures, éventuelles vulnérabilités « moderate » restantes)
