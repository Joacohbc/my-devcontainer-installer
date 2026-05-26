// @ts-check
import tseslint from 'typescript-eslint';

/**
 * Flat config. Single purpose for now: enforce that every cross-module import
 * uses the `@/` absolute alias (mapped to `src/*` in tsconfig.json) instead of
 * relative `./` or `../` paths. See AGENTS.md → "Architecture overview".
 *
 * @type {import('eslint').Linter.Config[]}
 */
export default [
  {
    ignores: ['dist/**', 'node_modules/**'],
  },
  {
    files: ['src/**/*.ts'],
    languageOptions: {
      parser: tseslint.parser,
      parserOptions: {
        sourceType: 'module',
      },
    },
    plugins: {
      '@typescript-eslint': tseslint.plugin,
    },
    rules: {
      // The base rule and the TS extension can conflict; disable base, use the
      // TS one so type-only imports (`import type`) are also checked.
      'no-restricted-imports': 'off',
      '@typescript-eslint/no-restricted-imports': [
        'error',
        {
          patterns: [
            {
              group: ['./*', '../*', './**', '../**'],
              message:
                'Use the absolute "@/..." alias instead of relative imports (./ or ../). See tsconfig.json paths and AGENTS.md.',
            },
          ],
        },
      ],
    },
  },
];
