# JavaScript / TypeScript and browser-automation tool caches

Raw facts for the `ccc` registry contract. 27 tools are covered. Verification labels: `official-docs` (read on the vendor page listed in `sources` during this research), `ran-command` (confirmed by running a read-only command), `unverified` (written from memory or only inferred). Nothing in this file was cleaned or deleted; every clean command below is documented, not executed. Tools that could not be run (yarn, deno, fnm, volta, n, cypress, electron-builder) have discover commands that are `official-docs` at best. Where a vendor page did not state a default path, the field is marked `unverified` and the doc gap is stated.

---

## npm
- ecosystem: javascript
- detect: `npm` on PATH; or `~/.npm` exists.
- scope: global
- locations:
  1. cache `_cacache` (content-addressable tarballs + index) — discover: `npm config get cache` (prints the cache root, `~/.npm` by default; real cache is `<that>/_cacache`); override: env `npm_config_cache` / `NPM_CONFIG_CACHE`, `.npmrc` key `cache`, `--cache`; macOS default `~/.npm`; Linux default `~/.npm`.
  2. `_npx` (npx-installed package trees) — same parent as cache (`<cache>/_npx`); same override.
  3. `_logs` (debug logs) — `<cache>/_logs`; separate override `logs-dir` config key (unverified); same default parent.
- measure: `du -sk "$(npm config get cache)"` (covers all three). Native: none for size (`npm cache ls` lists entries only).
- clean: full wipe: `npm cache clean --force` (docs: `clean` requires `--force`). Does not clearly remove `_npx` / `_logs` (docs say nothing about them) -> for those, `delete directory` of `<cache>/_npx` and `<cache>/_logs`. Prune variant: `npm cache verify` (removes unneeded/corrupt data, offline). Needs npm installed.
- regenerable: yes (re-downloaded). `_npx` re-fetched at next `npx`.
- in_use_hazard: concurrent `npm install` may fail with cache integrity/ENOENT errors; npm cache is concurrency-safe by design but deleting the dir mid-install is not.
- sources: https://docs.npmjs.com/cli/v10/commands/npm-cache
- verification: discover `ran-command`; default paths `official-docs` (macOS/Linux `~/.npm`; `_logs`/`_npx` names `ran-command`, seen under `~/.npm`); clean `official-docs` (`--force` requirement); `logs-dir` key `unverified`.

## Yarn Classic (v1)
- ecosystem: javascript
- detect: `yarn` on PATH with `yarn --version` starting `1.`.
- scope: global
- locations:
  1. global package cache — discover: `yarn cache dir`; override: env `YARN_CACHE_FOLDER`, `yarn config set cache-folder`, `--cache-folder`, `.npmrc` `cache=`; macOS default `~/Library/Caches/Yarn` (unverified; docs only say "in your user directory"); Linux default `~/.cache/yarn` (unverified; Next.js CI doc lists `$HOME/.cache/yarn`).
- measure: `du -sk "$(yarn cache dir)"`.
- clean: `yarn cache clean` (all) or `yarn cache clean <module...>`. No prune variant. Needs yarn installed.
- regenerable: yes.
- in_use_hazard: running install concurrently can fail on missing tarballs.
- sources: https://classic.yarnpkg.com/en/docs/cli/cache
- verification: discover `official-docs` (not run, yarn absent); default paths `unverified`; clean `official-docs`.

## Yarn Berry (v2+)
- ecosystem: javascript
- detect: `yarn --version` >= 2, or `.yarnrc.yml` / `.yarn/` in project; global dir `~/.yarn/berry`.
- scope: global (cache + metadata) AND per-project (`.yarn/cache` when `enableGlobalCache: false`, `.yarn/` internals); per-project marker: `.yarnrc.yml` or `package.json` with `packageManager: yarn@2+`. NOTE: `.yarn/cache` is often committed to git (zero-installs); must not be wiped blindly.
- locations:
  1. global folder `globalFolder` (global cache under `<globalFolder>/cache`, metadata cache) — discover: `yarn config get globalFolder`; override: `globalFolder` in rc, env `YARN_GLOBAL_FOLDER`; default (all OS) `~/.yarn/berry` (docs state `${HOME}/.yarn/berry`; no macOS/Linux difference).
  2. project cache `.yarn/cache` — discover: `yarn config get cacheFolder`; override env `YARN_CACHE_FOLDER`; used only when `enableGlobalCache` is false (default true).
