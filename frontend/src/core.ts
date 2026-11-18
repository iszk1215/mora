export interface Repo {
  id: string
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
  id: string
  hits: number
  lines: number
  entries: CoverageEntry[]
  revision: string
  revision_url: string
  time: string
}

export interface CoverageRepo {
  url: string
  namespace: string
  name: string
}

export type CoverageBlock = [number, number, number]

export interface FileData {
  filename: string
  hits: number
  lines: number
}

export interface UserData {
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
  show_slider?: boolean
  show_toolbox?: boolean
  palette?: string
  y_axes?: YAxisConfig[]
  grid_levels?: number
}

export function normalizeChartConfig(cc: ChartConfig): ChartConfig {
  return cc.x_axis_type ? cc : { ...cc, x_axis_type: 'date' }
}

export interface SeriesConfig {
  value_format?: string
  type?: 'line' | 'bar'
  y_axis_index?: number
}

export interface SeriesModel {
  id: string
  tracker_id: string
  name: string
  data_type: string
  config: string
  position?: number
}

export interface PreviewSeriesModel {
  id?: string
  tracker_id: string
  name: string
  data_type: string
  config: string
}

export interface PreviewSeriesValues {
  series: PreviewSeriesModel
  values: Array<{ id?: string; time: string; value: number }>
}

export interface TrackerResponse {
  id: string
  name: string
  description?: string
  body?: string
  visibility: string
  type: string
  chart_config: string
  owner_name?: string
  created_at?: string
  last_updated_at?: string
  role: string
  liked: boolean
  like_count: number
  series?: PreviewSeriesValues[]
}
