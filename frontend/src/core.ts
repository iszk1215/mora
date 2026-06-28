export interface Repo {
  id: number
  url: string
  namespace: string
  name: string
}

export interface CoverageEntry {
  name: string
  hits: number
  lines: number
}

export interface Coverage {
  index: number
  hits: number
  lines: number
  entries: CoverageEntry[]
  revision: string
  revision_url: string
  time: string
}

export type CoverageBlock = [number, number, number]

export interface FileData {
  filename: string
  hits: number
  lines: number
}

export interface UserData {
  id: number
  provider: string
  provider_user_id: string
  username: string
  avatar_url: string
}
