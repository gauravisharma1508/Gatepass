import tseslint from 'typescript-eslint'
import boundaries from 'eslint-plugin-boundaries'
import reactHooks from 'eslint-plugin-react-hooks'

// Module boundaries (spec §3.4):
//  - a module may import another module only through that module's index.ts
//  - shared/ imports no module and no app code
//  - only app/ may import every module
export default tseslint.config(
  { ignores: ['dist', 'node_modules'] },
  ...tseslint.configs.recommended,
  {
    files: ['src/**/*.{ts,tsx}'],
    plugins: { boundaries, 'react-hooks': reactHooks },
    settings: {
      'import/resolver': { typescript: { project: './tsconfig.json' } },
      'boundaries/include': ['src/**/*'],
      'boundaries/ignore': ['src/main.tsx', 'src/vite-env.d.ts'],
      'boundaries/elements': [
        { type: 'app', pattern: 'src/app' },
        { type: 'module', pattern: 'src/modules/*', capture: ['name'] },
        { type: 'shared', pattern: 'src/shared' },
        { type: 'test', pattern: 'src/test' },
      ],
      'boundaries/files': [{ pattern: 'src/modules/*/index.ts', category: 'module-entry' }],
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      'react-hooks/set-state-in-effect': 'off',
      'react-hooks/purity': 'off',
      '@typescript-eslint/no-explicit-any': 'error',
      'boundaries/dependencies': [
        'error',
        {
          default: 'disallow',
          policies: [
            { from: { element: { type: 'app' } }, allow: { to: { element: { types: ['app', 'module', 'shared'] } } } },
            { from: { element: { type: 'shared' } }, allow: { to: { element: { type: 'shared' } } } },
            { from: { element: { type: 'module' } }, allow: { to: { element: { type: 'shared' } } } },
            // inside one module anything goes ...
            { from: { element: { type: 'module' } },
              allow: { to: { element: { type: 'module', captured: { name: '{{ from.element.captured.name }}' } } } } },
            // ... other modules only through their index.ts
            { from: { element: { type: 'module' } },
              allow: { to: { element: { type: 'module' }, file: { categories: 'module-entry' } } } },
            { from: { element: { type: 'test' } }, allow: { to: { element: { types: ['app', 'module', 'shared', 'test'] } } } },
          ],
        },
      ],
    },
  },
)