- measure: `du -sk` of discovered paths.
- clean: `yarn cache clean` removes local cache by default; `--mirror` removes the global cache; `--all` removes both. No prune variant. Needs yarn (berry) installed. Local `.yarn/cache` may be committed -> deleting shows as git changes.
- regenerable: yes (re-downloaded; with zero-install the committed cache is the offline source).
- in_use_hazard: with PnP + zero-install, wiping `.yarn/cache` makes the project unrunnable until `yarn install`.
- sources: https://yarnpkg.com/features/caching , https://yarnpkg.com/cli/cache/clean , https://yarnpkg.com/configuration/yarnrc
- verification: discover `official-docs`; default paths `official-docs` (`~/.yarn/berry`, `./.yarn/cache`); clean `official-docs`; env var `YARN_GLOBAL_FOLDER` by naming rule `official-docs`.

## pnpm
- ecosystem: javascript
- detect: `pnpm` on PATH.
- scope: global
- locations:
  1. content-addressable store — discover: `pnpm store path` (prints the store path, e.g. `~/Library/pnpm/store/v10` on macOS; note the version suffix `v10` inside storeDir); override: `storeDir` config / `--store-dir`, env `PNPM_HOME` (-> `$PNPM_HOME/store`), `XDG_DATA_HOME` (-> `$XDG_DATA_HOME/pnpm/store`); macOS default `~/Library/pnpm/store`; Linux default `~/.local/share/pnpm/store`. Falls back to `<volume root>/.pnpm-store` or `node_modules/.pnpm-store` when home is on a different filesystem than the project (store is hard-linked into projects).
  2. metadata/dlx cache — discover: `pnpm config get cache-dir` prints `undefined` when the key is not set (the default is not printed); override: `cacheDir` setting, `XDG_CACHE_HOME`; macOS default `~/Library/Caches/pnpm`; Linux default `~/.cache/pnpm`. Holds package metadata, `pnpm dlx` cache, install verification results.
- measure: `du -sk "$(pnpm store path)"`. Caveat: files are hard-linked into projects, so du overstates reclaimable space; unreferenced-only size is not reported without pruning. `pnpm store status` checks integrity, not size.
- clean: store: `pnpm store prune` (removes packages no project references; docs say safe). No documented full-wipe command -> `delete directory` (breaks nothing permanently but projects with hard links keep their files; node_modules remain intact because they are hard links, but pnpm may need `pnpm install` to repopulate the store). Cache dir: `delete directory` (no native command documented).
- regenerable: yes (re-downloaded).
- in_use_hazard: deleting the store while an install runs breaks it; existing `node_modules` keep working (hard links) but `pnpm install --offline` / reinstall needs refetch.
- sources: https://pnpm.io/cli/store , https://pnpm.io/settings/store , https://pnpm.io/settings/other
- verification: discover `ran-command` (store path); `config get cache-dir` `ran-command` (undefined); default paths `official-docs`; clean `official-docs` (prune) / `unverified` for "no full wipe command" beyond the doc not describing one.

## Bun (install cache)
- ecosystem: javascript
- detect: `bun` on PATH; `~/.bun` exists.
- scope: global
- locations:
  1. install cache — discover: `bun pm cache` (must run inside a directory with package.json: from `/tmp` it errored `No package.json was found`; docs describe it as printing the path); override: env `BUN_INSTALL_CACHE_DIR`, `bunfig.toml` `[install.cache] dir`; macOS default `~/.bun/install/cache`; Linux default `~/.bun/install/cache` (docs give one path, no per-OS split; `BUN_INSTALL` env may also relocate `~/.bun`, unverified).
- measure: `du -sk` of path from `bun pm cache`.
- clean: `bun pm cache rm` (full wipe; no flags documented). No prune variant. Needs bun installed.
- regenerable: yes.
- in_use_hazard: concurrent install may fail; bun hard-links/clonefiles from cache into node_modules, so existing node_modules unaffected.
- sources: https://bun.sh/docs/install/cache , https://bun.sh/docs/pm/cli/pm
- verification: discover `ran-command` (failure mode observed: requires package.json cwd) + `official-docs`; default paths `official-docs`; clean `official-docs`.

## Deno
- ecosystem: javascript/typescript
- detect: `deno` on PATH.
- scope: global
- locations:
  1. DENO_DIR (remote modules `deps/`, compiled TS `gen/`, npm packages, etc.) — discover: `deno info` prints "DENO_DIR location" (`deno info --json` also has `denoDir`, unverified); override: env `DENO_DIR`; macOS default `~/Library/Caches/deno` (docs example); Linux default `~/.cache/deno` (inferred from sample output, docs do not state it).
