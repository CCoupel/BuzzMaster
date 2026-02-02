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

**Contexte projet :** Voir `context/COMMON.md` section 1
**Versionnement :** Voir `context/COMMON.md` section 5

**Input utilisateur :** $ARGUMENTS

**Livrables :**
- Release notes : docs/releases/v[X.Y.Z].md
- Posts réseaux sociaux (Twitter, LinkedIn)
- Newsletter (si major)

**Ton par type :**
- Major (x.0.0) : Très enthousiaste
- Minor (x.y.0) : Modéré
- Patch (x.y.z) : Calme, rassurant
```

## Action immédiate

Lance maintenant le sous-agent marketing-release avec le Task tool.
