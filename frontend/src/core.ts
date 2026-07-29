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

export interface YAxisConfig {
  id: number
  label?: string
  min?: number
  max?: number
  position: 'left' | 'right'
}

export interface ChartConfig {
  x_axis_label?: string
  x_axis_type?: 'date' | 'datetime'
  area?: boolean
  show_legend?: boolean
  show_symbols?: boolean
  palette?: string
  y_axes?: YAxisConfig[]
}

export interface SeriesConfig {
  value_format?: string
  type?: 'line' | 'bar'
  y_axis_index?: number
}

export interface SeriesModel {
  id: number
  tracker_id: number
  name: string
  data_type: string
  config: string
}

export interface TrackerResponse {
  id: number
  name: string
  description?: string
  visibility: string
  type: string
  repo_id?: number
  chart_config: string
  role: string
  liked: boolean
  like_count: number
}
