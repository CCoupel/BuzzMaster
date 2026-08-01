# Procédure de Test — Réponses ARDOISE en grille (#116)

**Version** : à définir (branche `feature/ardoise-grid-layout`)
**Date** : 2026-08-01
**Branche** : feature/ardoise-grid-layout (depuis `main`, inclut #117)
**Testeur** : QA

---

## Contexte de la Feature

Le panneau « Réponses ARDOISE » de la page admin passait d'une liste verticale (une ligne pleine
largeur par équipe) à une grille responsive. Avec seize équipes, seize lignes empilées deviennent
quatre rangées ; avec six, deux rangées. Le contenu de chaque carte est strictement conservé :
rang, pastille et nom d'équipe, texte de réponse, délai au millième (#117), bouton d'attribution
de points en phase `REVEALED`, équipes sans réponse en fin de liste.

**Décisions de conception à vérifier** :
- `auto-fill` (pas `auto-fit`) : à 2-3 équipes, les cellules gardent leur taille naturelle au lieu
  de s'étirer sur toute la largeur.
- Le rang, discret en liste (la position verticale suffisait), devient explicite en tête de
  cellule en grille — la première réponse est encadrée.
- **Aucune troncature** des réponses longues : l'animateur corrige sur ce texte, une coupure
  risquerait une attribution de points erronée. Pas d'infobulle non plus (ne fonctionne pas au
  doigt sur tablette).

Maquette de référence (avant/après, 16 équipes, réponses inégales) :
https://claude.ai/code/artifact/417cb137-a88d-4dc3-a362-9f050f3197e6

---

## Prérequis

- [ ] Environnement : QUALIF ou LOCAL avec build de la branche `feature/ardoise-grid-layout`
- [ ] Un quiz avec au moins une question de type ARDOISE
- [ ] Pouvoir varier rapidement le nombre de VJoueurs inscrits (2, 6, 16) — utiliser plusieurs
      onglets/appareils ou un script d'inscription en rafale si disponible
- [ ] Un accès à la page admin (`/game`) redimensionnable (fenêtre navigateur), et idéalement une
      tablette réelle pour la vérification finale

---

## Scénario 1 — Disposition en grille à différents effectifs

**Objectif** : Vérifier le passage en grille et son comportement responsive.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer une question ARDOISE avec **2 équipes** inscrites, laisser les deux répondre | Les deux cellules gardent une taille compacte, **ne s'étirent pas** sur toute la largeur du panneau (comparer à la maquette « avant/après ») | | |
| 2 | Même question avec **6 équipes** | La grille affiche environ 2 rangées de cellules, hauteur nettement réduite par rapport à l'ancienne liste verticale | | |
| 3 | Même question avec **16 équipes** | La grille se remplit sur plusieurs rangées, gain de hauteur très net comparé à 16 lignes empilées | | |
| 4 | Redimensionner la fenêtre du navigateur (réduire puis agrandir) | Le nombre de colonnes s'adapte à la largeur disponible, rien ne déborde horizontalement | | |
| 5 | Consulter la page sur une tablette réelle | Rendu cohérent, cellules lisibles, pas de débordement | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Repère de rang et ordre chronologique (non-régression #117)

**Objectif** : Vérifier que le rang reste un repère fiable une fois la lecture verticale perdue.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | 3-4 équipes répondent à une question ARDOISE, dans un ordre connu et espacé de quelques secondes | Chaque cellule affiche son rang (1, 2, 3…) en tête, cohérent avec l'ordre réel de saisie | | |
| 2 | Observer la première cellule (rang 1) | Visuellement distinguée des autres (bordure accentuée), repérable d'un coup d'œil même en grille sur plusieurs colonnes | | |
| 3 | Une équipe corrige sa réponse en cours de saisie | Son rang ne change pas (non-régression #117) | | |
| 4 | Vérifier le délai affiché sur chaque cellule | Toujours au format `X.XXX s` (trois décimales), identique à avant la feature | | |
| 5 | Passer en phase `STOPPED` puis `REVEALED` | Ordre et délais inchangés à travers les transitions | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Réponses longues, sans troncature

**Objectif** : Vérifier la décision de conception n° 3 (pas de coupure, pas d'infobulle).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Depuis un VJoueur, saisir une réponse très longue (plusieurs dizaines de caractères, avec espaces) | Côté admin, la cellule s'agrandit en hauteur pour afficher le texte **intégralement** — aucune coupure, aucun « … », aucune infobulle au survol | | |
| 2 | Observer les cellules voisines dans la même rangée | Elles ne s'étirent pas artificiellement à cause de la réponse longue (`align-items: start`) | | |
| 3 | Depuis un autre VJoueur, saisir une chaîne longue **sans aucun espace** (ex. un mot inventé de 40+ caractères) | Le texte se coupe proprement à l'intérieur de la cellule (retour à la ligne au milieu du mot si nécessaire) — **aucun débordement horizontal** de la cellule ni de la page | | |
| 4 | Vérifier sur tablette (ou en réduisant la fenêtre) | Même comportement, pas de scroll horizontal parasite | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Non-régressions

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Phase `REVEALED`, cliquer sur le bouton d'attribution de points d'une équipe | Les points sont attribués à la bonne équipe, comme avant la feature | | |
| 2 | Une équipe n'ayant pas répondu | Sa cellule apparaît en fin de grille, sans rang ni délai, texte « — » | | |
| 3 | Lancer une question QCM, MEMORY et MEMOTION | Aucun changement visuel ou fonctionnel sur ces modes — la feature ne touche que le panneau ARDOISE | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] La disposition en grille est visible et responsive à 2, 6 et 16 équipes (scénario 1)
- [ ] Les cellules ne s'étirent pas à faible effectif (scénario 1)
- [ ] Le rang reste un repère fiable, la première réponse est visuellement distinguée (scénario 2)
- [ ] L'ordre chronologique et le format du délai (#117) sont inchangés (scénario 2)
- [ ] Aucune réponse n'est tronquée, quelle que soit sa longueur (scénario 3)
- [ ] Aucun débordement horizontal, y compris sur chaîne sans espace (scénario 3)
- [ ] Bouton de points et équipes sans réponse en fin de liste : comportement inchangé (scénario 4)
- [ ] Aucun impact sur les autres modes de jeu (scénario 4)

## Notes QA

[Espace pour observations]
