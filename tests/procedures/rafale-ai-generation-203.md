# Procédure de Test — Génération IA du réservoir RAFALE (#203, v8.1.0)

**Version** : v8.1.0 (branche `milestone/v8.1.0`)
**Date** : 2026-09-01
**Testeur** : QA / Utilisateur (règle projet — jamais `qa` ni `deployer`, pas de navigateur fiable
dans les sessions agents)
**Issue** : #203 — génération IA dédiée au réservoir RAFALE (le seul type de question qui en était
privé), via un chemin de génération séparé (endpoint, schéma plat, persistance dans
`reservoir.json`) qui réutilise entièrement l'infrastructure de job existante.
**Référence** : `contracts/rafale-ai-generation.md`, `_work/reports/plan-20260901-162105.md`,
`docs/mockups/rafale-ai-generation-203.html`

---

## ⚠️ Points d'attention centraux

1. **Rejet, jamais troncature** — une question générée dont l'énoncé dépasse 100 caractères ou la
   réponse 40 caractères doit être **absente** du réservoir (comptée dans le compteur "écartées"),
   **jamais** raccourcie. Une question tronquée visible dans le réservoir est un bug bloquant.
2. **Aucun mélange des deux modales** — une génération lancée depuis `/admin/rafale` ne doit
   **jamais** faire réagir la modale de génération de `/admin/questions` (et réciproquement). Si la
   mauvaise modale affiche "Génération en cours", c'est un bug bloquant (fuite `TARGET`).
3. **Aucune donnée existante perdue** — le bouton « Générer via IA » n'écrase et ne supprime jamais
   de question existante du réservoir, et ne touche jamais au flag "déjà utilisée".
4. **[BREAKING mineur, à connaître]** — l'éditeur manuel du réservoir refuse désormais en erreur les
   énoncés > 100 caractères et les réponses > 40 caractères. Une question déjà en base plus longue
   reste **lisible et jouable**, elle ne peut simplement plus être ré-enregistrée telle quelle.

---

## Prérequis

- [ ] Environnement : QUALIF (ou LOCAL avec build de la branche `milestone/v8.1.0`)
- [ ] Une clé API valide configurée pour la génération, **avec les deux providers testés au moins
      une fois chacun** (Claude ET Groq — #142 et #196 n'ont cassé que sur Groq, jamais sur
      Anthropic ; ne fournir la clé que hors de ce document, jamais committée ni collée dans un
      rapport)
- [ ] Accès admin → `/admin/rafale` (réservoir RAFALE) et `/admin/questions` (générateur Quiz
      existant, pour le Scénario 4 de non-régression)
- [ ] Au moins une catégorie connue (dure ou personnalisée)
- [ ] Postes `/anim` et `/tv` disponibles pour le Scénario 5 (question jouable de bout en bout)
- [ ] Un jeu de données réservoir non vide recommandé pour observer la matrice "existant → après"
      (sinon tous les compteurs de départ sont à 0, ce qui reste un cas valide mais moins parlant)

---

## Scénario 1 — Génération nominale depuis `/admin/rafale`

