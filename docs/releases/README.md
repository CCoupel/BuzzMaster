# Release Notes Publiques

Ce répertoire contient les release notes publiques de BuzzControl.

## Format des fichiers

Chaque release a son propre fichier Markdown :
```
releases/
├── v2.40.0.md
├── v2.39.0.md
├── v2.38.0.md
└── ...
```

## Structure d'une release note

```markdown
# BuzzControl v[X.Y.Z] - [Nom de code]

**Date de sortie** : AAAA-MM-JJ

## 🎉 Quoi de neuf ?

### [Icône] [Nom de la feature principale]

[Description accessible de ce que ça fait]

**Bénéfice** : [Ce que ça apporte aux utilisateurs]

![Screenshot ou GIF](../site/img/screenshots/feature.gif)

---

### [Autres features]

[...]

## 🛠️ Améliorations

- [Liste des améliorations]

## 🐛 Corrections

- [Bugs corrigés]

## 📥 Téléchargement

[Lien vers la release]
```

## Différence avec CHANGELOG.md

| CHANGELOG.md | Release Notes Publiques |
|--------------|------------------------|
| Technique | Grand public |
| Liste exhaustive | Features principales uniquement |
| Noms de fichiers/fonctions | Bénéfices utilisateur |
| Pour développeurs | Pour utilisateurs finaux |

## Exemples de ton

**CHANGELOG.md** (technique) :
```markdown
### Added
- **Memory**: Add MemoryMode field in Question model
  - Support for SOLO, CHACUN_SON_TOUR, TANT_QUE_JE_GAGNE, MAILLON_FAIBLE
  - Add MemoryCurrentTeam in GameState for team rotation
```

**Release Notes** (public) :
```markdown
### 🎮 Modes de jeu Memory

Le jeu Memory devient multi-joueurs ! Quatre nouveaux modes pour animer vos équipes :

- **Solo** : Chaque équipe joue seule, la plus rapide gagne
- **Chacun son tour** : Les équipes jouent à tour de rôle
- **Tant que je gagne** : Continuez tant que vous trouvez des paires !
- **Maillon faible** : Mode élimination, restez concentré !

**Bénéfice** : Adaptez le jeu à votre ambiance (compétition intense ou coopération)
```

## Contenu généré par l'agent MARKETING

L'agent MARKETING crée automatiquement ces fichiers après chaque déploiement PROD en se basant sur :
1. Le CHANGELOG.md (pour les changements techniques)
2. Le CLAUDE.md (pour les détails si besoin)
3. Les templates de communication définis dans `.claude/agents/marketing.md`

L'utilisateur peut ensuite réviser/adapter le contenu avant publication.
