# BuzzControl v10.0.0 est disponible !

Depuis les débuts de BuzzControl, l'ambiance passait par l'écran et par les buzzers. Avec cette version, elle passe aussi par les murs : la salle elle-même rejoue avec vous.

## Les grandes nouveautés

### La salle prend les couleurs du jeu

BuzzControl pilote désormais un pont **Philips Hue**. Chaque équipe se voit attribuer une couleur fixe sur les ampoules qui lui sont associées — la même identité de couleur que sur son buzzer physique — et seule l'intensité varie selon ce qui se joue à l'instant. Quand aucune équipe n'est en avant, l'éclairage général de la salle adopte la teinte du thème de la question en cours.

[Capture à ajouter — page Réglages de jeu / association du pont]

### Les temps forts se voient, littéralement

Un score marqué déclenche un clignotement équipe/or, proportionnel aux points obtenus — un gros score, un gros effet. Et pendant le compte à rebours d'une question, la salle respire doucement avec une pulsation qui s'intensifie à mesure que le temps s'écoule, faisant monter la pression sans que l'animateur n'ait à dire un mot.

### Une configuration pensée pour ne jamais bloquer une soirée

Trois étapes suffisent : découverte automatique du pont sur le réseau, appui sur le bouton physique du pont pour l'associer, puis affectation de chaque ampoule à une équipe ou à l'éclairage général. Un panneau de conduite manuelle — allumé / éteint / automatique, plus un flash d'identification — reste accessible à tout moment depuis n'importe quel écran d'administration, pour la régie.

Et si le réseau fait des siennes en pleine soirée ? BuzzControl continue de fonctionner normalement, et reprend automatiquement le contrôle du pont dès que la connexion revient. Rien ne peut bloquer votre partie à cause de l'éclairage.

## Ce qu'il vous faut

Cette fonctionnalité est **entièrement optionnelle** — sans matériel Hue, votre installation BuzzControl continue de fonctionner exactement comme avant. Pour en profiter, il vous faut un **pont Philips Hue** et des **ampoules compatibles couleur**, disponibles dans le commerce, à associer à votre serveur depuis l'écran `Réglages de jeu`. Tout se passe en local sur votre réseau — aucune donnée ne part vers un service cloud tiers.

## Migration

Aucune action requise. Mettez à jour votre exécutable comme d'habitude : vos questions, équipes, scores et réglages existants sont conservés à l'identique. La configuration Hue est une étape entièrement optionnelle, réalisable à tout moment depuis l'écran `Réglages de jeu`.

## Merci

Un grand merci à celles et ceux qui ont fait remonter l'envie d'une salle plus immersive — cette version leur doit beaucoup.

[Télécharger](https://github.com/CCoupel/BuzzMaster/releases/tag/v10.0.0)  [Documentation](https://github.com/CCoupel/BuzzMaster/blob/main/docs/ADMIN_GUIDE.md)  [GitHub](https://github.com/CCoupel/BuzzMaster)
