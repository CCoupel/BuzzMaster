# Style des cartes joueurs - Page Admin Jeu

**Statut** : 📋 Planifié

## Description

Sur la page admin Jeu, les cartes des joueurs (dans la colonne Équipes) utilisent actuellement la couleur assignée au joueur (couleur QCM) comme fond. Il faut remplacer cette couleur par une couleur neutre gris clair, tout en conservant les mêmes atténuations (opacité, etc.) appliquées selon l'état du joueur.

## Objectifs

- [ ] Remplacer la couleur QCM du joueur par un gris clair neutre
- [ ] Conserver les mêmes règles d'atténuation existantes (opacité selon état)

## Tâches

### Phase 1 - Modification du style

- [ ] Modifier `TeamCard.jsx` ou `TeamCard.css` pour utiliser un fond gris clair standard
- [ ] Supprimer la référence à `ANSWER_COLOR` pour le fond des lignes joueur
- [ ] Garder les atténuations existantes (opacité réduite pour inactifs, etc.)

### Fichiers concernés

- `server-go/web/src/components/TeamCard.jsx`
- `server-go/web/src/components/TeamCard.css`

## Comportement attendu

| Avant | Après |
|-------|-------|
| Fond = couleur QCM du joueur (rouge, vert, jaune, bleu) | Fond = gris clair neutre |
| Atténuation selon état (opacité) | Atténuation identique conservée |

## Version cible

v2.48.0
