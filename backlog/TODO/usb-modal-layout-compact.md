# Réorganisation layout modale USB (compact, sans scroll)

**Statut** : 📋 Planifié

## Description

Revoir l'organisation visuelle de la modale USB (`USBConfigModal`) pour que tout son contenu tienne dans la modale sans nécessiter de scroll.

De plus, le bouton **"Flash USB"** doit être repositionné sur la même ligne que le bouton **"Envoyer et configurer"**, côte à côte.

## Objectifs

- [ ] Modale entièrement visible sans scroll, quelle que soit la résolution standard
- [ ] Bouton "Flash USB" aligné horizontalement avec "Envoyer et configurer" (même ligne)

## Tâches

### Phase 1 — Frontend
- [ ] Analyser le layout actuel de `USBConfigModal.jsx` et identifier ce qui dépasse
- [ ] Réduire les marges/paddings excessifs, compresser les sections si nécessaire
- [ ] Déplacer le bouton "Flash USB" sur la même ligne que "Envoyer et configurer"
- [ ] Vérifier que la modale tient sans scroll sur les résolutions 1280×720 et 1920×1080
- [ ] Tester visuellement (capture ou revue)

## Version cible

v3.2.0 (à déterminer)
