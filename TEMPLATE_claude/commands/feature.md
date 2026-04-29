# Commande /feature

Workflow complet pour l'implementation d'une nouvelle fonctionnalite.

## Usage

```
/feature <description de la fonctionnalite>
```

## Argument recu

$ARGUMENTS

## Mots-cles de controle

**Reference :** Voir `context/COMMON.md` section 12

| Mot-cle | Action |
|---------|--------|
| `help` | Affiche l'aide et les mots-cles disponibles |
| `status` | Affiche l'etat du workflow en cours |
| `plan` | Affiche le plan sans executer |
| `resume <phase>` | Reprend a une phase |
| `skip <phase>` | Saute une phase |
| `jumpto <tache>` | Demarre a une tache precise du plan |

Si `$ARGUMENTS` commence par un mot-cle -> executer l'action correspondante.
Sinon -> workflow normal.

## Workflow

```
/feature <description>
    |
    v
[CLARIFICATION] --> Etude spec + issues GitHub + questions si besoin
    |
    v
[PLAN] --> Plan d'implementation + contrats API
    |
    v
[DEV] --> Implementation (agents tech selon stack)
    |           |
    v           v
[REVIEW]   [TEST-WRITER] --> (en parallele)
    \           /
     v         v
      [QA] --> Execution scripts + procedures manuelles
    |
    v
[DOC] --> Documentation
    |
    v
[DEPLOY] --> Deploiement QUALIF (PROD sur /deploy prod)
```

## Prompt a transmettre au CDP

Orchestre le workflow FEATURE pour {PROJECT_NAME}.

**Contexte projet :** Voir `context/COMMON.md` section 1
**Workflow CDP :** Voir `context/CDP_WORKFLOWS.md`
- Type : FEATURE
- Phases : section 3
- Clarification : section 4
- Dispatch DEV : section 5
- Validation : section 6
- Erreurs : section 7
- Regles : section 9

**Contexte DEV :** Voir `context/DEVELOPMENT.md`
**Contexte Qualite :** Voir `context/QUALITY.md`

**Demande utilisateur :** $ARGUMENTS

## Exemples

```
/feature Ajouter l'authentification OAuth2
/feature Implementer la page de profil utilisateur
/feature Creer l'endpoint API pour les notifications
/feature Ajouter le support du mode sombre
```

## Sortie Anticipee

A tout moment, l'utilisateur peut :
- Demander de passer une etape
- Arreter le workflow
- Modifier le plan

## Agent

Delegue au CDP (`cdp.md`) qui orchestre les agents specialises.
