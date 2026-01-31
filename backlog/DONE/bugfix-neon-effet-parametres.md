# Bugfix - Effet Néon : Paramètres non appliqués

**Statut** : ✅ Terminé (v2.47.0)

## Description

L'effet néon (v2.46.0) a plusieurs paramètres de configuration qui ne sont pas correctement appliqués dans le CSS.

## Problèmes identifiés

### 1. Amplitude de pulsation ignorée
- Les paramètres `glow_pulse_min` et `glow_pulse_max` ne sont pas appliqués
- L'intensité (`intensity_gap`) est utilisée à la place de l'amplitude

### 2. Intensité à retirer
- Le paramètre `intensity_gap` ne devrait pas affecter la pulsation
- Seule l'amplitude (min/max) doit contrôler la pulsation du glow

### 3. Vitesse de pulsation forcée à 2s
- Le paramètre `glow_pulse_speed` n'est pas appliqué
- La valeur est hardcodée à 2s dans le CSS

### 4. Épaisseur de l'arc non appliquée
- Le paramètre `bar_thickness` n'est pas pris en compte
- L'épaisseur de l'arc lumineux reste constante

## Tâches

### Diagnostic
- [x] Vérifier `neon.css` pour identifier les variables non utilisées
- [x] Vérifier `PlayerDisplay.jsx` pour les variables CSS injectées
- [x] Identifier les valeurs hardcodées à remplacer par les variables

### Corrections CSS
- [x] Appliquer `--neon-glow-pulse-speed` à l'animation
- [x] Utiliser `--neon-glow-pulse-min` et `--neon-glow-pulse-max` pour l'opacité
- [x] Appliquer `--neon-bar-thickness` à l'épaisseur de l'arc
- [x] Supprimer l'utilisation de `intensity_gap` pour la pulsation

### Validation
- [x] Tester chaque paramètre individuellement
- [x] Vérifier l'aperçu temps réel dans ConfigPage

## Corrections appliquées (v2.47.0)

**Cause racine identifiée** :
Le struct `NeonEffectPayload` dans le protocole WebSocket ne contenait que 8 des 11 champs de configuration néon.

**Solution** :
- Ajout des 3 champs manquants dans `internal/protocol/messages.go`
- Mise à jour de la sérialisation dans `cmd/server/main.go` (2 fonctions)
- Complétion des defaults dans `PlayerDisplay.jsx`

**Améliorations UI** :
- Bouton mode "Barre" → "Neon" (plus clair)
- Slider "Intensité" déplacé vers "Arc lumineux" (meilleure organisation)

## Fichiers concernés

| Fichier | Modification |
|---------|--------------|
| `web/src/styles/neon.css` | Corriger les animations et variables |
| `web/src/pages/PlayerDisplay.jsx` | Vérifier injection des variables CSS |
| `web/src/pages/ConfigPage.jsx` | Vérifier les valeurs envoyées |

## Version cible

v2.46.1 (patch)