**Objectif** : Vérifier le parcours complet — formulaire, paliers, catégories annotées, matrice,
progression, écriture en base, longueurs respectées.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `/admin/rafale` | Un bouton « ✨ Générer via IA » est visible dans l'en-tête de la carte réservoir | | |
| 2 | Cliquer sur le bouton | La modale s'ouvre avec un formulaire : thème, langue, publics, précisions, taille de lot (boutons 10/20/50/100/200), catégories, difficultés (★/★★/★★★) | | |
| 3 | Vérifier les catégories proposées | **Toutes** les catégories connues sont sélectionnables (pas seulement celles déjà dans le réservoir), chacune annotée d'un nombre (compte existant) | | |
| 4 | Sélectionner 1-2 catégories et 1-2 difficultés | Une matrice « existant → après » apparaît, une ligne par catégorie, une colonne par difficulté sélectionnée | | |
| 5 | Renseigner thème, au moins un public, choisir un palier (ex. 20) | Le bouton « ✨ Générer » devient actif | | |
| 6 | Lancer la génération | La modale passe en « Génération en cours », une barre de progression par lot s'affiche, la liste du réservoir se met à jour au fil de l'eau (sans rechargement de page) | | |
| 7 | Attendre la fin | Écran « Terminé » : nombre de questions créées + écartées, **total** du réservoir. Aucune ventilation par catégorie, aucun badge « nouveau » sur les lignes (décision GATE 2 — pas d'état « ajouté ») | | |
| 8 | Ouvrir `server-go/data/files/rafale/reservoir.json` (ou équivalent QUALIF) | Les nouvelles questions ont un `ID` de forme `r-NNN`, aucune question existante n'a changé d'ID ni de contenu | | |
| 9 | Mesurer la longueur des nouveaux énoncés/réponses affichés dans le tableau | Aucun énoncé > 100 caractères, aucune réponse > 40 caractères | | |
| 10 | Répéter la génération une seconde fois sur les **mêmes** catégorie/difficulté | Pas de doublon évident avec le lot précédent (même énoncé) dans le tableau | | |

---

## Scénario 2 — Fermeture/réouverture de la modale pendant le job

**Objectif** : Vérifier que fermer la modale n'interrompt pas la génération et que la rouvrir
retrouve la progression exacte.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer une génération avec un volume assez grand (ex. 100) pour laisser le temps de manipuler | Génération démarrée, plusieurs lots à venir | | |
| 2 | Fermer la modale (croix ou Échap) pendant que « Génération en cours » est affiché | La modale se ferme, **aucune confirmation demandée**, aucune erreur | | |
| 3 | Observer le tableau du réservoir en arrière-plan | Le compte de questions continue d'augmenter au fil des lots, sans rouvrir la modale | | |
| 4 | Rouvrir la modale via le bouton « ✨ Générer via IA » | La modale se rouvre directement sur l'état « Génération en cours », avec la progression à jour (pas un formulaire vierge) | | |
| 5 | Recharger complètement la page (F5) pendant que le job tourne encore | Après rechargement, rouvrir la modale retrouve aussi la progression en cours (ré-attachement serveur) | | |
| 6 | Laisser le job se terminer | Écran « Terminé » cohérent avec le total observé dans le tableau | | |

---

## Scénario 3 — Génération refusée pendant une manche RAFALE

**Objectif** : Vérifier le message explicite de refus quand une manche RAFALE est en cours.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Démarrer une manche RAFALE (`/anim`, lancer un round RAFALE) | La manche est en cours (compte à rebours actif) | | |
| 2 | Depuis un autre onglet admin, ouvrir `/admin/rafale` et tenter une génération IA | La requête est refusée, la modale affiche un message explicite indiquant qu'une manche RAFALE est en cours et qu'il faut réessayer après | | |
| 3 | Mettre la manche en PAUSE plutôt qu'en cours normal, retenter | Refus identique (le refus couvre STARTED **et** PAUSED) | | |
| 4 | Terminer/arrêter la manche RAFALE | | | |
| 5 | Retenter la génération | La génération démarre normalement | | |

---

## Scénario 4 — Non-régression : générateur Quiz existant

**Objectif** : Vérifier qu'aucun comportement du chemin de génération Quiz historique n'a changé,
et que les deux modales ne se marchent jamais dessus.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `/admin/questions`, lancer une génération Quiz classique (SPEEDY/QCM/MEMORY/MEMOTION/ARDOISE) | Comportement strictement identique à avant #203 : mêmes types proposés, RAFALE **absent** de la liste des types générables | | |
| 2 | Pendant que ce job Quiz tourne, ouvrir `/admin/rafale` et regarder si un état de génération s'affiche | Le bouton « ✨ Générer via IA » de `/admin/rafale` reste dans son état normal (pas de "Génération en cours" causé par le job Quiz) | | |
| 3 | Tenter malgré tout de lancer une génération RAFALE pendant ce job Quiz | Refus avec un message de type "une génération est déjà en cours" | | |
| 4 | Attendre la fin du job Quiz, puis lancer une génération RAFALE ; pendant qu'elle tourne, ouvrir `/admin/questions` | La modale Questions reste dans son état normal (ne réagit pas au job RAFALE en cours) | | |
| 5 | Vérifier les `question.json` produits par l'étape 1 | Forme strictement inchangée (mêmes champs qu'avant #203) | | |

---

## Scénario 5 — Question jouable de bout en bout, aucune troncature visuelle

**Objectif** : Valider empiriquement le plafond de 100/40 caractères sur les vraies surfaces
d'affichage — c'est la preuve finale que le plafond serveur est bien calibré.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Générer un petit lot (10) sur une catégorie/difficulté donnée | Nouvelles questions visibles dans le réservoir | | |
| 2 | Configurer une manche RAFALE utilisant cette catégorie/difficulté, démarrer sur `/anim` | La manche démarre normalement | | |
| 3 | Faire défiler plusieurs questions du lot généré, observer l'affichage `/anim` | 🔴 **Aucun énoncé n'est coupé/tronqué visuellement** (clamp 3 lignes), la réponse tient sur une ligne | | |
| 4 | Répéter en observant `/tv` | 🔴 **Aucun énoncé n'est coupé/tronqué visuellement** (clamp 4 lignes) | | |
| 5 | Si une troncature visuelle est malgré tout observée | **Bug bloquant** — noter l'énoncé exact et sa longueur en caractères, remonter au CDP (le plafond serveur devra être resserré ou le prompt ajusté, jamais l'inverse : ne pas relever le plafond sans revoir aussi le CSS) | | |

