# Menu déroulant sur la pastille de connexion

**Statut** : 📋 Planifié

## Description

Transformer la pastille de connexion dans la navbar (qui affiche le nombre de clients Admin/TV connectés) en un bouton cliquable qui ouvre un menu déroulant. Ce menu contiendra les liens vers Configuration et Logs, permettant de simplifier la barre de navigation principale.

## Objectifs

- [ ] Simplifier la navbar en réduisant le nombre de liens visibles
- [ ] Regrouper les fonctions utilitaires (Config, Logs) dans un menu secondaire
- [ ] Conserver l'affichage du compteur de connexions visible en permanence

## Design proposé

### État actuel
```
[Jeu] [Scores] [Équipes] [Questions] [Historique] [Palmarès] [Config] [Logs]  (🔵2 📺1)
```

### État cible
```
[Jeu] [Scores] [Équipes] [Questions] [Historique] [Palmarès]  [🔵2 📺1 ▼]
                                                                    │
                                                              ┌─────┴─────┐
                                                              │ ⚙️ Config  │
                                                              │ 📋 Logs   │
                                                              └───────────┘
```

## Tâches

### Phase 1 - Menu déroulant
- [ ] Transformer la pastille en bouton cliquable
- [ ] Créer le composant menu déroulant
- [ ] Déplacer les liens Config et Logs dans le menu
- [ ] Fermeture au clic extérieur

### Phase 2 - Styles
- [ ] Style du menu cohérent avec le reste de l'UI
- [ ] Animation d'ouverture/fermeture
- [ ] Indicateur visuel que c'est cliquable (hover, curseur)

## Fichiers concernés

| Fichier | Modification |
|---------|--------------|
| `web/src/components/Navbar.jsx` | Ajout menu déroulant |
| `web/src/components/Navbar.css` | Styles du menu |

## Version cible

v2.47.0
