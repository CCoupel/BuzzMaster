# BuzzControl v10.0.0 - "La salle s'allume"

**Date de sortie** : 9 septembre 2026

---

## 🎉 Quoi de neuf ?

### 💡 L'éclairage de la salle rejoue avec vous (Philips Hue)

BuzzControl pilote désormais un pont **Philips Hue** pour transformer l'éclairage de la salle en véritable partenaire de jeu. Chaque équipe reçoit sa propre couleur sur les ampoules qui lui sont associées — la couleur ne bouge jamais, seule son intensité varie selon ce qui se passe dans la partie, exactement comme sur les buzzers physiques. Quand aucune équipe n'est à l'honneur, l'éclairage général prend la couleur du thème de la question en cours.

**Bénéfice** : la salle entière participe visuellement à l'ambiance, sans que l'animateur ait quoi que ce soit à faire pendant le jeu.

### 🎉 Un flash de célébration à chaque bonne réponse

Quand une équipe marque des points, ses ampoules clignotent en équipe/or — plus l'équipe marque de points, plus l'effet est marqué. Une manière visuelle et immédiate de souligner les temps forts de la soirée, visible même depuis le fond de la salle.

### ⏳ La salle respire avec le chronomètre

Pendant le compte à rebours d'une question, l'éclairage général pulse doucement — et l'intensité de la pulsation augmente à mesure que le temps s'écoule, pour faire monter la pression naturellement sans un mot de l'animateur.

### 🎛️ Une configuration en trois clics, et un panneau de régie toujours à portée de main

Une nouvelle page dédiée (`Réglages de jeu`) permet d'associer votre pont Hue en un clic (découverte automatique sur le réseau) puis d'affecter chaque ampoule à une équipe ou à l'éclairage général. Un panneau de conduite manuelle, accessible depuis n'importe quel écran d'administration, permet à tout moment de basculer l'éclairage en allumé, éteint ou automatique, et de faire clignoter une ampoule pour l'identifier physiquement.

**Bénéfice** : aucune compétence technique requise pour brancher son pont Hue, et un contrôle manuel de secours toujours disponible pendant l'événement.

### 🛡️ Robuste même quand le réseau ne l'est pas

Si le pont Hue devient injoignable en cours de soirée (coupure Wi-Fi, redémarrage du pont...), BuzzControl continue de fonctionner normalement et reprend la main automatiquement dès que la connexion revient — sans jamais bloquer une partie. À l'arrêt du serveur, l'éclairage est proprement éteint.

---

## ℹ️ À savoir avant de mettre à jour

- Cette fonctionnalité est **entièrement optionnelle** : sans pont Philips Hue configuré, rien ne change dans votre installation existante.
- Elle nécessite un **pont Philips Hue** et des **ampoules compatibles couleur**, à vous procurer séparément (non fournis par BuzzControl).
- Le pilotage se fait **en local, sur votre réseau** — aucune donnée ne transite par un service cloud tiers.

## Comment mettre à jour

1. Téléchargez la dernière release sur [GitHub Releases](https://github.com/CCoupel/BuzzMaster/releases)
2. Remplacez l'exécutable existant par la nouvelle version
3. Relancez le serveur — vos questions, équipes et scores existants sont conservés
4. (Optionnel) Rendez-vous dans `Réglages de jeu` pour associer votre pont Philips Hue

## Liens

- [Documentation](https://github.com/CCoupel/BuzzMaster/blob/main/docs/ADMIN_GUIDE.md)
- [GitHub Release](https://github.com/CCoupel/BuzzMaster/releases/tag/v10.0.0)
- [Signaler un problème](https://github.com/CCoupel/BuzzMaster/issues)
