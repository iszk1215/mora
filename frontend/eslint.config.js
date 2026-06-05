import tseslint from 'typescript-eslint'
import n from 'eslint-plugin-n'
import promise from 'eslint-plugin-promise'

export default tseslint.config(
  { ignores: ['dist/**'] },
  ...tseslint.configs.recommended,
  n.configs['flat/recommended'],
  promise.configs['flat/recommended'],
  {
    rules: {
      'n/no-missing-import': 'off',
      'n/no-unsupported-features/node-builtins': 'off',
      'promise/catch-or-return': 'warn',
      '@typescript-eslint/no-explicit-any': 'warn',
      '@typescript-eslint/no-unused-vars': ['warn', { argsIgnorePattern: '^_' }],
    },
  },
)