---

## Scénario 6 — Éditeur manuel : plafond de longueur (BREAKING mineur)

**Objectif** : Vérifier le compteur de caractères et le refus au-delà du plafond dans l'éditeur.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur `/admin/rafale`, ouvrir le formulaire d'ajout manuel | Un compteur `n/100` sous le champ Énoncé et `n/40` sous le champ Réponse | | |
| 2 | Taper un énoncé de 101 caractères ou plus | Le compteur passe en état d'alerte (couleur/style différent), le bouton d'enregistrement se désactive | | |
| 3 | Raccourcir en dessous de 100 caractères | Le compteur revient à l'état normal, le bouton se réactive | | |
| 4 | Taper une réponse de 41 caractères ou plus | Même comportement d'alerte que l'étape 2, côté réponse | | |
| 5 | Forcer l'envoi malgré tout (ex. contournement du bouton désactivé, si testable) | Le serveur refuse avec une erreur explicite (400), **aucune question n'est enregistrée tronquée** | | |
| 6 | Ouvrir une question déjà existante en base et **plus longue** que le nouveau plafond (si disponible) | Elle reste visible et lisible dans la liste, jouable en manche — seul un **nouvel enregistrement** tel quel est refusé | | |

---

## Critères de Validation

- [ ] Tous les scénarios nominaux (1, 2, 5) passent
- [ ] Les refus (Scénario 3, 4) affichent des messages lisibles et corrects
- [ ] 🔴 Aucune troncature visuelle observée sur `/anim` ni `/tv` (Scénario 5)
- [ ] 🔴 Aucune fuite croisée entre les deux modales de génération (Scénario 4)
- [ ] Aucune régression sur la génération Quiz existante (Scénario 4)
- [ ] Le compteur de caractères et le refus serveur de l'éditeur manuel fonctionnent (Scénario 6)
- [ ] Testé avec **les deux providers** (Claude et Groq) au moins une fois sur le Scénario 1

## Notes QA

[Espace pour observations]
