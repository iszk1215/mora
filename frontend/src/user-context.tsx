import { createContext, useContext } from 'react'
import { UserData } from './core'

const UserContext = createContext<UserData | null>(null)

export const UserProvider = UserContext.Provider

export function useUser(): UserData | null {
  return useContext(UserContext)
}
