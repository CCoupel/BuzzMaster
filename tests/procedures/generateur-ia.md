# Procédure de Test — Générateur de questions via IA (#8, v6.0.0)

**Version** : v6.0.0 (branche `feature/generateur-ia`)
**Date** : 2026-08-05
**Testeur** : QA

---

## Contexte de la Feature

Un bouton « ✨ Générer via IA » dans `QuestionsPage` ouvre une modale de paramétrage ; le backend
appelle l'API Claude en sortie structurée et écrit directement de nouvelles questions sur disque,
**en création uniquement** — aucune question existante n'est modifiée ni supprimée.

Le chantier est précédé d'un **correctif de bug destructif** sur `POST /config.json` : avant #8,
toute sauvegarde partielle (ex. un simple changement de l'effet néon) réinitialisait silencieusement
toutes les autres sections, y compris une éventuelle clé API. Le scénario 1 ci-dessous est donc une
**non-régression prioritaire**, pas un simple test de la nouvelle fonctionnalité.

Références :
- Contrat : `contracts/ai-generation.md`
- Maquette (normative pour les états chargement/succès/erreur, absents de l'artefact visuel) :
  `_work/mockups/8-generateur-ia.md`
- Plan : `_work/reports/planner-20260805-121900.md`

**Points GATE 2 à garder en tête pendant les tests** (cf. plan, section "Points appelant une
décision explicite") :
- Le contenu généré pour MEMORY et MEMOTION suit le **modèle réel** du produit (paires
  d'appariement / cartes à 3 faces), pas la description initiale de la spec (pas de notion
  d'émotion pour MEMOTION) — normal, ne pas remonter comme un bug.
- L'endpoint `/api/generate-questions` n'est **pas authentifié** : toute génération déclenchée
  depuis le LAN est facturée sur le compte Anthropic configuré — dette assumée, à ne pas tester
  comme un défaut de sécurité mais garder en tête en scénario 7.

---

## Prérequis

- [ ] Environnement : QUALIF (ou LOCAL avec build de la branche `feature/generateur-ia`)
- [ ] Accès admin (`/admin` ou `/anim`) → `QuestionsPage` et `ConfigPage`
- [ ] Une clé API Anthropic valide au format `sk-ant-...` pour les scénarios nominaux (fournie
      hors de ce document — ne jamais la committer ni la coller dans un rapport QA)
- [ ] Au moins 2 catégories existantes dans le quiz cible (une suffit pour les scénarios simples,
      2+ pour vérifier le regroupement par catégorie du panneau de succès)
- [ ] Un moyen de simuler une clé invalide (`sk-ant-xxxxxxxxxxxxinvalid`) et une coupure réseau
      (mode avion / pare-feu sortant) pour les scénarios d'erreur
- [ ] Accès à `server-go/data/files/questions/` (ou équivalent QUALIF) pour inspecter les
      `question.json` générés

---

## Scénarios

### Scénario 1 — Non-régression : la clé API survit à une sauvegarde d'un autre réglage (CA1, CA2)

**Objectif** : vérifier que le correctif de `POST /config.json` empêche la destruction silencieuse
d'une section lors de la sauvegarde d'une autre.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Aller dans Paramètres → section IA, saisir une clé `sk-ant-...` valide, cliquer Enregistrer | Toast « Clé API enregistrée », badge passe à « ✅ Clé configurée » | | |
| 2 | Recharger la page Paramètres | Le badge reste « ✅ Clé configurée » (le champ mot de passe reste vide, la clé n'est jamais renvoyée) | | |
| 3 | Aller dans la section Effet Néon, changer une valeur (ex. activer l'effet), cliquer Enregistrer | Sauvegarde réussie, effet appliqué | | |
| 4 | Revenir à la section IA sans recharger, puis recharger la page | Le badge affiche toujours « ✅ Clé configurée » — **la clé n'a pas été effacée** | | |
| 5 | Aller dans Paramètres serveur, changer un réglage (ex. Mode debug), Enregistrer | Sauvegarde réussie | | |
| 6 | Recharger, vérifier la section IA **et** l'effet Néon réglé à l'étape 3 | Les deux sections sont **intactes** — aucune n'a été réinitialisée par les sauvegardes des étapes 3/5 | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 2 — Suppression explicite de la clé (contrat §2, §9)

**Objectif** : vérifier que seule l'action « Supprimer la clé » efface la clé — pas un
« Enregistrer » avec champ vide.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Avec une clé déjà configurée, cliquer Enregistrer **sans rien saisir** dans le champ mot de passe | La clé reste configurée (badge inchangé) — un champ vide ne l'efface pas | | |
| 2 | Cliquer « Supprimer la clé » | Une confirmation apparaît (« Supprimer la clé API Claude enregistrée ? ») | | |
| 3 | Annuler la confirmation | Rien ne change, badge toujours « ✅ Clé configurée » | | |
| 4 | Recliquer « Supprimer la clé », confirmer | Toast « Clé API supprimée », badge passe à « ⚠️ Aucune clé configurée » | | |
| 5 | Aller dans QuestionsPage | Le bouton « ✨ Générer via IA » est **désactivé**, note « Configurer une clé API dans Paramètres pour activer la génération IA » visible (CA4) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 3 — Validation du format de clé (contrat §2)

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Section IA, saisir une clé qui ne commence pas par `sk-ant-` (ex. `abc123`), Enregistrer | Message d'erreur « Format de clé API invalide (attendu : sk-ant-...) », rien n'est sauvegardé | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 4 — Parcours complet clé → génération → questions visibles sans rechargement (CA5, CA7, CA8, CA9)

**Objectif** : le golden path de la fonctionnalité, avec les 4 types de questions.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Clé API configurée, aller dans QuestionsPage | Bouton « ✨ Générer via IA » actif (pilule accent) | | |
| 2 | Cliquer le bouton | Modale s'ouvre : bloc « Paramètres du Quiz » pré-rempli depuis le Quiz global (thème, population, difficulté cochée, langue), bloc « Cette génération » avec catégories/volume/répartition par défaut (Speedy 40% / QCM 40% / Memory 20% / Memotion 0% désactivé) | | |
| 3 | Vérifier que modifier le thème **dans la modale** ne change pas le thème affiché dans la section Quiz de QuestionsPage en arrière-plan | Le thème global reste inchangé (CA6) — aucun `UPDATE_QUIZ_META` émis depuis la modale | | |
| 4 | Cocher au moins une catégorie, activer les 4 types (glisser Memotion à une valeur non nulle), régler le volume à 8 questions | Répartition affiche toujours 100% au total ; bouton « ✨ Générer » devient actif | | |
| 5 | Cliquer « ✨ Générer » | La modale bascule immédiatement sur un panneau « Génération en cours… » avec spinner et note « 1 à 3 minutes » ; le bouton × est désactivé | | |
| 6 | Pendant l'attente, essayer de fermer via ×, Échap, clic hors modale | Aucune fermeture n'a lieu (maquette §6.1) | | |
| 7 | Attendre la fin (jusqu'à 3 min) | Panneau de succès : « N question(s) créée(s). », liste par catégorie (« • <Catégorie> — X questions ») | | |
| 8 | Cliquer « Fermer » | Modale se ferme, la liste de QuestionsPage se met à jour **sans rechargement de page** et défile jusqu'aux nouvelles questions (CA8) | | |
| 9 | Ouvrir chacune des 4 types de questions créées (Speedy, QCM, Memory, Memotion) en édition | Chaque question s'affiche et s'édite normalement, comme une question créée manuellement — aucun champ manquant ou mal formé (CA9) | | |
| 10 | Vérifier `question.json` de 2-3 questions générées (accès fichier ou export) | `POINTS` et `TIME` sont des **chaînes** (`"20"`, pas `20`) ; `POINTS_TARGET` = `PLAYER` pour Speedy, `TEAM` pour QCM/Memory/Memotion ; questions MEMORY ont bien `MEMORY_PAIRS[].ID` entiers ; questions MEMOTION ont `MOTION_CARDS[].ID` = `"mc-1"`, `"mc-2"`, ... | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 5 — Non-régression sur les questions existantes (CA7)

**Objectif** : une génération ne doit jamais toucher aux questions déjà présentes.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Noter le contenu exact (texte, ID, ORDER, médias) de 2-3 questions existantes avant génération | — | | |
| 2 | Lancer une génération de plusieurs questions (scénario 4) | Génération réussie | | |
| 3 | Revérifier les 2-3 questions notées à l'étape 1 | **Identiques bit à bit** — aucun champ modifié, aucun média perdu, aucun changement d'ordre | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 6 — Sliders de répartition (maquette §3)

**Objectif** : valider manuellement l'algorithme de rebalance (déjà couvert en automatisé,
`AIGenerateModal.test.jsx` — ce scénario est la validation visuelle/ergonomique).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir la modale, observer la répartition par défaut | Speedy 40% / QCM 40% / Memory 20% / Memotion 0% (désactivé, curseur grisé) | | |
| 2 | Glisser le curseur Speedy à 80% | QCM et Memory diminuent proportionnellement, le total reste 100% | | |
| 3 | Désactiver le toggle QCM | Sa valeur redescend à 0%, les autres types actifs absorbent sa part au prorata, total toujours 100% | | |
| 4 | Réactiver Memotion (toggle ON) | Memotion repart à 20%, les autres se rééquilibrent, total 100% | | |
| 5 | Désactiver tous les types un par un | Le dernier type actif finit à 100% ; en désactivant également celui-ci, le bouton « ✨ Générer » devient **désactivé** (aucune répartition valide) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 7 — Cas d'erreur (maquette §6.3, contrat §3)

**Objectif** : chaque erreur affiche le message dédié, sans crash ni création partielle silencieuse.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Configurer une clé invalide (ex. `sk-ant-invalide000`), tenter une génération | « Clé API Claude invalide ou absente. Vérifiez-la dans Paramètres. » + bouton « Configurer une clé API » qui renvoie vers Paramètres | | |
| 2 | Couper l'accès réseau externe du serveur (ou bloquer `api.anthropic.com`), tenter une génération | « Le serveur n'a pas pu joindre l'API Claude. Vérifiez l'accès réseau. » | | |
| 3 | Réduire `ai.timeout_seconds` très bas (via `config.json`, test technique) et lancer une génération volumineuse | « La génération a dépassé le temps imparti. Réduisez le volume et réessayez. » | | |
| 4 | Sur une erreur quelconque, cliquer « Réessayer » | Retour au formulaire avec **toutes les valeurs saisies conservées** (thème, catégories, répartition...) — pas de resaisie | | |
| 5 | Sur une erreur quelconque, cliquer « Fermer » | Modale se ferme, **aucune question créée** n'apparaît dans QuestionsPage | | |
| 6 | Après chaque scénario d'erreur, vérifier `data/files/questions/` | Aucun répertoire de question orphelin ou partiellement écrit n'a été créé | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 8 — Écran TV NEW_GAME avec les 6 champs (CA11, risque R4)

**Objectif** : valider l'absence de scroll/débordement avec le contenu le plus défavorable.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Renseigner les 6 champs du Quiz (Nom, Thème, Notes, Population, Difficulté, Langue) avec des **textes longs** (ex. thème de 80+ caractères) | Sauvegarde réussie | | |
| 2 | Ouvrir l'écran TV (`/tv` ou `/player`) sur l'état NEW_GAME | Population / Difficulté / Langue s'affichent groupées sur **une seule ligne compacte de badges**, à côté de Nom/Thème/Notes | | |
| 3 | Observer l'écran en entier | **Aucun scroll, aucun débordement**, `overflow: hidden` respecté (contrainte projet) | | |
| 4 | Vider un des 3 nouveaux champs (ex. Difficulté) côté admin, revenir sur l'écran TV | Le badge correspondant **disparaît** (affichage conditionnel — champ vide = masqué), pas de badge vide | | |
| 5 | Tester sur au moins 2 résolutions d'écran (Full HD et une résolution TV plus petite si disponible) | Comportement identique, pas de débordement | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 9 — Validation serveur des questions générées (contrat §5.1)

**Objectif** : vérifier qu'une réponse LLM partiellement invalide n'empêche pas la création des
questions valides. **Difficile à déclencher à la demande** (dépend de la réponse réelle du LLM) —
à valider en priorité via les tests automatisés (`ai_generation_test.go`), ce scénario sert de
confirmation si l'occasion se présente naturellement (ex. génération d'un gros volume).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer une génération d'un volume important (ex. 50 questions, les 4 types actifs) | Génération réussie | | |
| 2 | Observer le panneau de succès | Si `skipped_count > 0` : ligne d'avertissement « ⚠️ M question(s) écartée(s) (format invalide ou catégorie inconnue). » visible sous le décompte | | |
| 3 | Compter les questions réellement apparues dans QuestionsPage | Le nombre correspond exactement à `created_count` annoncé, ni plus ni moins | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] Tous les scénarios nominaux (1, 2, 4, 6, 8) passent
- [ ] Les messages d'erreur (scénario 7) sont lisibles, corrects, et ne créent jamais de question
      partielle
- [ ] Aucune régression sur les questions existantes (scénario 5) ni sur les autres sections de
      Paramètres (scénario 1)
- [ ] Aucun scroll ni débordement sur l'écran TV NEW_GAME (scénario 8)
- [ ] La clé API n'apparaît à aucun moment dans la console navigateur, les logs serveur, ou un
      message d'erreur affiché (CA12 — vérifier `/logs` et la console DevTools pendant les
      scénarios 4 et 7)

## Notes QA

[Espace pour observations — en particulier, documenter tout écart entre le contenu réellement
généré pour MEMORY/MEMOTION et ce qui était attendu, cf. point GATE 2 en tête de document.]
