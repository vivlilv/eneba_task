import { createContext, useContext } from 'react'
import type { User } from './api'

export type AppContextValue = {
  user: User | null
  authReady: boolean
  cartCount: number
  onSignedIn: (user: User) => Promise<void>
  onSignedOut: () => Promise<void>
  refreshCartCount: () => Promise<void>
}

export const AppContext = createContext<AppContextValue | null>(null)

export function useApp() {
  const value = useContext(AppContext)
  if (!value) throw new Error('App context missing')
  return value
}
