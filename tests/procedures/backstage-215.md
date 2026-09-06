# Procédure de Test — Zone de préparation Backstage (#215)

**Version** : 9.0.0.x (milestone v9.0.0, Batch 1)
**Date** : 2026-09-04
**Issue** : #215
**Maquette** : `docs/mockups/backstage-215.html`
**Testeur** : QA / Utilisateur (validation manuelle obligatoire — jamais exécuté par `qa`/`deployer`)

---

## Contexte

`/admin/quiz` (page « Gestion des Questions ») mélangeait deux métiers sans rapport : la définition
du contenu du quiz et le réglage de la soirée (métadonnées de partie, entracte global, fonds
d'écran). #215 extrait ces trois briques vers une zone dédiée `/admin/backstage`, et `/admin/quiz`
passe à 2 onglets (Questions, Rafale). Aucun endpoint/action WebSocket/format disque nouveau — les
trois briques déplacées avaient déjà leurs points d'accès (`UPDATE_QUIZ_META`,
`UPDATE_ENTRACTE_CONFIG` + `/api/game/entracte-image`, `/background`, `/new-game-backgrounds`).

---

## ⚠️ Mise à jour Batch 2 — retour QUALIF v9.0.0.4 (Lot B / #214)

#214 a ajouté un 7e type de question (Entracte) sans toucher au sélecteur de type de `/admin/quiz`
→ onglet Questions, écrit pour 5 types (regroupement figé en 2 rangées) — la seconde rangée porte
désormais 4 boutons qui débordent en largeur réduite. **Scénario 10 ci-dessous**, corrigé dans ce
lot (même règle CSS que le sélecteur de type de carte MEMOTION, `tests/procedures/
rafale-memotion-217.md` Scénario 5, étape 6 — RAFALE y est devenu nestable pour la même raison).

---

## Prérequis

- [ ] Environnement : QUALIF ou LOCAL, serveur démarré
- [ ] Accès admin (`/admin`)
- [ ] Au moins une partie initialisée (pour tester NOUVELLE PARTIE sans risque)
- [ ] Navigateur avec accès à la console (facultatif, pour vérifier l'absence d'erreur JS)

---

## Scénario 1 — Structure de `/admin/backstage`

**Objectif** : Vérifier la présence et l'organisation des 3 onglets.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Naviguer vers `/admin/backstage` (directement, ou via la navbar) | Page « Backstage » affichée, en-tête « Préparation de la partie — quiz, entracte, fonds d'écran » | | |
| 2 | Observer la barre d'onglets | 3 onglets dans l'ordre : **Quiz**, **Entracte**, **Fonds d'écran** — onglet « Quiz » actif par défaut | | |
| 3 | Cliquer sur chaque onglet successivement | Contenu changé sans rechargement de page, un seul onglet actif à la fois | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Onglet Quiz : métadonnées de partie

**Objectif** : Vérifier la persistance des métadonnées (nom, thème, publics, difficultés, langue,
objectif, notes).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur l'onglet Quiz, saisir un nom et un thème de quiz | Champs modifiables | | |
| 2 | Cliquer les chips « Publics cibles » (ex : Ado, Adulte) | Chips deviennent actifs au clic, sélection multiple possible | | |
| 3 | Cliquer les chips « Difficultés visées » | Même comportement que les publics | | |
| 4 | Cliquer sur un interrupteur « TV » à côté d'un champ (ex. Thème) | Interrupteur bascule visuellement | | |
| 5 | Cliquer **Enregistrer** | Bouton affiche « Enregistré ✓ » brièvement | | |
| 6 | Recharger la page (`F5`) | Toutes les valeurs saisies sont toujours présentes | | |
| 7 | Naviguer vers `/tv` (autre onglet navigateur) | Les champs dont l'interrupteur TV est activé sont visibles sur l'écran TV ; ceux désactivés sont absents | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Onglet Entracte : configuration + piège `entracteConfigSaved` (C4)

