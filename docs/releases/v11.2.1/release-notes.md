# Release v11.2.1 - Des points en feu d'artifice et des réponses tablette confirmées

**Date** : 2026-10-13

## Nouveautés

### Des points en feu d'artifice
Quand l'animateur donne des points à plusieurs équipes en une fois, la TV les célèbre par lots de 6 équipes au plus,
rangées dans une grille 3 × 2 : une fusée par équipe, des étincelles, et chaque carte reste lisible, sans
recouvrement. Chaque équipe voit sur sa tablette uniquement son propre gain, au moment où son lot passe sur la TV.
Les noms trop longs sont tronqués proprement. Pour les personnes qui préfèrent éviter les animations, un simple
fondu remplace le feu d'artifice. Une correction de score cachée n'anime rien.

### Réponses tablette confirmées
Une réponse QCM ou un buzz SPEEDY n'est confirmé (« ✓ Réponse reçue », « ✓ Buzz reçu ») qu'une fois le serveur
certain de l'avoir reçu. En cas de coupure WiFi, la tablette renvoie la réponse d'elle-même (à la reconnexion, au retour
au premier plan, à la reprise après pause, puis régulièrement). Si l'envoi échoue définitivement, le joueur le voit
(« ✕ … non reçue »). Le retournement d'une carte MEMORY / MEMOTION suit le même principe : un geste renvoyé ne
retourne jamais la carte deux fois.

## Corrections

- L'arrêt du serveur par `/shutdown` ne produit plus d'erreur de fermeture et se termine proprement.
- Un appui sur un buzzer physique pendant une ARDOISE ne fige plus les tablettes ni n'affiche de pause fantôme.
- La zone de réponse de l'animateur se révèle correctement pendant la révélation d'une carte MEMOTION.
- Le bouton « REPRISE » de la barre de navigation est mieux annoncé par les lecteurs d'écran.
- Stabilisation de tests internes (sans impact sur les joueurs).

## Comment mettre à jour

Installez la version v11.2.1 depuis la page des releases (ou via la mise à jour OTA de l'interface d'administration).
Les anciens clients restent compatibles : ils ignorent simplement les nouveaux champs.

## Liens

- [Documentation](https://github.com/CCoupel/BuzzMaster/blob/main/docs/ADMIN_GUIDE.md)
- [GitHub Release](https://github.com/CCoupel/BuzzMaster/releases/tag/v11.2.1)
