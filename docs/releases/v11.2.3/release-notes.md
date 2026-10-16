# Release v11.2.3 - Tests et sécurité

**Date** : 2026-10-15

Une version de consolidation : pas de nouvelle fonctionnalité de jeu, mais un correctif, un renforcement de la
confidentialité des informations réseau et un socle technique plus solide.

## Corrections

- Un enregistrement de question refusé par le serveur ne laisse plus de dossier vide derrière lui : l'identifiant reste disponible pour votre essai suivant.

## Sécurité

- Les informations réseau du serveur (adresses et interfaces) ne sont plus envoyées à la TV, à l'animateur ni aux joueurs virtuels : seule l'administration les reçoit. L'écran d'administration ne change pas.
- Mise à jour de la bibliothèque de navigation de l'interface (react-router 7) : les deux avis de sécurité signalés sur l'ancienne version sont éteints.

## Sous le capot

- Les tests de l'interface web sont désormais exécutés à chaque build de la chaîne d'intégration et la bloquent en cas d'échec.
- Les tests du serveur attendent correctement l'inscription des clients au lieu de pauses fixes : ils sont plus fiables.

Sans impact sur le déroulement de vos parties.

## Comment mettre à jour

Installez la version v11.2.3 depuis la page des releases (ou via la mise à jour OTA de l'interface d'administration). Les anciens clients restent compatibles.

## Liens

- [Documentation](https://github.com/CCoupel/BuzzMaster/blob/main/docs/ADMIN_GUIDE.md)
- [GitHub Release](https://github.com/CCoupel/BuzzMaster/releases/tag/v11.2.3)