**Objectif** : Vérifier la configuration du panneau de pause globale, et le piège de régression
historique (le formulaire ne doit jamais afficher la config *diffusée* pendant une pause active).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur l'onglet Entracte, saisir un titre (ex. « Pause déjeuner ») et un sous-titre | Champs modifiables | | |
| 2 | Régler les curseurs (taille du panneau, vitesse/intensité du mouvement, vitesse de transition) | Valeurs affichées se mettent à jour en direct | | |
| 3 | Uploader une image de fond | Aperçu mis à jour, message de succès | | |
| 4 | Cliquer **Enregistrer** | Bouton affiche « Enregistré ✓ » | | |
| 5 | Depuis la navbar (bouton ENTRACTE), déclencher un entracte manuel | Écrans TV/VJoueur affichent le panneau avec le titre/sous-titre/image enregistrés à l'étape 1-3 | | |
| 6 | **Pendant que l'entracte est actif**, revenir sur `/admin/backstage` → onglet Entracte, **modifier le titre** (ex. « Changement de salle ») et cliquer Enregistrer | Message « Un entracte est en cours — prendra effet au prochain entracte » visible ; l'enregistrement réussit sans être bloqué | | |
| 7 | Vérifier que le panneau **actuellement affiché** sur TV n'a **PAS** changé (toujours l'ancien titre) | Le panneau diffusé reste figé jusqu'à la fin de l'entracte en cours (contrat C4 — gel de config) | | |
| 8 | Terminer l'entracte, puis en déclencher un nouveau | Le nouveau titre (« Changement de salle ») est cette fois affiché | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Onglet Fonds d'écran : les deux destinations mutualisées

**Objectif** : Vérifier que le composant unique (Ambiance en jeu / Écran d'accueil Nouvelle Partie)
route bien chaque action vers **sa propre** destination, sans confusion.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur l'onglet « Fonds d'écran », repérer les 2 sections : « Ambiance — pendant le jeu » et « Écran d'accueil — Nouvelle Partie » | Deux blocs distincts, chacun avec son propre bouton **+ Image** | | |
| 2 | Uploader une image dans « Ambiance — pendant le jeu » | Image ajoutée **uniquement** à cette section | | |
| 3 | Uploader une image différente dans « Écran d'accueil — Nouvelle Partie » | Image ajoutée **uniquement** à cette section — la première section n'est pas affectée | | |
| 4 | Régler la durée et l'opacité d'une image (glisser le slider / champ numérique) | Valeurs persistées après rechargement | | |
| 5 | Glisser-déposer une image pour changer l'ordre | Nouvel ordre conservé après rechargement | | |
| 6 | Cliquer **Tout supprimer** sur UNE SEULE des deux sections | Seules les images de cette section disparaissent, l'autre section reste intacte | | |
| 7 | Naviguer vers `/tv` pendant une partie en cours | Les images « Ambiance » défilent en boucle sur l'écran TV | | |
| 8 | Déclencher NOUVELLE PARTIE, observer l'écran TV pendant la phase « Nouvelle Partie » | Les images « Écran d'accueil » défilent (ou dégradé animé par défaut si aucune image) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — NOUVELLE PARTIE accessible depuis 2 emplacements

**Objectif** : Vérifier que l'action est disponible aussi bien en préparation qu'en pleine soirée.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur `/admin/backstage` → onglet Quiz, repérer le bouton **NOUVELLE PARTIE** en en-tête | Bouton présent et cliquable | | |
| 2 | Sur `/admin` (GamePage), repérer également un bouton **NOUVELLE PARTIE** | Bouton présent, sans avoir à changer de page | | |
| 3 | Cliquer NOUVELLE PARTIE depuis `/admin` en pleine partie | Confirmation demandée (si applicable), la partie se réinitialise | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — `/admin/quiz` : 2 onglets (Questions, Rafale)

**Objectif** : Vérifier que la page Questions ne porte plus que la définition du contenu.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Naviguer vers `/admin/quiz` | 2 onglets : **Questions**, **Rafale** — onglet Questions actif par défaut | | |
| 2 | Vérifier l'ABSENCE des sections Quiz méta / Entracte / Fonds d'écran sur cette page | Ces sections ont disparu (déplacées vers Backstage) | | |
| 3 | Cliquer sur l'onglet Rafale | Éditeur du réservoir RAFALE affiché (identique à l'ancienne page dédiée) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 7 — Redirection `/admin/rafale` → onglet Rafale (favoris préservés)

