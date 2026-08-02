---
name: feedback_marketing_image_verification
description: Toujours ouvrir/regarder visuellement une image existante avant de la réutiliser ou de l'assumer conforme à son nom/label
metadata:
  type: feedback
---

Ne jamais assigner une image à une carte/feature en se basant uniquement sur son nom de fichier ou un
label hérité d'un audit précédent — toujours l'ouvrir (Read) et vérifier visuellement son contenu réel
avant de l'utiliser ou de la recommander.

**Pourquoi** : lors de la réorganisation de la section Fonctionnalités du site marketing (2026-08-02),
l'agent `marketing-release` avait réutilisé `images/Jeu/Capture d'écran 2026-01-18 175935.png` en la
labellisant "Podium" (nom hérité d'une note d'audit jamais vérifiée visuellement) pour illustrer la carte
"Classements en Direct". En réalité ce fichier montre un écran MEMOTION/associations, pas un podium.
L'utilisateur a détecté le problème après publication sur `gh-pages` — un round de correction
supplémentaire a été nécessaire (commit `26f4440` après `dd5d5a1`).

**Comment appliquer** : avant toute publication (maquette GATE ou implémentation réelle) qui réutilise
une image déjà présente dans le repo (pas seulement les nouvelles captures fournies par l'utilisateur),
extraire le fichier et le lire avec le tool Read (vision) pour confirmer que son contenu correspond bien
à la description/au nom qu'on s'apprête à lui donner. Ne jamais faire confiance à un nom de fichier ou à
un label d'audit précédent sans revérification visuelle au moment de l'usage réel. Voir aussi
[feedback_marketing_gate_iterations.md](feedback_marketing_gate_iterations.md) pour le pattern GATE
global de cette session.
