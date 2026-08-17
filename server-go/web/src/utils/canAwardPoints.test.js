import { describe, it, expect } from 'vitest'
import { canAwardPoints } from './canAwardPoints'

// ---------------------------------------------------------------------------
// canAwardPoints — #171/F4 (T5), décision D inversée (plan
// _work/reports/plan-20260816-192400.md §5) : ne décide plus SI l'on affiche
// le contrôle de crédit (toujours affiché désormais, F6), seulement SI
// "+N pts" est proposé à côté de "0 pt" (toujours proposé, #170).
//
// Règle : SPEEDY/QCM -> vrai si au moins un bumper de l'équipe a TIME > 0
// (a buzzé/répondu) ; TOUT AUTRE TYPE (y compris inconnu/absent) -> vrai,
// défaut PERMISSIF délibéré (R4 du plan : un défaut restrictif bloquerait
// silencieusement un mode futur — MEMORY/MEMOTION — le jour de son arrivée).
// ---------------------------------------------------------------------------

describe('canAwardPoints — SPEEDY', () => {
  it('vrai si un bumper de l\'équipe a TIME > 0 (a buzzé)', () => {
    expect(canAwardPoints({ TYPE: 'SPEEDY' }, [{ TIME: 1000 }])).toBe(true)
  })

  it('faux si aucun bumper de l\'équipe n\'a buzzé (TIME absent ou 0)', () => {
    expect(canAwardPoints({ TYPE: 'SPEEDY' }, [{ TIME: 0 }])).toBe(false)
    expect(canAwardPoints({ TYPE: 'SPEEDY' }, [{}])).toBe(false)
  })

  it('faux si l\'équipe n\'a aucun bumper (tableau vide)', () => {
    expect(canAwardPoints({ TYPE: 'SPEEDY' }, [])).toBe(false)
  })

  it('vrai si AU MOINS UN bumper a buzzé parmi plusieurs', () => {
    expect(canAwardPoints({ TYPE: 'SPEEDY' }, [{ TIME: 0 }, { TIME: 500 }])).toBe(true)
  })
})

describe('canAwardPoints — QCM', () => {
  it('vrai si un bumper de l\'équipe a répondu (même test TIME > 0)', () => {
    expect(canAwardPoints({ TYPE: 'QCM' }, [{ TIME: 2000, ANSWER_COLOR: 'RED' }])).toBe(true)
  })

  it('faux si aucun bumper n\'a répondu', () => {
    expect(canAwardPoints({ TYPE: 'QCM' }, [{ TIME: 0 }])).toBe(false)
  })
})

describe('canAwardPoints — défaut permissif (R4)', () => {
  it('ARDOISE : toujours vrai (le contrôle "+N pts"/"0 pt" reste consommé par AnimArdoiseList, pas ce prédicat, mais le défaut reste permissif si jamais appelé)', () => {
    expect(canAwardPoints({ TYPE: 'ARDOISE' }, [])).toBe(true)
  })

  it('MEMORY : toujours vrai (mode pas encore construit, défaut permissif)', () => {
    expect(canAwardPoints({ TYPE: 'MEMORY' }, [])).toBe(true)
  })

  it('MEMOTION : toujours vrai (mode pas encore construit, défaut permissif)', () => {
    expect(canAwardPoints({ TYPE: 'MEMOTION' }, [])).toBe(true)
  })

  it('type totalement inconnu (chaîne arbitraire) : vrai — cas explicite du défaut permissif', () => {
    expect(canAwardPoints({ TYPE: 'UN_TYPE_FUTUR_INCONNU' }, [])).toBe(true)
  })

  it('question sans TYPE (undefined) : vrai', () => {
    expect(canAwardPoints({}, [])).toBe(true)
  })

  it('question null : vrai (ne doit jamais planter, toujours permissif)', () => {
    expect(canAwardPoints(null, [])).toBe(true)
  })
})

describe('canAwardPoints — robustesse des bumpers', () => {
  it('teamBumpers undefined (équipe sans entrée dans bumpersByTeam) ne plante pas — SPEEDY -> faux', () => {
    expect(canAwardPoints({ TYPE: 'SPEEDY' }, undefined)).toBe(false)
  })

  it('teamBumpers undefined pour un type permissif -> vrai quand même', () => {
    expect(canAwardPoints({ TYPE: 'ARDOISE' }, undefined)).toBe(true)
  })
})