- measure: `du -sk` of DENO_DIR.
- clean: `deno clean` (full wipe; `--dry-run` preview); prune variant: `deno clean --except <entrypoint...>` keeps data needed by the named entry points. Needs deno installed.
- regenerable: yes (re-downloaded; offline projects lose offline ability).
- in_use_hazard: running `deno run` / LSP may fail to resolve cached modules; `--cached-only` setups break.
- sources: https://docs.deno.com/runtime/reference/cli/clean/ , https://docs.deno.com/runtime/reference/cli/info/
- verification: discover `official-docs`; default paths macOS `official-docs` (example only), Linux `unverified`; clean `official-docs`.

## Corepack
- ecosystem: javascript
- detect: `corepack` on PATH (shipped in the Node bin directory, e.g. under the nvm-managed node).
- scope: global
- locations:
  1. COREPACK_HOME (downloaded yarn/pnpm shims' package manager versions) — discover: `echo "${COREPACK_HOME:-$HOME/.cache/node/corepack}"` (no native print command found); override: env `COREPACK_HOME`; macOS default `~/.cache/node/corepack`; Linux default `~/.cache/node/corepack` (README: all non-Windows systems).
- measure: `du -sk` of path.
- clean: `corepack cache clean` (README also lists `corepack cache clear`, same description). Needs corepack installed.
- regenerable: yes (re-downloaded on next use).
- in_use_hazard: low; a running `yarn`/`pnpm` shim would re-download.
- sources: https://github.com/nodejs/corepack#readme
- verification: discover `unverified` (derived); default paths `official-docs`; clean `official-docs`.

## nvm (installed Node versions + cache)
- ecosystem: javascript (version manager)
- detect: `$NVM_DIR` set (shell function, not on PATH: `command -v nvm` is empty in non-interactive shells); directory `~/.nvm` exists.
- scope: global
- locations:
  1. installed Node versions — `$NVM_DIR/versions/node/vX.Y.Z` (inferred from README paths); discover: `nvm ls` (shell function, needs `. "$NVM_DIR/nvm.sh"`); override: env `NVM_DIR`; default `~/.nvm` (or `$XDG_CONFIG_HOME/nvm` when set at install); same on macOS and Linux. These are USER-CHOSEN toolchains, not a cache: removal changes what `node` resolves to.
  2. download cache `$NVM_DIR/.cache` — discover: `echo "$NVM_DIR/.cache"` (docs do not mention `nvm cache dir`); same defaults.
- measure: `du -sk "$NVM_DIR/versions/node"/* "$NVM_DIR/.cache"`.
- clean: cache: `nvm cache clear` (docs). Versions: `nvm uninstall <version>` per version; no bulk prune. Both require nvm sourced into a shell (cannot run from a plain exec of `ccc` without `bash -c '. $NVM_DIR/nvm.sh && ...'`). Uninstalling the active/default version breaks the shell's `node`.
- regenerable: cache yes; versions `yes-but-large-download` and only if the user remembers which versions they need (global npm packages installed under a version are LOST, they live in `versions/node/<v>/lib/node_modules`).
- in_use_hazard: uninstalling a version in use by a running process/dev server; global CLIs installed for that version vanish.
- sources: https://github.com/nvm-sh/nvm#readme
- verification: discover `unverified` for cache path; `nvm ls` `official-docs` (not run, shell function); default paths `official-docs` (`~/.nvm`; `~/.nvm/versions/node` `ran-command`: per-version directories); clean `official-docs`.

## fnm
- ecosystem: javascript (version manager)
- detect: `fnm` on PATH ; dir `~/.local/share/fnm` or `~/Library/Application Support/fnm`.
- scope: global
- locations:
  1. fnm dir (Node versions under `node-versions/`, aliases) — discover: `fnm env` output contains `FNM_DIR` (docs do not state this); override: env `FNM_DIR` (existence from memory), `XDG_DATA_HOME`; macOS default `~/Library/Application Support/fnm`; Linux default `~/.local/share/fnm` (or `$XDG_DATA_HOME/fnm`).
- measure: `du -sk` of dir.
- clean: per-version `fnm uninstall <version>` (command exists per fnm docs/commands.md; not on README, so `unverified`); whole tool: `delete directory`. No cache-only clean.
- regenerable: `yes-but-large-download`; user-chosen toolchains, global npm packages lost.
- in_use_hazard: removing the active version breaks `node` in open shells.
- sources: https://github.com/Schniz/fnm#readme
- verification: discover `unverified`; default paths `official-docs`; clean `unverified` for `fnm uninstall`.

## Volta
- ecosystem: javascript (toolchain manager)
- detect: `volta` on PATH; `~/.volta` exists.
- scope: global
- locations:
  1. VOLTA_HOME (`tools/` inventory + image of Node/npm/yarn versions, `cache/` downloaded archives, shims, `bin/`) — discover: `echo "${VOLTA_HOME:-$HOME/.volta}"` (derived); override: env `VOLTA_HOME`; default `~/.volta` on macOS and Linux (from memory).
- measure: `du -sk "$VOLTA_HOME/tools" "$VOLTA_HOME/cache"` (unverified subdirectory names).
- clean: no cache-clean command documented; `volta uninstall <tool>` removes installed tools. Cache subdir: `delete directory`. Do not delete `bin/` (shims).
- regenerable: cache yes; installed tools `yes-but-large-download`; `VOLTA_HOME/bin` shims are needed for the tool to work.
- in_use_hazard: removing tools pinned by projects makes Volta re-download on next use.
- sources: https://docs.volta.sh/guide/understanding
- verification: all path/discover fields `unverified` (docs page did not state them); clean `unverified`.

## n
- ecosystem: javascript (version manager)
- detect: `n` on PATH; `/usr/local/n` exists.
- scope: global
- locations:
  1. version cache `<N_CACHE_PREFIX or N_PREFIX>/n/versions` — discover: `n ls` lists cached versions; path derived; override: env `N_PREFIX` (moves install root, default `/usr/local`), `N_CACHE_PREFIX` (moves downloads); default `/usr/local/n/versions` (docs: "with the default prefix") on both macOS and Linux (Homebrew-installed `n` uses same).
- measure: `du -sk /usr/local/n/versions`.
- clean: prune: `n prune` (removes all cached versions except the installed one); specific: `n rm <version>`; `n uninstall` removes installed Node, leaves cache. May require `sudo` when prefix is `/usr/local`.
- regenerable: yes-but-large-download (but the installed version is retained by prune).
- in_use_hazard: `n rm` of the installed version breaks `node`.
- sources: https://github.com/tj/n#readme
- verification: discover `official-docs`; default paths `official-docs`; clean `official-docs`.

## node-gyp (headers/devdir cache)
- ecosystem: javascript (native addons)
- detect: dir exists (node-gyp is bundled with npm; `node-gyp` rarely on PATH).
- scope: global
- locations:
  1. devdir (Node headers per version) — discover: none native; derive from env/default; override: `--devdir`, `node_gyp_devdir` in package.json `config`; macOS default `~/Library/Caches/node-gyp` (`ran-command`: per-version directories); Linux default `~/.cache/node-gyp` (unverified; docs say only "OS cache directory").
- measure: `du -sk` of path.
- clean: `node-gyp remove <version>` removes headers for a version (docs mention, syntax unverified); otherwise `delete directory`.
- regenerable: yes (re-downloaded at next native build; requires network).
- in_use_hazard: an in-flight `npm install` compiling an addon fails if headers vanish mid-build.
- sources: https://github.com/nodejs/node-gyp#readme
- verification: discover `unverified`; default paths macOS `ran-command`, Linux `unverified`; clean `unverified` for exact syntax.

## Electron download cache (@electron/get)
- ecosystem: javascript
- detect: dir exists (no binary).
- scope: global
- locations:
  1. Electron zip download cache — discover: none native; override: env `ELECTRON_CACHE` / `electron_config_cache` (unverified, README did not mention it); macOS default `~/Library/Caches/electron/`; Linux default `$XDG_CACHE_HOME` if set else `~/.cache/electron/`.
- measure: `du -sk`.
- clean: `delete directory`.
- regenerable: yes-but-large-download (~100 MB per version/arch).
- in_use_hazard: low; running `npm install electron` re-downloads.
- sources: https://github.com/electron/get#readme
- verification: discover `unverified`; default paths `official-docs`; clean `unverified` (no command exists as far as docs show).

## electron-builder cache
- ecosystem: javascript
- detect: dir exists.
- scope: global
- locations:
  1. builder cache (winCodeSign, nsis, appimage, fpm tools) — discover: none native; override: env `ELECTRON_BUILDER_CACHE`; macOS default `~/Library/Caches/electron-builder`; Linux default `~/.cache/electron-builder`.
- measure: `du -sk`.
- clean: `delete directory`.
- regenerable: yes-but-large-download.
- in_use_hazard: an in-flight `electron-builder` build fails.
- sources: attempted https://www.electron.build/ pages; the fetched pages gave no content.
- verification: ALL fields `unverified` (from memory; docs pages 404 or empty).

## Playwright browsers
- ecosystem: browser-automation
- detect: `playwright` bin (via `npx playwright`) or dir exists (macOS `~/Library/Caches/ms-playwright`).
- scope: global (plus one nuance: per-project `node_modules/playwright*` are separate).
- locations:
  1. browser binaries (chromium-NNNN, firefox-NNNN, webkit-NNNN, ffmpeg-NNNN, headless shell) — discover: `echo "${PLAYWRIGHT_BROWSERS_PATH:-<default>}"` (no native path command; `npx playwright install --list` lists browsers from all installations); override: env `PLAYWRIGHT_BROWSERS_PATH` (value `0` = inside node_modules); macOS default `~/Library/Caches/ms-playwright`; Linux default `~/.cache/ms-playwright`.
  2. `ms-playwright-go` (Go bindings driver) — `~/Library/Caches/ms-playwright-go` (macOS); its role/clean command `unverified`.
- measure: `du -sk` (native: `npx playwright install --list` lists but gives no sizes — sizes are `unverified`).
- clean: `npx playwright uninstall` (browsers of the current installation); `npx playwright uninstall --all` (every installation). Prune: Playwright auto-removes unused versions on install (disable with `--no-remove` or `PLAYWRIGHT_SKIP_BROWSER_GC=1`). Needs a Playwright install reachable (npx may download playwright itself).
- regenerable: yes-but-large-download (hundreds of MB per browser).
- in_use_hazard: running tests/MCP servers launching the browser fail with "Executable doesn't exist"; installed-version pinning means other projects need `npx playwright install` again.
- sources: https://playwright.dev/docs/browsers
- verification: discover `official-docs`; default paths `official-docs` + macOS `ran-command`; clean `official-docs`.

## Puppeteer browsers
- ecosystem: browser-automation
- detect: dir `~/.cache/puppeteer` (contains `chrome-headless-shell` among other downloads).
- scope: global
- locations:
  1. browser cache (`chrome/`, `chrome-headless-shell/`, `firefox/`) — discover: none native; override: `cacheDirectory` in `.puppeteerrc.*`, env `PUPPETEER_CACHE_DIR` (`unverified`, docs page did not mention); default (all OS incl. macOS and Linux) `~/.cache/puppeteer`.
- measure: `du -sk`.
- clean: `delete directory` (no native command documented; `npx @puppeteer/browsers clear` is not verified and must not be used).
- regenerable: yes-but-large-download.
- in_use_hazard: running scripts/MCP tools fail "Could not find Chrome".
- sources: https://pptr.dev/guides/configuration
- verification: discover `unverified`; default paths `official-docs` + `ran-command` (dir exists); clean `unverified` (none exists in docs read).

## Cypress binary cache
- ecosystem: browser-automation / testing
- detect: `cypress` bin (project-local) or dir exists.
- scope: global
- locations:
  1. binary cache `<cache>/<version>` — discover: `cypress cache path` (needs cypress CLI, e.g. `npx cypress cache path`); override: env `CYPRESS_CACHE_FOLDER`; macOS default `~/Library/Caches/Cypress`; Linux default `~/.cache/Cypress`.
- measure: `cypress cache list --size`.
- clean: full wipe: `cypress cache clear`; prune: `cypress cache prune` (deletes all versions except the current one). Needs cypress installed (project npm package).
- regenerable: yes-but-large-download (~500 MB per version).
- in_use_hazard: running `cypress run/open` fails with binary-not-found.
- sources: https://docs.cypress.io/app/references/advanced-installation
- verification: discover `official-docs`; default paths `official-docs`; clean `official-docs`.

## Turborepo (local cache)
- ecosystem: javascript
- detect: `turbo.json` in project (and `turbo` bin).
- scope: per-project; marker: `turbo.json` sibling of `.turbo/`; when `cacheDir` unset, docs mention Git worktree cache sharing so the path may be shared (unverified where).
- locations:
  1. task cache `.turbo/cache` — discover: none native; override: `cacheDir` in `turbo.json`, `--cache-dir`; default (all OS) `<project>/.turbo/cache`.
- measure: `du -sk`.
- clean: `delete directory` (docs read describe no clear command).
- regenerable: yes (rebuilt by re-running tasks; may be slow).
- in_use_hazard: running `turbo` task writes cache; deleting mid-run loses just that entry.
- sources: https://turborepo.dev/docs/crafting-your-repository/caching
- verification: discover `unverified`; default paths `official-docs`; clean `unverified`.

## Nx
- ecosystem: javascript
- detect: `nx.json` in project.
- scope: per-project; marker: `nx.json`.
- locations:
  1. local task cache — discover: none native; override: `cacheDirectory` in `nx.json` (docs: default `~/.nx/<id>/cache`, shared by every checkout of the workspace, in newer Nx; older Nx used `.nx/cache` / `node_modules/.cache/nx`, unverified); also env `NX_CACHE_DIRECTORY` (not in docs read, unverified). Default therefore depends on Nx version: a registry entry must resolve it per project.
  2. workspace data / daemon state — `.nx/workspace-data` (unverified path), cleaned by reset.
- measure: `du -sk` of resolved path (resolution rule uncertain).
- clean: `nx reset` (all: clears cache, workspace data, stops daemon); granular: `nx reset --onlyCache`, `--onlyDaemon`, `--onlyWorkspaceData`, `--onlyCloud`. Remote cache is not touched.
- regenerable: yes.
- in_use_hazard: daemon is shut down; running `nx` command may be interrupted.
- sources: https://nx.dev/docs/reference/nx-commands , https://nx.dev/docs/reference/nx-json
- verification: discover `unverified`; default paths `official-docs` for the newer default only; clean `official-docs`.

## Next.js (`.next`)
- ecosystem: javascript
- detect: `next` in `package.json` / `next.config.*`.
- scope: per-project; marker: `package.json` (with `next`) or `next.config.{js,mjs,ts}` sibling of `.next`.
- locations:
  1. `.next` build output (incl. `.next/cache`: webpack/swc/image cache) — discover: none; override: `distDir` in `next.config.*`; default `<project>/.next`.
- measure: `du -sk`.
- clean: `delete directory` (`.next/cache` only for cache-only; whole `.next` also removes build output).
- regenerable: yes (rebuilt by `next dev` / `next build`).
- in_use_hazard: deleting `.next` under a running `next dev` breaks the server (500s, missing chunks) until restarted; a deployed `next start` from that dir breaks.
- sources: https://nextjs.org/docs/app/guides/ci-build-caching
- verification: discover n/a; default paths `official-docs` for `.next/cache`; `distDir` `unverified`; clean `unverified` (no native command).

## Vite
- ecosystem: javascript
- detect: `vite.config.*` or `vite` in `package.json`.
- scope: per-project; marker: `package.json` sibling of `node_modules/.vite`.
- locations:
  1. dep pre-bundle cache `node_modules/.vite` — discover: none; override: `cacheDir` in vite config; default `<project>/node_modules/.vite` (`.vite` if no package.json).
- measure: `du -sk`.
- clean: `delete directory`, or run `vite --force` to re-bundle (docs).
- regenerable: yes.
- in_use_hazard: deleting under a running dev server causes optimized-dep 504/outdated errors until restart.
- sources: https://vite.dev/config/shared-options.html
- verification: default paths `official-docs`; clean `official-docs`.

## webpack / babel-loader / other `node_modules/.cache` users
- ecosystem: javascript
- detect: `node_modules/.cache` directory exists.
- scope: per-project; marker: `node_modules/.cache` inside a dir with `package.json`.
- locations:
  1. `node_modules/.cache/webpack` — override: `cache.cacheDirectory`; default `node_modules/.cache/webpack`.
  2. `node_modules/.cache/babel-loader` — override: `cacheDirectory` loader option (default off; true -> this path; falls back to OS temp dir if no node_modules).
  3. other tools (terser-webpack-plugin, eslint-loader, `find-cache-dir`-based tools, `.cache/prettier`, `.cache/@babel`) writing to `node_modules/.cache/<tool>` — names `unverified`.
- measure: `du -sk node_modules/.cache`.
- clean: `delete directory` (webpack/babel docs describe no clean command).
- regenerable: yes.
- in_use_hazard: running watch builds may rebuild from scratch; absolute paths are stored so moved projects invalidate anyway.
- sources: https://webpack.js.org/configuration/cache/ , https://github.com/babel/babel-loader#readme
- verification: default paths `official-docs` (webpack, babel-loader); other users `unverified`; clean `unverified`.

## Parcel
- ecosystem: javascript
- detect: `parcel` in `package.json`, `.parcelrc`.
- scope: per-project; marker: `package.json` sibling of `.parcel-cache`.
- locations:
  1. `.parcel-cache` — override: `--cache-dir <path>`; default `.parcel-cache` in project root.
- measure: `du -sk`.
- clean: `delete directory`.
- regenerable: yes.
- in_use_hazard: `parcel watch/serve` running holds the LMDB cache open; deleting under it can crash or corrupt the watcher until restart.
- sources: https://parceljs.org/features/cli/
- verification: default paths `official-docs`; clean `unverified` (LMDB hazard from memory).

## ESLint cache
- ecosystem: javascript
- detect: `.eslintcache` file exists.
- scope: per-project; marker: `package.json` / eslint config sibling of `.eslintcache`.
- locations:
  1. cache file `.eslintcache` — override: `--cache-location <file|dir>`; default `.eslintcache` in cwd where eslint runs (dir form creates `.cache_<hash of cwd>`).
- measure: `du -sk` / file size.
- clean: `delete directory` (a file); docs note running ESLint without `--cache` deletes the file.
- regenerable: yes.
- in_use_hazard: none significant.
- sources: https://eslint.org/docs/latest/use/command-line-interface
- verification: default paths `official-docs`; clean `official-docs`.

## Jest cache
- ecosystem: javascript
- detect: `jest` in `package.json` / `jest.config.*`.
- scope: global-ish (default cacheDirectory is under the OS temp dir, e.g. `/tmp/jest_*`, `unverified`) with per-project override.
- locations:
  1. Jest cacheDirectory — discover: `npx jest --showConfig` (read `cacheDirectory`; docs-confirmed); override: `cacheDirectory` config / `--cacheDirectory`; default path `unverified` (docs say find it via `--showConfig`; macOS typically `/private/var/folders/.../jest_*`).
- measure: `du -sk` of the `cacheDirectory` value.
- clean: `npx jest --clearCache` (deletes cacheDirectory and exits). Needs jest installed in the project. Docs warn performance drops.
- regenerable: yes.
- in_use_hazard: jest `--watch` running slows/recomputes.
- sources: https://jestjs.io/docs/cli
- verification: discover `official-docs`; default paths `unverified`; clean `official-docs`.

## TypeScript incremental build info
- ecosystem: typescript
- detect: `tsconfig.json`; `*.tsbuildinfo` files.
- scope: per-project; marker: `*.tsbuildinfo` next to `tsconfig*.json` / in `outDir`.
- locations:
  1. `*.tsbuildinfo` — override: `tsBuildInfoFile`, `outDir`; default `tsconfig.tsbuildinfo` (name/placement rules from memory, doc page was truncated); discover: `tsc --showConfig` (existence of key unverified).
  2. `~/Library/Caches/typescript` (TypeScript Automatic Type Acquisition cache, ; `ran-command`) — Linux `~/.cache/typescript` (unverified); role `unverified`; clean `delete directory`.
- measure: `du -sk`.
- clean: `tsc --build --clean` (removes outputs incl. buildinfo; may remove emitted JS/d.ts, not just cache -> destructive for committed build output; documented only from memory) else `delete directory` of the `.tsbuildinfo` file.
- regenerable: yes (full re-typecheck).
- in_use_hazard: `tsc --watch` rebuilds.
- sources: https://www.typescriptlang.org/tsconfig/ (page truncated, relevant sections not read)
- verification: all `unverified` except the macOS `typescript` cache dir which `ran-command` exists.

## Project `node_modules`
- ecosystem: javascript
- detect: directory named `node_modules` containing... (walk projects).
- scope: per-project; marker: sibling `package.json`. Skip nested `node_modules` inside `node_modules`.
- locations:
  1. `<project>/node_modules` — no discover; override: pnpm `virtual-store-dir`, Yarn PnP (no node_modules), Deno `--node-modules-dir` (unverified); default `<project>/node_modules`.
- measure: `du -sk` (hard-link/clonefile layouts (pnpm, bun) overstate reclaimable space).
- clean: `delete directory` (reinstall via the project's manager: `npm ci`, `pnpm install`, etc.). No native "clean node_modules" command. Not applicable to read-only perms in general, but pnpm store hard links share inodes (deleting is safe).
- regenerable: yes-but-large-download; fails when offline or when lockfile missing/registry package withdrawn (`npm ci` needs lockfile).
- in_use_hazard: running dev servers, IDE language servers, git hooks (husky) break; locally patched dependencies (patch-package without committed patches) or `npm link`ed packages are lost.
- sources: https://docs.npmjs.com/cli/v10/commands/npm-ci (not fetched)
- verification: all `unverified` as documentation was not read.

---

## Contract observations

1. Path discoverable only by running the tool: npm (`npm config get cache`), pnpm (`pnpm store path`), Yarn (`yarn cache dir`, `yarn config get globalFolder`), Bun (`bun pm cache`, which must run in a directory with a `package.json`), Cypress (`cypress cache path`), Jest (`jest --showConfig`), Deno (`deno info`). The contract needs a `discover` command with a working-directory requirement and a fallback chain: tool command -> env var -> OS default path.
2. More than one location per tool, with different semantics: npm (`_cacache`, `_npx`, `_logs`), pnpm (store vs cache-dir), Yarn Berry (global vs `.yarn/cache`), nvm (versions vs `.cache`), Nx (cache vs workspace data). Each location needs its own measure/clean/regenerable/hazard fields, and the native clean command may cover only some of them (`npm cache clean --force` does not document `_npx`/`_logs`).
3. Per-project instances found by walking directories with a marker file: `node_modules` (`package.json`), `.next`, `node_modules/.vite`, `node_modules/.cache`, `.parcel-cache`, `.turbo/cache`, `.nx`, `.eslintcache`, `*.tsbuildinfo`, `.yarn/cache`. The registry needs marker rules, walk-skip rules (do not descend into `node_modules`), and a way to tell committed content from cache (Yarn Berry zero-install `.yarn/cache`, possibly committed `.tsbuildinfo`).
4. Clean actions of several kinds: a native full-wipe (`npm cache clean --force`, `deno clean`, `bun pm cache rm`, `cypress cache clear`, `nx reset`, `jest --clearCache`), a native prune-unused (`pnpm store prune`, `cypress cache prune`, `n prune`, `deno clean --except`, `npm cache verify`), per-item removal (`nvm uninstall <v>`, `n rm <v>`), flags that change the scope (`yarn cache clean --mirror/--all`, `nx reset --only*`) and `delete directory` with no native command (Puppeteer, Electron, Turborepo, Vite, Next.js, webpack). The contract must express "native command preferred, else delete" and the flag-selected scope.
5. Clean command requires the tool installed and sometimes a project context or a shell function: `cypress cache clear` / `jest --clearCache` need the project package, nvm is a shell function (`. $NVM_DIR/nvm.sh`), `n` may need sudo for `/usr/local`, Playwright needs `npx`. Fallback when the tool is absent (e.g. browser caches left behind after the package was removed) is `delete directory`.
6. Size not measurable by `du` alone: pnpm/bun/Yarn PnP use hard links or clonefile, so `du` overstates reclaimable space; some tools have a native size command (`cypress cache list --size`), others only list (`npx playwright install --list`). The contract needs `measure.kind = du | command` and a flag for "du overstates".
7. Version-manager "caches" (nvm, fnm, Volta, n versions, Node installs) are user-selected toolchains that also hold global npm packages; they must be marked separately from true caches (`regenerable: yes-but-large-download`, user state) and need a "protect active/default version" rule.
8. In-use hazards that depend on running processes (`next dev`, `vite`, `parcel watch`, `jest --watch`, `nx` daemon, Playwright MCP servers); the registry needs a `blocking_processes` or similar hint plus a note field.
9. Defaults that vary by version of the tool and by environment, so a hardcoded default is risky: Nx default cache dir changed to `~/.nx/<id>/cache` in newer versions vs `.nx/cache`; pnpm store location depends on `PNPM_HOME`, `XDG_DATA_HOME` and on the filesystem the project lives on (`.pnpm-store` fallback); versioned subdir `v10` appended; `XDG_CACHE_HOME` overrides on Linux for Electron, pnpm; Jest's default is temp-dir-based.
10. Directories with the cache's parent shared with non-cache data: `~/.npm` (config in `~/.npmrc` is outside, but `_update-notifier-last-checked` is inside), `~/.bun` (holds the bun binary itself in `~/.bun/bin`, so removing `~/.bun` removes the tool), `~/.volta` (contains `bin/` shims), `~/.yarn/berry` (global folder holds config-ish state as well as cache). Clean must target the sub-path, never the parent.
