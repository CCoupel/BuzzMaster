# Site Marketing BuzzControl

Ce répertoire contient les fichiers du site marketing public de BuzzControl.

## Structure recommandée

```
site/
├── index.html              # Page d'accueil
├── features.html           # Liste des fonctionnalités
├── download.html           # Page de téléchargement
├── releases.html           # Liste des releases
├── css/
│   └── style.css          # Styles du site
├── img/
│   ├── hero.jpg           # Image principale
│   ├── screenshots/       # Captures d'écran
│   └── features/          # Icônes et images features
└── js/
    └── main.js            # Scripts interactifs
```

## Contenu géré par l'agent MARKETING

Quand l'agent MARKETING est appelé après un déploiement PROD, il met à jour :
- La version affichée sur la page d'accueil
- Les nouvelles features sur la page features.html
- Les liens de téléchargement
- Les release notes publiques

## Ton de communication

- ✅ Accessible, convivial, sans jargon technique
- ✅ Bénéfices utilisateur avant les détails techniques
- ✅ Visuels attractifs et GIFs démo
- ❌ Pas de termes backend (GameState, WebSocket, etc.)

## Exemples de titre

- ✅ "Transformez vos soirées quiz avec BuzzControl"
- ✅ "Nouveau : Mode Memory pour animer vos équipes"
- ❌ "Extension du GameState avec MemoryCurrentTeam"
