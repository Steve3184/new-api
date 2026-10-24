import type { Variants } from 'motion/react'

export const EASE_OUT = [0.16, 1, 0.3, 1] as const

export const IN_VIEW = { once: true, amount: 0.25 } as const

/** Blur-to-sharp rise used for every revealed block on preset 2. */
export function riseVariants(reduced: boolean): Variants {
  if (reduced) {
    return { hidden: { opacity: 1 }, visible: { opacity: 1 } }
  }
  return {
    hidden: { opacity: 0, y: 26, filter: 'blur(10px)' },
    visible: {
      opacity: 1,
      y: 0,
      filter: 'blur(0px)',
      transition: { duration: 0.9, ease: EASE_OUT },
    },
  }
}

export function cascadeVariants(reduced: boolean, stagger = 0.09): Variants {
  return {
    hidden: {},
    visible: {
      transition: reduced
        ? { duration: 0 }
        : { delayChildren: 0.08, staggerChildren: stagger },
    },
  }
}