**Objectif** : Vérifier qu'un ancien favori/lien direct ne casse pas.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Saisir directement `/admin/rafale` dans la barre d'adresse (ou cliquer un favori enregistré avant #215) | Redirection automatique vers `/admin/quiz`, onglet **Rafale** actif d'emblée | | |
| 2 | Vérifier l'URL finale dans la barre d'adresse | `/admin/quiz?tab=rafale` (ou équivalent) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 8 — Lien « configurer le quiz » depuis la modale IA (navigation réelle)

**Objectif** : Vérifier que le lien navigue vers Backstage au lieu de faire défiler la page (bug
corrigé par #215 — l'ancienne cible n'existe plus sur cette page).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur `/admin/quiz`, ouvrir la modale « ✨ Générer via IA » | Modale ouverte, rappel des métadonnées du quiz visible | | |
| 2 | Cliquer sur le lien « modifier » (configuration du quiz) | Navigation **réelle** vers `/admin/backstage` — la modale se ferme, la page change entièrement (PAS un simple défilement vers une section qui n'existe plus ici) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 9 — Navbar : groupe Config

**Objectif** : Vérifier les entrées de navigation.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir le menu « Config » de la navbar admin | Deux entrées distinctes : **Quiz** et **Backstage** | | |
| 2 | Vérifier l'ABSENCE d'une entrée « Rafale » séparée | Rafale n'apparaît plus comme entrée de menu dédiée (c'est un onglet de Quiz désormais) | | |
| 3 | Cliquer sur chaque entrée | Navigation correcte vers `/admin/quiz` et `/admin/backstage` respectivement | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 10 — Sélecteur de type de question ne déborde plus (Lot B, retour QUALIF v9.0.0.4)

**Objectif** : Vérifier que les 7 types de question restent tous accessibles, y compris en largeur
réduite.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur `/admin/quiz` (onglet Questions), ouvrir le formulaire d'ajout de question, observer le sélecteur « Type de question » en pleine largeur d'écran | Les 7 types (Speedy, QCM, Memory, Memotion, Ardoise, Rafale, Entracte) sont tous visibles, aucun tronqué | | |
| 2 | Réduire la largeur de la fenêtre (ou passer en vue tablette portrait) | Les boutons passent à la ligne proprement (`flex-wrap`) — aucun bouton coupé, poussé hors cadre, ni texte tronqué | | |
| 3 | Cliquer sur « Entracte » (dernier type de la liste) | Le type est bien sélectionné (bouton actif), quelle que soit la largeur d'écran testée à l'étape 2 | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] Les 3 onglets Backstage fonctionnent et persistent leurs données
- [ ] Le piège C4 (entracte gelé pendant une pause active) est vérifié visuellement, pas seulement
      lu dans le code
- [ ] Les 2 destinations de fonds d'écran ne se mélangent jamais
- [ ] NOUVELLE PARTIE accessible des deux emplacements
- [ ] Aucun favori `/admin/rafale` cassé
- [ ] Navigation réelle (pas de scroll résiduel) depuis la modale IA
- [ ] Navbar cohérente avec la nouvelle structure
- [ ] Sélecteur de type de question (7 types) ne déborde/ne tronque jamais, y compris en largeur
      réduite (Scénario 10)

---

## Notes QA

[Espace pour observations, captures d'écran, anomalies constatées]
