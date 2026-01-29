# Commande /deploy

Deployer l'application vers un environnement cible.

## Usage

```
/deploy <environnement>
```

## Environnements

| Environnement | Description |
|---------------|-------------|
| `qualif` | Environnement de qualification/staging |
| `prod` | Production |

## Exemples

```
/deploy qualif    # Deploiement en qualification
/deploy prod      # Deploiement en production
```

## Prerequis

### Pour QUALIF
- [ ] Tests QA passes
- [ ] Build reussi

### Pour PROD
- [ ] QUALIF validee
- [ ] Tests complets OK
- [ ] Documentation a jour
- [ ] Confirmation utilisateur

## Workflow QUALIF

```
/deploy qualif
    |
    v
Build --> Push --> Smoke Tests --> Notification
```

## Workflow PROD

```
/deploy prod
    |
    v
Confirmation --> Merge main --> Tag --> CI/CD
    |
    |-- SI OK --> Release Notes --> Monitoring
    |
    |-- SI ECHEC --> Rollback --> Analyse
```

## Rollback

En cas de probleme :
```
/deploy rollback    # Revenir a la version precedente
```

## Agent

Lance l'agent `deploy` defini dans `.claude/agents/deploy.template.md`
