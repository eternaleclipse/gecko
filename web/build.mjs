import * as esbuild from 'esbuild';
import { cpSync, mkdirSync, rmSync, writeFileSync } from 'node:fs';

const watch = process.argv.includes('--watch');
rmSync('dist', { recursive: true, force: true });
mkdirSync('dist', { recursive: true });
writeFileSync('dist/.keep', '');
cpSync('public', 'dist', { recursive: true });
cpSync('src/index.html', 'dist/index.html');

const opts = {
  entryPoints: ['src/main.js'],
  bundle: true,
  minify: !watch,
  sourcemap: watch,
  format: 'esm',
  target: ['es2022', 'safari16'],
  outdir: 'dist',
  loader: { '.woff2': 'file', '.woff': 'file' },
  assetNames: 'assets/[name]-[hash]',
  logLevel: 'info',
};
if (watch) {
  const ctx = await esbuild.context(opts);
  await ctx.watch();
} else {
  await esbuild.build(opts);
}
