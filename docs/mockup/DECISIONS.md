# Contraintes de conception

> Contraintes durables issues des refus/corrections de l'utilisateur, par composant.
> Le planner les respecte, QA les vérifie. Tenu par le CDP.

## vplayer_ardoise_keyboard
- En PAUSE (et plus généralement hors saisie), aucun message n'invite le joueur à continuer ou à écrire : afficher l'état de jeu (« Jeu en pause ») et le statut de la réponse (envoi en cours / reçue), rien d'autre. _(v11.2.0, final_ack, #246)_
- Clavier verrouillé et vide : texte indicatif « Aucune réponse », jamais « Votre réponse… ». _(v11.2.0, final_ack, #246)_

## admin_game_ardoise_zone
- L'état de réception d'une réponse est une icône à gauche du texte (colonne fixe), jamais une ligne de pastille supplémentaire : la carte ne grandit pas selon l'état. _(v11.2.0, receipt_icon, #246)_
- La légende de l'en-tête (« ✓ reçue · ⏳ en attente · ⚠ hors ligne ») reste la référence des icônes. _(v11.2.0, receipt_icon, #246)_

## anim_team_award
- Attribution groupée sur l'anim : non retenue en v11.2 (#244/#251 admin seulement). Maquette brouillon `anim_team_award__selection` non archivée. _(v11.2.0, #244/#251)_

## admin_new_game
- Cocher « Joueurs des tablettes » coche et verrouille « Scores » : on ne peut pas supprimer les joueurs en gardant les scores. _(v11.2.0, purge_popup_details, #243, D2)_
- L'option « Questions » (remettre toutes les questions à jouer) est cochée par défaut ; décochée, les questions déjà jouées le restent. _(v11.2.0, purge_popup_details, #243, D1)_

## admin_scores
- Le bandeau « Correction cachée » dit que le classement de la TV est mis à jour sans animation (décision utilisateur 2026-10-11). L'ancien libellé de la maquette (« … se mettra à jour au prochain affichage normal ») est périmé. _(v11.2.0, free_points_tv_toggle, #252)_

## admin_game_award
- Le commutateur « Tenir compte des indices » n'apparaît que s'il y a eu des indices (QCM avec indices activés, au moins un indice révélé, montant > 0). Il est activé par défaut, remis à activé à chaque question ou phase, et n'est modifié ni par les clics sur les réponses ni par la saisie. _(v11.2.0, compact_block_hints_toggle / team_or_player_level_hints_toggle, #254)_
- La ligne 1 du bloc est liée au commutateur. Activé : « barème de la question, par équipe » ; avec une valeur saisie à la main : « montant saisi, ajusté selon les indices de chaque équipe ». Désactivé : montant uniforme. _(v11.2.0, #254)_
- La pénalité de mauvaise réponse est toujours uniforme : le commutateur est masqué et les indices sont sans effet. _(v11.2.0, #254)_

## admin_questions_list
- « Sans manche » se joue en premier. _(v11.2.0, rounds_entractes_delete_choice, #245)_
- Supprimer une manche ouvre une fenêtre Conserver / Supprimer aussi / Annuler : les entractes suivent le même sort que les questions. _(v11.2.0, rounds_entractes_delete_choice, #245)_
- Le repli des manches est mémorisé par navigateur. _(v11.2.0, rounds_entractes_delete_choice, #245)_

## game_round_indicator
- Une entracte est proposée par « à suivre » comme toute entrée. _(v11.2.0, admin_anim_tv_next_entracte, #245)_
- L'anim peut terminer une entracte ; déclencher une entracte manuelle reste réservé à l'admin. _(v11.2.0, admin_anim_tv_next_entracte, #245)_
- Les compteurs de manche ne comptent pas les entractes ; le compteur global de l'anim les compte. _(v11.2.0, admin_anim_tv_next_entracte, #245)_

