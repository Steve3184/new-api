/* Copyright (C) 2023-2026 QuantumNous */
import { create } from 'zustand'
import { persist } from 'zustand/middleware'

type ContributionView = '2d' | '3d'

export const useContributionsStore = create<{
  view: ContributionView
  setView: (view: ContributionView) => void
}>()(
  persist((set) => ({ view: '2d', setView: (view) => set({ view }) }), {
    name: 'contributions-view',
    merge: (saved, current) => ({
      ...current,
      view: (saved as { view?: unknown } | null)?.view === '3d' ? '3d' : '2d',
    }),
  })
)
