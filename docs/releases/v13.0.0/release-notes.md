# Release v13.0.0 - Picto : dessinez, l'équipe devine

**Date** : 24 novembre 2026

## Nouveautés

### Un nouveau type de question : ✏️ Picto
Un joueur dessine un mot sur sa tablette, son équipe le devine à l'oral. Le tracé apparaît en direct sur la TV, chez l'admin et chez l'animateur.

### Préparer la manche
L'animateur choisit l'équipe qui dessine, puis le dessinateur : à la main parmi les joueurs connectés, ou tiré au hasard (rejouable tant que la manche n'a pas démarré). Le lancement demande un dessinateur connecté.

### Un mot secret
À chaque lancement, un mot est tiré parmi ceux de la question. Seuls le dessinateur, l'admin et l'animateur le voient : jamais la TV, ni les autres tablettes.

### Fin de manche et crédit
« Trouvé ! » arrête et révèle la manche en une seule action, y compris après « Temps écoulé ». Le temps écoulé, l'arrêt manuel et l'absence de mot terminent aussi la manche. La TV et les tablettes montrent alors le mot. « Trouvé ! » ne donne pas de points : l'animateur crédite ensuite l'équipe, avec les points de la question pré-remplis et ajustables. La question peut être relancée, et les LED des buzzers allument l'équipe qui dessine.

### Un éditeur dédié
De 1 à 20 mots (un par ligne), une durée de 30 à 300 secondes (90 par défaut), des points et une catégorie, avec un résumé avant l'enregistrement.

## Sécurité

Une tablette joueur ne peut plus reprendre une place avec son seul identifiant : chaque tablette reçoit un jeton de session à l'inscription et à chaque reconnexion. Les places créées avant la v13.0.0 sont prises par la première tablette qui s'y connecte. Les buzzers physiques ne sont pas concernés.

## Améliorations

La page Jeu reprend désormais le temps et les points de la question, pour tous les types.

## Comment mettre à jour

Installer la v13.0.0 ; vos questions et équipes existantes sont conservées. Gardez les tablettes joueurs sur un réseau séparé de l'administration (voir le guide d'administration, « Sécurité »).

## Liens

- [GitHub Release](https://github.com/CCoupel/BuzzMaster/releases/tag/v13.0.0)
