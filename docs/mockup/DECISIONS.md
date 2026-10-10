# Contraintes de conception

> Contraintes durables issues des refus/corrections de l'utilisateur, par composant.
> Le planner les respecte, QA les vérifie. Tenu par le CDP.

## vplayer_ardoise_keyboard
- En PAUSE (et plus généralement hors saisie), aucun message n'invite le joueur à continuer ou à écrire : afficher l'état de jeu (« Jeu en pause ») et le statut de la réponse (envoi en cours / reçue), rien d'autre. _(v11.2.0, final_ack, #246)_
- Clavier verrouillé et vide : texte indicatif « Aucune réponse », jamais « Votre réponse… ». _(v11.2.0, final_ack, #246)_

## admin_game_ardoise_zone
- L'état de réception d'une réponse est une icône à gauche du texte (colonne fixe), jamais une ligne de pastille supplémentaire : la carte ne grandit pas selon l'état. _(v11.2.0, receipt_icon, #246)_
- La légende de l'en-tête (« ✓ reçue · ⏳ en attente · ⚠ hors ligne ») reste la référence des icônes. _(v11.2.0, receipt_icon, #246)_
