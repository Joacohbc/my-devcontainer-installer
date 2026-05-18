import { build } from 'esbuild';
import { readFileSync } from 'fs';

const pkg = JSON.parse(readFileSync(new URL('./package.json', import.meta.url), 'utf8'));
const version = process.env.VERSION || pkg.version || 'dev';

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
  define: {
    __CLI_VERSION__: JSON.stringify(version),
  },
});
