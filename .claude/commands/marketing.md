# Commande /marketing - Communication de Release

Lance le sous-agent MARKETING pour créer les contenus de communication d'une nouvelle version.

## Argument reçu (optionnel)

$ARGUMENTS

**Formats possibles** :
- `/marketing` : Auto-détecte la version actuelle
- `/marketing 2.40.0` : Version spécifique
- `/marketing 2.40.0 PROD` : Version + environnement
- `/marketing "Mode Memory multi-équipes"` : Focus sur une feature

## Instructions

Utilise le Task tool pour lancer le sous-agent marketing-release avec les paramètres suivants :

```
subagent_type: "marketing-release"
description: "Créer contenus marketing"
prompt: voir ci-dessous
```

### Prompt à transmettre au sous-agent

```
Crée les contenus de communication pour une release BuzzControl.

**Contexte projet :**
- Répertoire : C:\Users\cyril\Documents\VScode\buzzcontrol
- Config version : server-go/config.json
- Changelog : CHANGELOG.md
- Release notes : docs/releases/
- Site marketing : docs/site/ (si existant)

**Input utilisateur :** $ARGUMENTS

**Étapes à exécuter :**

1. **Collecter les informations de release**
   - Version : lire server-go/config.json → "version"
   - Changelog : lire CHANGELOG.md → section de la version
   - Type de release :
     - x change → Major (breaking changes)
     - y change → Minor (nouvelles features)
     - z change → Patch (bug fixes)

2. **Analyser le changelog**
   - Nouvelles fonctionnalités (🎉)
   - Corrections de bugs (🐛)
   - Améliorations (💡)
   - Changements breaking (⚠️)

3. **Produire les contenus marketing**

   | Section | Contenu |
   |---------|---------|
   | 📊 Informations | Version, date, type, code name créatif |
   | 🌐 Site Web | Fichiers à mettre à jour |
   | 📝 Release Notes | Notes publiques user-friendly en français |
   | 📱 Réseaux Sociaux | Posts Twitter, LinkedIn, Reddit |
   | 📧 Newsletter | Email si release majeure |
   | ✅ Checklist | Vérifications finales |

4. **Créer les fichiers**
   - Release notes : docs/releases/v[X.Y.Z].md
   - Mettre à jour site marketing si existant

**Structure des release notes :**
```markdown
# BuzzControl v[X.Y.Z] - [Code Name Créatif]

**Date de sortie** : [Date en français]

## 🎉 Nouveautés
### [Emoji] [Nom de la feature]
[Description accessible, non-technique en français]
**Avantage** : [Ce que ça apporte aux utilisateurs]

## 🐛 Corrections
- [Corrections formulées positivement]

## 💡 Améliorations
- [Améliorations de performance, UI, UX]

## 🚀 Comment mettre à jour
[Instructions simples]
```

**Contenus réseaux sociaux :**
- Twitter/X : Max 280 caractères, emojis, hashtags #BuzzControl #QuizGame
- LinkedIn : Plus détaillé, ton professionnel
- Reddit : Technique mais accessible, invite au feedback

**Ton et style :**
- Langue : Français principalement
- Ton : Enthousiaste mais professionnel
- Public : Organisateurs de quiz, animateurs, éducateurs
- Emojis : Utilisés stratégiquement

**Niveau d'enthousiasme :**
| Type | Ton |
|------|-----|
| Major (x.0.0) | 🎉 Très enthousiaste, transformation majeure |
| Minor (x.y.0) | 😊 Modéré, focus sur les améliorations |
| Patch (x.y.z) | 😌 Calme, rassurant sur la stabilité |

**Checklist finale :**
- Numéros de version corrects et cohérents
- Dates au format français (22 janvier 2026)
- Descriptions accessibles (pas de jargon technique)
- Posts réseaux sociaux dans les limites de caractères
- Code name créatif et mémorable
```

## Action immédiate

Lance maintenant le sous-agent marketing-release avec le Task tool.
