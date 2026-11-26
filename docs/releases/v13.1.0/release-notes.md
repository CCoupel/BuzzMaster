# Release v13.1.0 - Rafale Picto

**Date** : 2026-11-25

## Nouveautés

### 🖍️ Rafale Picto : le plus de mots possible
Une équipe fait deviner **un maximum de mots** pendant le temps de la question, avec le même dessinateur pour toute la manche. Chaque « Trouvé » ou « Passer » amène un nouveau mot ; la TV affiche un grand compteur de mots trouvés.

- **Passes** : le dessinateur l'annonce à voix haute, l'animateur la déclenche. Coût de 0 à 100 points par passe, limite à 1 passe ou illimitées.
- **Fin de pool** : reboucler sur les mots passés, ou finir la manche. Un mot trouvé ne revient jamais.
- **Crédit pré-rempli** : mots trouvés × points − passes × coût (jamais en dessous de 0), modifiable.

### 📚 Pool Picto et génération par IA
Un réservoir de mots dessinables, filtrable par catégorie et difficulté, avec saisie d'une liste de 1 à 200 mots et « ✨ Générer via IA » (10, 20 ou 50 mots). Le tirage est stratifié par catégorie et difficulté, et la catégorie du mot s'affiche comme indice. Le Pool Picto rejoint la sauvegarde « Pools ».

### 🏆 Gagnants présélectionnés au crédit
À la révélation, la page Jeu coche d'office les équipes ou joueurs que la question désigne (Picto, Mémoire, Rafale, QCM, Speedy, Ardoise, Memotion). Une chip **« +N pts »** montre le montant à créditer. Rien n'est attribué tant que l'animateur ne clique pas sur « Attribuer les points ».

## Améliorations et corrections

- Le compte à rebours 3-2-1 / GO! s'affiche aussi sur la tablette du dessinateur Picto.
- « Rafale » devient « Rafale Speedy » dans l'interface.
- Sécurité : mise à jour de `golang.org/x/net` en v0.62.0.

## Liens

- [GitHub Release](https://github.com/CCoupel/BuzzMaster/releases/tag/v13.1.0)
- [Changelog](../../../CHANGELOG.md)
