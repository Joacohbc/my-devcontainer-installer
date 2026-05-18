import { build } from 'esbuild';

await build({
  entryPoints: ['src/index.ts'],
  bundle: true,
  platform: 'node',
  format: 'cjs',
  target: 'node20',
  outfile: 'dist/sea-bundle.cjs',
  external: [],
  legalComments: 'none',
  minify: false,
});
