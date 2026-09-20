import type { LanguageFn } from 'highlight.js'

declare module 'highlight.js/lib/languages/*' {
  const language: LanguageFn
  export default language
}