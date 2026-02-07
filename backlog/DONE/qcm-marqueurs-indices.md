# QCM - Marqueurs d'indices sur la barre de temps

**Statut** : ✅ Terminé (v2.52.0)

## Description

Afficher des marqueurs visuels sur la barre de progression du timer pour indiquer quand les indices QCM apparaîtront. Permet aux joueurs d'anticiper les pénalités de points.

## Objectifs

- [ ] Visualiser les seuils d'indices sur la barre de temps
- [ ] Permettre aux joueurs d'anticiper les pénalités
- [ ] Améliorer l'expérience de jeu QCM avec indices

## Design

```
┌──────────────────────────────────────────────────────────┐
│████████████████████████████│░░░░░░░░░░░░│░░░░░░░░░░░░░░░│
│                            ▲              ▲              │
│                         Indice 1       Indice 2          │
└──────────────────────────────────────────────────────────┘
```

- Simple trait vertical orange/jaune sur la barre de progression
- Position calculée : `(seuil / temps_total) * 100%`
- Pas de texte, juste la barre verticale

## Tâches

### Phase 1 - Implémentation (v2.42.0)

- [ ] **Calcul de la position des marqueurs**
  - Récupérer `QCM_HINT_THRESHOLD_1` et `QCM_HINT_THRESHOLD_2` depuis la question
  - Convertir les seuils (% du temps restant) en position sur la barre
  - Exemple avec timer 30s :
    - Seuil 1 à 25% → indice quand il reste 7.5s → position à 75% depuis la gauche
    - Seuil 2 à 12.5% → indice quand il reste 3.75s → position à 87.5%

- [ ] **Rendu des marqueurs**
  - Élément positionné en absolu sur la barre de progression
  - Couleur : orange/jaune (`--warning`)
  - Hauteur : 100% de la barre
  - Largeur : 2-3px

- [ ] **Conditions d'affichage**
  - Visible uniquement si `QCM_HINTS_ENABLED = true`
  - Visible uniquement pendant les phases STARTED
  - Masquer le marqueur une fois l'indice déclenché
  - Ne pas afficher si timer trop court (contraintes de sécurité)

### Phase 2 - Animations (v2.42.0)

- [ ] **Pulsation d'approche**
  - Légère pulsation quand le timer approche du seuil (5s avant)
  - Animation CSS `@keyframes hint-marker-pulse`

- [ ] **Flash au déclenchement**
  - Effet visuel quand l'indice est déclenché
  - Le marqueur disparaît avec un fade-out

### Phase 3 - Configuration (optionnel)

- [ ] **Option pour masquer les marqueurs**
  - Checkbox dans la configuration de la question
  - Champ `QCM_SHOW_HINT_MARKERS` (défaut: true)

## Fichiers à modifier

| Fichier | Modification |
|---------|--------------|
| `PlayerDisplay.jsx` | Ajout des marqueurs dans la zone timer |
| `PlayerDisplay.css` | Styles `.hint-marker`, `.hint-marker-label`, animations |
| `Timer.jsx` | Si composant Timer séparé |

## Styles CSS

```css
.hint-marker {
  position: absolute;
  top: 0;
  width: 3px;
  height: calc(100% + 20px);
  background: var(--warning);
  opacity: 0.8;
  z-index: 10;
}

.hint-marker-label {
  position: absolute;
  bottom: -18px;
  left: 50%;
  transform: translateX(-50%);
  font-size: 0.7rem;
  color: var(--warning);
  white-space: nowrap;
}

.hint-marker.triggered {
  animation: hint-marker-fade 0.5s ease-out forwards;
}

@keyframes hint-marker-pulse {
  0%, 100% { opacity: 0.8; }
  50% { opacity: 1; box-shadow: 0 0 8px var(--warning); }
}

@keyframes hint-marker-fade {
  to { opacity: 0; transform: scaleY(0); }
}
```

## Dépendances

- Nécessite que `QCM_HINTS_ENABLED` soit activé sur la question
- Utilise les seuils existants : `QCM_HINT_THRESHOLD_1`, `QCM_HINT_THRESHOLD_2`
- Référence : `backlog/DONE/qcm-indices-penalites.md` (v2.38.0)

## Version cible

v2.42.0
