// #171/F4 — l'équipe a-t-elle "tenté" la question, au sens où proposer
// "+N pts" en plus de "0 pt" a un sens ? Ne décide PAS si le geste de
// crédit est monté (ça reste la phase, cf. AnimPage.jsx `creditEnabled`,
// inchangé) — seulement si un montant positif doit être offert.
//
// SPEEDY/QCM : une équipe n'a "tenté" que si un de ses bumpers a
// effectivement buzzé (même test que getTeamQcmAnswer/handleCredit,
// AnimPage.jsx — le verrou de buzz est PAR ÉQUIPE, #157/T2).
// Tout autre type (ARDOISE, MEMORY, MEMOTION, et tout mode futur) : true
// par défaut — permissif délibérément, pour ne jamais bloquer silencieusement
// un mode qui n'a pas encore de règle dédiée écrite ici.
export function canAwardPoints(question, teamBumpers) {
  const type = question?.TYPE
  if (type === 'SPEEDY' || type === 'QCM') {
    return (teamBumpers || []).some(b => (b.TIME ?? 0) > 0)
  }
  return true
}
