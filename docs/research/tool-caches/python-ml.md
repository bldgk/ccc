# Python, data science and ML tool caches (plus R and Julia)

This file records raw, source-checked facts about where 29 Python/ML/R/Julia tools keep caches and other storage, how to discover those paths, measure them, and clean them with each tool's own command. "ran-command" means the fact was confirmed by running a read-only command; tools that could not be run are `official-docs` or `unverified`. Several official doc URLs (pipx, virtualenv, tox, Jupyter paths) returned 404 during fetching; those fields are marked `unverified` rather than filled from memory. Key facts: some tools put user-built environments next to caches (Poetry, PDM, Hatch, conda, pipx, pyenv, uv tools/pythons), some have paths only the tool can print, and several tools use a different default path on macOS than on Linux (pip, Poetry, Hatch, Jupyter, renv, matplotlib) while uv uses `~/.cache/uv` on both.

Verification vocabulary: `official-docs`, `ran-command`, `unverified`.

---

## pip
- name: pip; ecosystem: Python
- detect: `pip` / `pip3` / `python -m pip` on PATH.
- locations:
  - HTTP response cache + locally built wheels (one cache root; subdirs `http-v2` (pip >= 23.3), `http` (older), `wheels`)
    - discover: `pip cache dir` (prints e.g. `~/Library/Caches/pip` on macOS); `pip cache info` prints per-subdir paths and sizes
    - override: env `PIP_CACHE_DIR`, CLI `--cache-dir <dir>`; `--no-cache-dir` / `PIP_NO_CACHE_DIR` disables. Pip config file key `cache-dir` under `[global]` follows the generic option mapping (unverified for config-file form).
    - macOS default: `~/Library/Caches/pip` (docs say pip 26.2 will also respect `XDG_CACHE_HOME` on macOS)
    - Linux default: `~/.cache/pip` (respects `XDG_CACHE_HOME`)
- scope: global
- measure: `pip cache info` (reports http and wheels size separately) or `du -sk "$(pip cache dir)"`
- clean:
  - full wipe: `pip cache purge` (clears both HTTP and wheel caches)
  - prune variant: `pip cache remove <pattern>` removes matching wheels only (glob or package name); it does NOT delete HTTP cache files. No "unused only" prune exists.
  - notes: internal layout is explicitly "an implementation detail" that may change between versions, so prefer the command over `rm -rf`. Different pip installs (different Pythons) can resolve to the same dir.
- regenerable: yes (re-downloaded; locally built wheels get rebuilt, which can be slow for C extensions)
- in_use_hazard: low; a concurrent `pip install` may hit a missing cache entry and re-fetch. No daemon.
- sources: https://pip.pypa.io/en/stable/topics/caching/ ; https://pip.pypa.io/en/stable/cli/pip_cache/ ; https://pip.pypa.io/en/stable/cli/pip/
- verification: discover = ran-command; default paths = official-docs (macOS path also ran-command); clean = official-docs

## uv
- name: uv; ecosystem: Python (Astral)
- detect: `uv` on PATH.
- locations:
  1. Cache (wheels, sdists, git checkouts, HTTP cache, built wheels, archive store; also centralized project environments that `prune` removes)
     - discover: `uv cache dir` (prints e.g. `~/.cache/uv`)
     - override: env `UV_CACHE_DIR`, CLI `--cache-dir`, config `tool.uv.cache-dir`; `XDG_CACHE_HOME` moves the default; `--no-cache` uses a temp dir
     - macOS default: `~/.cache/uv` (NOT `~/Library/Caches`; confirmed by `uv cache dir`)
     - Linux default: `$XDG_CACHE_HOME/uv` or `~/.cache/uv`
  2. Managed Python installs (interpreters downloaded by uv)
     - discover: `uv python dir` (prints e.g. `~/.local/share/uv/python`; the directory may not exist until a Python is installed)
     - override: env `UV_PYTHON_INSTALL_DIR`
     - macOS/Linux default: `$XDG_DATA_HOME/uv/python` or `~/.local/share/uv/python`
     - shims/executables: `uv python dir --bin` (prints e.g. `~/.local/bin`; override `UV_PYTHON_BIN_DIR`)
  3. Tool installs (isolated venvs created by `uv tool install`)
     - discover: `uv tool dir` (prints e.g. `~/.local/share/uv/tools`); executables: `uv tool dir --bin` (override `UV_TOOL_BIN_DIR`)
     - override: env `UV_TOOL_DIR`
     - macOS/Linux default: `$XDG_DATA_HOME/uv/tools` or `~/.local/share/uv/tools`
- scope: global (all three). Per-project `.venv` created by uv: see "project venv / .venv".
- measure: cache: `uv cache size` (prints bytes; experimental, emits a warning) or `du -sk "$(uv cache dir)"`. Pythons/tools: `du` of discovered dir.
- clean:
  - cache full wipe: `uv cache clean`; one package: `uv cache clean <package>`
  - cache prune: `uv cache prune` (unused entries and centralized project envs); `uv cache prune --ci` (drops pre-built wheels and unzipped sdists, keeps wheels built from source)
  - managed pythons: `uv python uninstall <version>` (not in fetched docs; unverified); tools: `uv tool uninstall <name>` (unverified)
  - notes: docs say it is "never safe to modify the cache directly" (do not rm it by hand). Cache-modifying commands take a lock and wait up to 5 min (`UV_LOCK_TIMEOUT`); `--force` ignores the lock (only when no uv process is running). `--preview-features cache-physical-space` gives a more accurate reclaimed-space estimate (macOS/Linux).
- regenerable: cache = yes; managed Pythons = yes-but-large-download (re-download of interpreters, and venvs that point to them break until recreated); tool installs = no for the installed set (user chose tools; reinstall needed)
- in_use_hazard: cache: locking makes concurrent clean block, not corrupt. Removing a managed Python breaks every venv whose `pyvenv.cfg` `home` points at it. Removing tools removes installed CLIs from `~/.local/bin`.
- sources: https://docs.astral.sh/uv/concepts/cache/ ; https://docs.astral.sh/uv/reference/storage/
- verification: discover = ran-command; default paths = official-docs + ran-command; clean (`cache clean`/`prune`/`--ci`) = official-docs; `python uninstall`, `tool uninstall` = unverified

## Poetry
- name: Poetry; ecosystem: Python
- detect: `poetry` on PATH.
- locations:
  1. Cache dir (downloaded artifacts, repository metadata)
     - discover: `poetry config cache-dir`
     - override: env `POETRY_CACHE_DIR`; config key `cache-dir`
     - macOS default: `~/Library/Caches/pypoetry`
     - Linux default: `$XDG_CACHE_HOME/pypoetry` or `~/.cache/pypoetry`
  2. Virtualenvs (built environments, one per project+python)
     - discover: `poetry config virtualenvs.path`; for the current project `poetry env info --path`; all of a project's envs `poetry env list --full-path`
     - override: env `POETRY_VIRTUALENVS_PATH`, config `virtualenvs.path`; `virtualenvs.in-project=true` / `POETRY_VIRTUALENVS_IN_PROJECT` puts them in `<project>/.venv` instead
     - default (both OSes): `{cache-dir}/virtualenvs`, i.e. inside the cache dir
- scope: cache global; virtualenvs global (default) or per-project (`.venv` when in-project)
- measure: `du` of `poetry config cache-dir` (the virtualenvs live INSIDE it, so du mixes cache and envs; size cache subdirs separately, excluding `virtualenvs`). `poetry cache list` lists repository caches by name only.
- clean:
  - cache: `poetry cache clear --all` is documented as `poetry cache clear --all` (all repos) or `poetry cache clear <repo> --all`; single entry `poetry cache clear PyPI:requests:2.24.0`
  - virtualenvs: `poetry env remove --all` (removes all managed venvs of the CURRENT project only); `poetry env remove <python>`
  - notes: no global "remove every project's venvs" command; deleting `{cache-dir}` wholesale also deletes all virtualenvs.
- regenerable: cache = yes; virtualenvs = no — they are built environments someone installed (though recreatable via `poetry install` if the lock file exists and network is available)
- in_use_hazard: deleting a venv in use by an IDE/running process breaks it; Poetry projects whose venv dir vanishes are silently re-created on next `poetry install` (slow).
- sources: https://python-poetry.org/docs/configuration/ ; https://python-poetry.org/docs/cli/
- verification: discover = official-docs (`poetry config <key>` documented; not run); default paths = official-docs; clean = official-docs

## Pipenv
- name: Pipenv; ecosystem: Python
- detect: `pipenv` on PATH.
- locations:
  1. Pipenv cache (pip + resolver caches)
     - discover: `pipenv --support` shows env vars; no dedicated cache-dir command found (unverified)
     - override: env `PIPENV_CACHE_DIR`
     - macOS/Linux default: `~/.cache/pipenv` (docs: Unix/Linux/macOS)
  2. Virtualenvs
     - discover: `pipenv --venv` (per project; unverified, not in fetched page)
     - override: `WORKON_HOME`; `PIPENV_VENV_IN_PROJECT=1` creates `.venv` in the project (default 0)
     - default: `~/.local/share/virtualenvs` (unverified)
- scope: cache global; venvs global or per-project (`.venv`)
- measure: `du` of the paths
- clean: cache: `pipenv --clear` (clears caches; unverified, docs fetched only mention env var `PIPENV_CLEAR`); venv: `pipenv --rm` (removes the project's venv; unverified)
- regenerable: cache = yes; venv = no (built environment), recreatable from Pipfile.lock
- in_use_hazard: removing a venv breaks running processes/IDE interpreter settings
- sources: https://pipenv.pypa.io/en/latest/configuration.html
- verification: discover = unverified; default paths = official-docs (cache) / unverified (venvs); clean = unverified

## PDM
- name: PDM; ecosystem: Python
- detect: `pdm` on PATH.
- locations:
  1. Cache (wheels, http, hashes, metadata, packages)
     - discover: `pdm config cache_dir` (unverified syntax) / `pdm cache info` (shows location and size)
     - override: env `PDM_CACHE_DIR`; config `cache_dir`
     - default per docs: `~/.cache/pdm` (page does not state macOS vs Linux; PDM uses platformdirs so macOS is likely `~/Library/Caches/pdm` — unverified)
  2. Virtualenvs
     - discover: `pdm venv list`
     - override: config `venv.location`; per-project `.venv` when `venv.in_project` is on
     - default: `~/.local/share/pdm/venvs` (per docs; platform variation unverified)
- scope: cache global; venvs global or per-project
- measure: `pdm cache info` (reports size)
- clean: `pdm cache clear <type>` (type positional; accepted values not confirmed); `pdm cache remove <pattern>`; venvs: `pdm venv remove <env> [-y]`, `pdm venv purge [-f] [-i]` (purge selected or all venvs)
- regenerable: cache = yes; venvs = no (built envs; recreatable from pdm.lock)
- in_use_hazard: as for other venv managers; `pdm venv purge` can remove all envs
- sources: https://pdm-project.org/latest/reference/configuration/ ; https://pdm-project.org/latest/reference/cli/
- verification: discover = official-docs (`pdm cache info`), `pdm config` form unverified; default paths = official-docs (macOS unverified); clean = official-docs

## Hatch
- name: Hatch; ecosystem: Python
- detect: `hatch` on PATH.
- locations:
  1. Cache dir `dirs.cache`
     - override: env `HATCH_CACHE_DIR`, CLI `--cache-dir`
     - macOS: `~/Library/Caches/hatch`; Linux: `$XDG_CACHE_HOME/hatch` or `~/.cache/hatch`
  2. Data dir `dirs.data` (holds envs and managed Pythons)
     - override: env `HATCH_DATA_DIR`, CLI `--data-dir`
     - macOS: `~/Library/Application Support/hatch`; Linux: `$XDG_DATA_HOME/hatch` or `~/.local/share/hatch`
  3. Environments `dirs.env` (default `<DATA_DIR>/env/<ENV_TYPE>`; config-file-only override)
     - discover: `hatch env find [ENV_NAME]`
  4. Managed Pythons `dirs.python` (`isolated` = `<DATA_DIR>/pythons`; config-file-only override)
     - discover: `hatch python show [--dir/-d <dir>]` lists distributions
- scope: global
- measure: `du` of the paths; `hatch config show` can print dirs (unverified)
- clean: envs: `hatch env prune` (removes ALL environments of the current project, per docs "Removes all environments"), `hatch env remove [ENV_NAME]`; pythons: `hatch python remove NAMES...` (`all` removes every distribution). Cache: delete directory.
- regenerable: cache = yes; envs = no (built; recreated on next `hatch run`); pythons = yes-but-large-download
- in_use_hazard: removing pythons breaks envs that link to them
- sources: https://hatch.pypa.io/latest/config/hatch/ ; https://hatch.pypa.io/latest/cli/reference/
- verification: discover = official-docs; default paths = official-docs; clean = official-docs

## pipx
- name: pipx; ecosystem: Python
- detect: `pipx` on PATH.
- locations:
  1. PIPX_HOME: installed app venvs (`venvs/`), shared libs (`shared/`), plus a cache for `pipx run` temporary venvs and logs
     - discover: `pipx environment` (prints resolved PIPX_HOME, PIPX_BIN_DIR, PIPX_SHARED_LIBS, etc.; per search result of official reference; not run)
     - override: env `PIPX_HOME`, `PIPX_SHARED_LIBS` (default `$PIPX_HOME/shared`), `PIPX_BIN_DIR`, `PIPX_VENV_CACHEDIR` (cache of `pipx run` envs; documented only in a man page, default unverified)
     - macOS default: `~/Library/Application Support/pipx` on newer pipx; legacy `~/.local/pipx` is used if it already exists (per search summary of the official reference)
     - Linux default: `~/.local/share/pipx` on newer pipx; legacy `~/.local/pipx` if it exists (unverified for exact value)
- scope: global
- measure: `du` of `PIPX_HOME` (value from `pipx environment --value PIPX_HOME`)
- clean: `pipx uninstall <package>`, `pipx uninstall-all` (names from memory of pipx CLI; unverified); temp-venv cache: delete directory
- regenerable: installed apps = no (user-installed tools, reinstallable); `pipx run` cache and shared libs = yes
- in_use_hazard: uninstalling removes CLI symlinks in `PIPX_BIN_DIR`
- sources: https://pipx.pypa.io/latest/reference/environment-variables/ (was 404 when fetched directly; content seen only through search summary)
- verification: discover = unverified (`pipx environment` is cited by official docs but not run); default paths = unverified (search summary only); clean = unverified

## conda (Anaconda / Miniconda / Miniforge)
- name: conda; ecosystem: Python/multi-language
- detect: `conda` on PATH, or install dir (`~/miniconda3`, `~/anaconda3`, `~/miniforge3`, `/opt/homebrew/Caskroom/miniconda`; unverified).
- locations:
  1. Package cache `pkgs_dirs` (extracted packages + tarballs; envs hard-link/symlink to it)
     - discover: `conda config --show pkgs_dirs` (unverified) or `conda info` ("package cache" field)
     - override: env `CONDA_PKGS_DIRS`; config key `pkgs_dirs`
     - default: `<conda root>/pkgs` and `~/.conda/pkgs` (unverified; docs page fetched did not state it)
  2. Environments `envs_dirs`
     - discover: `conda env list` / `conda info --envs`
     - override: env `CONDA_ENVS_PATH`; config `envs_dirs`
     - default: `<conda root>/envs` and `~/.conda/envs` (unverified)
  3. Index cache (repodata) inside pkgs dir (`cache/`), logs, tempfiles
- scope: global
- measure: `conda clean --all --dry-run` (shows what would be removed, with sizes); `du` of pkgs dir
- clean: `conda clean --all` (index cache, unused packages, tarballs, temp files, logs; does not include `-f`); granular flags `--index-cache`, `--packages` (unused only), `--tarballs`, `--tempfiles`, `--logfiles`; `--force-pkgs-dirs` removes ALL writable package caches and "will break environments with packages installed using symlinks back to the package cache"; `--dry-run`/`-d`, `--yes`/`-y`. Environments: `conda env remove -n <name>` (unverified). Prompts unless `-y`.
- regenerable: pkgs cache = yes-but-large-download; envs = no — user-built environments (data: installed packages/versions chosen by user, possibly packages from private channels)
- in_use_hazard: `--force-pkgs-dirs` and `--packages` break envs not listed in `conda info --envs` (docs warning: unlisted envs are not checked for links back to the cache). Packages are hard-linked, so removing cache of a package used by a running env is generally safe, but symlink/copy modes break.
- sources: https://docs.conda.io/projects/conda/en/stable/commands/clean.html ; https://docs.conda.io/projects/conda/en/stable/user-guide/configuration/use-condarc.html
- verification: discover = unverified; default paths = unverified; clean = official-docs

## mamba / micromamba
- name: mamba / micromamba; ecosystem: Python/multi-language
- detect: `micromamba` or `mamba` on PATH; env `MAMBA_ROOT_PREFIX`.
- locations:
  - Root prefix: holds `pkgs/` (package cache) and `envs/`
    - discover: `micromamba info` (shows "root prefix" and "package cache") — unverified, docs page fetched had no detail
    - override: env `MAMBA_ROOT_PREFIX`; `pkgs_dirs`, `envs_dirs` in `.mambarc`/`.condarc`
    - default: `~/micromamba` on both OS (unverified)
  - Classic `mamba` (conda-based) uses conda's locations (see conda).
- scope: global
- measure: `du` of `<root>/pkgs`
- clean: `micromamba clean --all` (flags `--index-cache`, `--packages`, `--tarballs`, `--lock`, `--trash`, `--yes`; all unverified); envs: `micromamba env remove -n <name>` (unverified)
- regenerable: pkgs = yes-but-large-download; envs = no — user-built
- in_use_hazard: as for conda
- sources: https://mamba.readthedocs.io/en/latest/user_guide/micromamba.html (fetched page lacked the needed detail)
- verification: discover = unverified; default paths = unverified; clean = unverified

## pyenv
- name: pyenv; ecosystem: Python
- detect: `pyenv` on PATH or dir `~/.pyenv` / `$PYENV_ROOT`.
- locations:
  1. Installed Python versions: `$(pyenv root)/versions/<version>`
     - discover: `pyenv root` (documented as a command; not run), `pyenv prefix <version>` gives one version dir
     - override: env `PYENV_ROOT`
     - default (both OS): `~/.pyenv`
  2. Download cache for python-build: `~/.pyenv/cache` (caching is enabled only if this dir exists)
     - override: env `PYTHON_BUILD_CACHE_PATH`
  3. Kept sources: `~/.pyenv/sources` (only when `pyenv install -k/--keep` was used)
  4. Build dir: subdirectory of `TMPDIR` by default (`PYTHON_BUILD_BUILD_PATH`)
- scope: global
- measure: `du` of `versions/`, `cache/`, `sources/`
- clean: versions: `pyenv uninstall <version>` (docs show no `-f` flag; `-f` skips the prompt per pyenv help, unverified); cache/sources: delete directory
- regenerable: versions = yes-but-large-download plus compile time (building from source takes minutes and needs build deps; virtualenvs that point to a removed version break); cache = yes
- in_use_hazard: uninstalling a version used by pyenv-virtualenv envs or `.python-version` files breaks them (plugins such as pyenv-virtualenv may run extra cleanup)
- sources: https://github.com/pyenv/pyenv/blob/master/README.md ; https://raw.githubusercontent.com/pyenv/pyenv/master/plugins/python-build/README.md
- verification: discover = official-docs (not run); default paths = official-docs; clean = official-docs (`uninstall` usage); `-f` unverified

## virtualenv (app-data)
- name: virtualenv app-data; ecosystem: Python
- detect: `virtualenv` on PATH or importable; dir exists.
- locations:
  - App data: embedded seed wheels (pip/setuptools/wheel), interpreter-discovery cache, periodic-update data
    - discover: `virtualenv --help` shows the `--app-data` default for the installed version (unverified); search results suggest a platform-specific user-data folder via platformdirs
    - override: option `--app-data`; env `VIRTUALENV_APP_DATA` pattern (unverified); `--reset-app-data` (older docs) / `--clear-app-data` (some versions) start from empty
    - macOS default: `~/Library/Application Support/virtualenv` (unverified, platformdirs convention)
    - Linux default: `~/.local/share/virtualenv` (unverified)
- scope: global
- measure: `du`
- clean: `virtualenv --reset-app-data` or `--clear-app-data` flag names differ by version (unverified); otherwise delete directory
- regenerable: yes (small)
- in_use_hazard: low; seeding venvs offline fails until refilled
- sources: search results only (virtualenv.pypa.io CLI doc pages for 20.2.2/20.3.0 listed `--reset-app-data`); direct fetch of current docs returned 404
- verification: discover = unverified; default paths = unverified; clean = unverified. Candidate to drop or fold into "delete directory" if not verifiable.

## tox
- name: tox; ecosystem: Python
- detect: `tox` on PATH or `tox.ini` / `tox.toml` / `[tool.tox]` in project.
- locations:
  - `.tox/` per project (venvs per env, packaging env)
    - override: `work_dir` setting / `--workdir`; legacy `toxworkdir` (unverified)
    - default: `<project>/.tox`
- scope: per-project; marker: `.tox/` containing env dirs, each with `pyvenv.cfg`; project has tox config (tox.ini etc.)
- measure: `du` of each `.tox`
- clean: no native clean command found; `tox -r`/`--recreate` recreates envs on next run (unverified). Delete directory.
- regenerable: yes (recreated from config, needs network)
- in_use_hazard: running tox session uses it
- sources: https://tox.wiki/en/latest/config.html (fetched empty)
- verification: discover = unverified; default paths = unverified; clean = unverified

## nox
- name: nox; ecosystem: Python
- detect: `nox` on PATH or `noxfile.py` in project.
- locations:
  - `.nox/` per project (session virtualenvs)
    - override: CLI `--envdir <dir>`
    - default: `./.nox`
- scope: per-project; marker: `noxfile.py` next to `.nox/`; each env has `pyvenv.cfg`
- measure: `du`
- clean: none documented (page states no clean command). Delete directory. `-r/--reuse-existing-virtualenvs` reuses envs (not a cleaner).
- regenerable: yes
- in_use_hazard: running session
- sources: https://nox.thea.codes/en/stable/usage.html
- verification: discover = official-docs; default paths = official-docs; clean = official-docs ("delete directory")

## pre-commit
- name: pre-commit; ecosystem: Python (language-agnostic hooks)
- detect: `pre-commit` on PATH; `.pre-commit-config.yaml` in repos.
- locations:
  - Hook repo store (clones + per-hook envs for python/node/ruby/etc.)
    - discover: none as command; resolve from env (`PRE_COMMIT_HOME`, then `XDG_CACHE_HOME`, then home default)
    - override: env `PRE_COMMIT_HOME`; else `$XDG_CACHE_HOME/pre-commit`
    - macOS/Linux default: `~/.cache/pre-commit`
- scope: global
- measure: `du`
- clean: full: `pre-commit clean` (removes cached files; exact contents unspecified in docs); prune-unused: `pre-commit gc` (removes unused cached repos). `pre-commit uninstall` only removes git hooks, does not touch cache.
- regenerable: yes (re-cloned and rebuilt on next run; needs network, can be slow)
- in_use_hazard: running hooks; removed envs rebuild on next commit, delaying commits and failing offline
- sources: https://pre-commit.com/
- verification: discover = unverified (no discover command exists; derived from docs); default paths = official-docs; clean = official-docs

## mypy cache
- name: mypy cache; ecosystem: Python
- detect: `mypy` on PATH or `.mypy_cache/` in project.
- locations:
  - `.mypy_cache/` per project (SQLite by default, `sqlite_cache = True`)
    - override: config `cache_dir`; env `MYPY_CACHE_DIR` takes precedence
    - default: `.mypy_cache` relative to cwd/config
- scope: per-project; marker: `.mypy_cache/` (contains `CACHEDIR.TAG` and `.gitignore` in practice; docs fetched do not mention this — unverified)
- measure: `du`
- clean: none; delete directory
- regenerable: yes (slow first full re-check)
- in_use_hazard: concurrent mypy run/dmypy daemon may error or recreate; stop `dmypy` first (unverified)
- sources: https://mypy.readthedocs.io/en/stable/config_file.html
- verification: discover = official-docs; default paths = official-docs; clean = official-docs (none documented)

## ruff cache
- name: ruff cache; ecosystem: Python
- detect: `ruff` on PATH or `.ruff_cache/` in project.
- locations:
  - `.ruff_cache/` in the project root
    - override: setting `cache-dir` (overrides even env); env `RUFF_CACHE_DIR`
    - default: `.ruff_cache`
- scope: per-project; marker: `.ruff_cache/` (with `CACHEDIR.TAG`, unverified)
- measure: `du`
- clean: `ruff clean` (removes all cache directories found under the current dir; exact behavior is from memory — unverified, fetched docs page did not cover it); else delete directory
- regenerable: yes
- in_use_hazard: none significant
- sources: https://docs.astral.sh/ruff/settings/#cache-dir
- verification: discover = official-docs; default paths = official-docs; clean = unverified

## pytest cache
- name: pytest cache; ecosystem: Python
- detect: `pytest` / `.pytest_cache/` in project.
- locations:
  - `.pytest_cache/` (lastfailed, nodeids, plugin values)
    - override: ini option `cache_dir` (unverified)
    - default: `.pytest_cache` in rootdir (inferred from docs example output `cachedir: .../.pytest_cache`)
- scope: per-project; marker: `.pytest_cache/`
- measure: `du` (typically tiny); `pytest --cache-show` lists contents
- clean: `pytest --cache-clear` clears cache contents before a run (it also runs the tests); otherwise delete directory
- regenerable: yes (loses `--lf`/`--ff` history)
- in_use_hazard: none
- sources: https://docs.pytest.org/en/stable/how-to/cache.html
- verification: discover = official-docs; default paths = official-docs (inferred); clean = official-docs

## __pycache__
- name: `__pycache__` / `.pyc` bytecode; ecosystem: Python
- detect: directories named `__pycache__` under project/site-packages.
- locations:
  - `__pycache__/` next to each source dir; override: env `PYTHONPYCACHEPREFIX` / `sys.pycache_prefix` redirects all bytecode into one tree (unverified, from CPython docs memory)
- scope: per-project (also inside every venv and system site-packages); marker: dir named `__pycache__` containing `*.pyc`
- measure: `du` of each
- clean: delete directory (or `python -m pyclbr`-style tools do not apply); `find . -name __pycache__` walk. No native command.
- regenerable: yes (automatic on next import)
- in_use_hazard: none for running processes; do not delete inside a venv you do not intend to slow down; must not match user data dirs that happen to be named `__pycache__`
- sources: none fetched
- verification: discover = unverified; default paths = unverified; clean = unverified

## project venv / .venv
- name: project virtual environments (`venv`, `.venv`, `env`, any dir with `pyvenv.cfg`); ecosystem: Python
- detect: directory containing `pyvenv.cfg` (marker). Name is not reliable.
- locations: wherever the project made it; created by `python -m venv`, `virtualenv`, `uv venv`/`uv sync` (`.venv` default), Poetry/PDM/Pipenv in-project mode.
- scope: per-project; marker: `pyvenv.cfg` file in the venv root (also `bin/python` on Unix)
- measure: `du` of the venv dir
- clean: delete directory. uv-managed: `uv cache prune` handles only centralized envs, not project `.venv`.
- regenerable: no — user-built environment (installed packages and versions), though recreatable from a lock/requirements file if one exists and network is available. A venv with packages installed from local paths or no lock file may not be reproducible.
- in_use_hazard: an activated shell, running server, or IDE interpreter setting points at it; deleting breaks them immediately. `pyvenv.cfg` `home` may reference a uv/pyenv-managed Python.
- sources: none fetched (marker is the standard `pyvenv.cfg` from the venv module; unverified in this research)
- verification: discover = unverified; default paths = unverified; clean = unverified

## Jupyter
- name: Jupyter (jupyter_core paths); ecosystem: Python
- detect: `jupyter` on PATH.
- locations:
  1. Config: `~/.jupyter` (override `JUPYTER_CONFIG_DIR`)
  2. Data (kernelspecs under `kernels/`, extensions, nbextensions, templates)
     - Linux: `~/.local/share/jupyter` (or `$XDG_DATA_HOME`); macOS: `~/Library/Application Support/Jupyter`; override `JUPYTER_DATA_DIR`; extra search dirs `JUPYTER_PATH`
  3. Runtime (connection files, PID files, logs): `runtime/` under data dir (macOS docs say `~/Library/Jupyter/runtime` historically — unverified); override `JUPYTER_RUNTIME_DIR`
  - discover: `jupyter --paths`, `jupyter --config-dir`, `jupyter --data-dir`, `jupyter --runtime-dir`
- scope: global
- measure: `du` of data/runtime dirs
- clean: kernels: `jupyter kernelspec list` then `jupyter kernelspec remove <name>` (remove not in fetched docs; unverified); runtime files: delete stale files in `runtime/` (connection files of dead kernels)
- regenerable: runtime = yes; kernelspecs = no (registered by user/installers; re-registerable); config = no — contains user settings
- in_use_hazard: deleting runtime files of live kernels disconnects running notebooks
- sources: https://docs.jupyter.org/en/latest/use/jupyter-directories.html
- verification: discover = official-docs; default paths = official-docs; clean = unverified

## Hugging Face Hub cache
- name: Hugging Face Hub cache; ecosystem: ML
- detect: `hf` (or legacy `huggingface-cli`) on PATH; dir exists.
- locations:
  1. Hub file cache (`models--*`, `datasets--*`, `spaces--*`; each has `blobs/`, `refs/`, `snapshots/` (symlinks to blobs), `trees/`, `.no_exist/`; `CACHEDIR.TAG` auto-created)
     - discover: no CLI prints it directly; resolve `HF_HUB_CACHE`, else `$HF_HOME/hub`, else default (docs list `HF_HOME` and `HF_HUB_CACHE`; legacy `HUGGINGFACE_HUB_CACHE`; `XDG_CACHE_HOME` also moves the default)
     - macOS/Linux default: `~/.cache/huggingface/hub`
  2. Xet cache (upload shard cache + staging): `~/.cache/huggingface/xet`, override `HF_XET_CACHE`; soft limit 16 GB for shard-cache
  3. Assets (downstream libs, e.g. `datasets`): `<HF_HOME>/assets`
  4. Shared xet blobs: `<CACHE_DIR>/blobs/<prefix>/<xet_hash>` with `.refs` manifests
- scope: global
- measure: `hf cache ls` (per-repo sizes, filters like `size>1GB`, `accessed>30d`, `--format json`); `du` of dir
- clean: delete specific: `hf cache rm <repo-or-revision>` (e.g. `model/bert-base-cased`; flags `--dry-run`, `--yes`, `--cache-dir PATH`; also `hf://` file URIs); prune-unused: `hf cache prune` (unreferenced revisions, `.incomplete` files, orphaned shared blobs; same flags); xet: docs say `rm -rf ~/.cache/huggingface/xet` is the way to reclaim it. No single "wipe all" command; wipe by `hf cache rm $(hf cache ls -q) -y`. Do NOT hand-delete files in `blobs/` or `snapshots/` (symlink structure; shared xet blobs; older clients' deleters may remove shared files still used by other repos).
- regenerable: yes-but-large-download (models can be tens of GB; gated/private models need auth tokens to re-download; models deleted from the Hub cannot be re-downloaded)
- in_use_hazard: a process loading a model has files open/symlinked; deleting mid-run breaks loading; interrupted downloads leave `.incomplete`
- sources: https://huggingface.co/docs/huggingface_hub/guides/manage-cache
- verification: discover = official-docs (no discover command); default paths = official-docs; clean = official-docs

## PyTorch hub / torchvision weights
- name: PyTorch hub cache; ecosystem: ML
- detect: `torch` importable; dir exists.
- locations:
  - Hub dir (hub repos + `checkpoints/` for `load_state_dict_from_url` weights, including torchvision pretrained weights)
    - discover: `python -c "import torch; print(torch.hub.get_dir())"` (API documented; not run)
    - override: `torch.hub.set_dir()`, env `TORCH_HOME` (uses `$TORCH_HOME/hub`), `XDG_CACHE_HOME` (`$XDG_CACHE_HOME/torch/hub`)
    - default (both OS): `~/.cache/torch/hub`
- scope: global
- measure: `du`
- clean: delete directory (no native command)
- regenerable: yes-but-large-download
- in_use_hazard: running process loading weights
- sources: https://docs.pytorch.org/docs/2.14/hub.html
- verification: discover = official-docs; default paths = official-docs; clean = official-docs (none)

## Ollama
- name: Ollama; ecosystem: ML (local LLM runtime)
- detect: `ollama` on PATH; macOS app; Linux systemd service.
- locations:
  - Models (blobs + manifests)
    - discover: no command prints the dir; `ollama ls` lists models
    - override: env `OLLAMA_MODELS` (Linux installer: the `ollama` user needs read/write)
    - macOS default: `~/.ollama/models`
    - Linux default (standard installer/service): `/usr/share/ollama/.ollama/models`; a per-user run may use `~/.ollama/models` (unverified)
- scope: global
- measure: `du` of models dir; `ollama ls` shows per-model size
- clean: `ollama rm <model>` (per model); `ollama stop <model>` unloads. No wipe-all; remove each model. Linux system path may need sudo to read/delete.
- regenerable: yes-but-large-download (models are multi-GB; custom models built with a Modelfile (`ollama create`) exist only locally and are NOT re-downloadable)
- in_use_hazard: daemon holds loaded models; `ollama rm` is the safe route while the server runs; removing files by hand under a running server corrupts its view
- sources: https://docs.ollama.com/faq ; https://docs.ollama.com/cli
- verification: discover = official-docs (no discover command); default paths = official-docs; clean = official-docs (`ollama rm`, `ollama ls`; docs say `ls`, not `list`)

## Keras / TensorFlow datasets
- name: Keras datasets + TFDS; ecosystem: ML
- detect: dirs exist; `tensorflow`/`keras`/`tensorflow_datasets` importable.
- locations:
  1. Keras `get_file` cache (datasets, pretrained weights): `~/.keras/datasets`, `~/.keras/models`; override env `KERAS_HOME` (default `~/.keras`) — from general knowledge, the fetched page was truncated (unverified)
  2. TensorFlow Datasets: `~/tensorflow_datasets/` (docs: `data_dir` defaults to `~/tensorflow_datasets/`; downloads in `~/tensorflow_datasets/downloads/`); override `data_dir=` argument or env `TFDS_DATA_DIR` (env not in docs fetched — unverified)
- scope: global
- measure: `du`
- clean: delete directory (no native clean documented)
- regenerable: yes-but-large-download (dataset downloads can be many GB; some sources need manual download into `manual_dir` and are not re-fetchable automatically)
- in_use_hazard: running training reading dataset
- sources: https://www.tensorflow.org/datasets/overview ; https://www.tensorflow.org/api_docs/python/tf/keras/utils/get_file
- verification: discover = unverified; default paths = official-docs (TFDS) / unverified (Keras); clean = official-docs (none)

## NLTK data
- name: NLTK data; ecosystem: Python NLP
- detect: `nltk_data` directory; `nltk` importable.
- locations:
  - `nltk_data` (corpora, tokenizers, models). Downloader searches for an existing `nltk_data` dir; central: `/usr/share/nltk_data` (Unix), `/usr/local/share/nltk_data` (Mac); user dir otherwise (commonly `~/nltk_data` — docs fetched do not name it; unverified)
    - override: env `NLTK_DATA`; downloader `-d <dir>`
    - discover: `python -c "import nltk; print(nltk.data.path)"` (from memory; unverified)
- scope: global
- measure: `du`
- clean: delete directory (or specific corpus subdir)
- regenerable: yes-but-large-download (some corpora large; downloaded corpora are regenerable via `nltk.download`)
- in_use_hazard: low
- sources: https://www.nltk.org/data.html
- verification: discover = unverified; default paths = official-docs (central) / unverified (`~/nltk_data`); clean = unverified

## matplotlib
- name: matplotlib config/cache; ecosystem: Python
- detect: `matplotlib` importable; `~/.matplotlib` or `~/.cache/matplotlib` exists.
- locations:
  - Config + cache dir (font list JSON `fontlist-v*.json`, tex cache)
    - discover: `python -c "import matplotlib; print(matplotlib.get_cachedir())"` (from memory; unverified) and `matplotlib.get_configdir()`
    - override: env `MPLCONFIGDIR`; fallback to `tempfile.gettempdir()/matplotlib` if home not writable
    - Linux default: `~/.config/matplotlib` (config) and `~/.cache/matplotlib` (cache)
    - macOS ("other platforms") default: `~/.matplotlib` (e.g. `~/.matplotlib/fontlist-v390.json`)
- scope: global
- measure: `du` (tiny)
- clean: delete cache files (`fontlist-*.json`, `tex.cache`); no native command. Config (`matplotlibrc`) lives in the same dir on macOS: do not delete the whole dir.
- regenerable: yes for cache files; matplotlibrc = no (user config)
- in_use_hazard: none; first import rebuilds font cache (seconds)
- sources: https://matplotlib.org/stable/install/environment_variables_faq.html
- verification: discover = unverified; default paths = official-docs + ran-command (macOS); clean = unverified

## R: renv
- name: renv; ecosystem: R
- detect: `R`/`Rscript` on PATH; project has `renv.lock` / `renv/` dir.
- locations:
  1. Global package cache (projects link into it)
     - discover: R call `renv::paths$cache()` (documented)
     - override: env `RENV_PATHS_CACHE` (may list several, `:`-separated on Unix); `RENV_PATHS_ROOT` sets the root (unverified)
     - macOS: `~/Library/Caches/org.R-project.R/R/renv/cache`; Linux: `~/.cache/R/renv/cache`
  2. Per-project library `renv/library` (links to cache) with lockfile `renv.lock`
- scope: cache global; library per-project (marker: `renv.lock` plus `renv/` dir)
- measure: `du` of cache dir (shared cache and hard/symlinked libraries: size of `renv/library` mostly links)
- clean: `renv::purge("<pkg>")` removes a package from the cache (docs tell users to read `?purge` for caveats); wipe: delete cache dir; `renv::paths$root()` + `unlink(root, recursive=TRUE)` is the documented way to remove all global renv infrastructure. `renv::clean()` is not confirmed (unverified).
- regenerable: cache = yes (re-downloaded/compiled from CRAN; compiling can be slow); project library = yes if `renv.lock` exists
- in_use_hazard: project libraries are symlinks into the cache — deleting a cached package leaves dangling links and breaks every project using it until `renv::restore()`
- sources: https://rstudio.github.io/renv/articles/package-install.html ; https://rstudio.github.io/renv/articles/renv.html
- verification: discover = official-docs (R call, not run); default paths = official-docs; clean = official-docs (`purge`)

## R: package library
- name: R user library; ecosystem: R
- detect: `R` on PATH.
- locations: user library from `.libPaths()` (R call; unverified), typically `~/Library/R/<arch>/<ver>/library` on macOS and `~/R/<arch>-library/<ver>` on Linux (both unverified); override `R_LIBS_USER`.
- scope: global
- measure: `du`
- clean: `remove.packages("<pkg>")` per package; no cache concept
- regenerable: no — installed set chosen by user (reinstallable by name, but versions/sources may not be)
- in_use_hazard: running R sessions
- sources: renv doc mentions `utils::remove.packages("renv")` only; no direct source
- verification: discover = unverified; default paths = unverified; clean = unverified

## Julia depots
- name: Julia depot (`~/.julia`); ecosystem: Julia
- detect: `julia` on PATH; `~/.julia` exists.
- locations:
  - User depot (first entry of `DEPOT_PATH`): `packages/`, `artifacts/`, `compiled/`, `registries/`, `scratchspaces/`, `logs/`, `environments/` (subdir names from memory; docs fetched only confirm `logs/repl_history.jl`)
    - discover: `julia -e 'println(DEPOT_PATH)'` (variable documented; not run)
    - override: env `JULIA_DEPOT_PATH` (must be set before start; leading empty entry expands to default depot)
    - macOS/Linux default: `~/.julia`; also read-only system depots `/usr/local/share/julia`, `/usr/share/julia`
- scope: global
- measure: `du` of each subdir (`packages`, `artifacts`, `compiled` are the large ones)
- clean: `julia -e 'using Pkg; Pkg.gc()'`: removes orphaned packages/artifacts unused for at least `collect_delay` (default `Day(7)`); `Pkg.gc(; collect_delay=Day(0))` removes immediately — exact call form from docs signature `Pkg.gc(; collect_delay::Period=Day(7), io::IO=stderr)` (needs `using Dates` for `Day`). Auto-GC can be disabled via `JULIA_PKG_GC_AUTO=false`. Compiled cache (`compiled/`) and registries: delete directory.
- regenerable: packages/artifacts = yes-but-large-download (artifacts can be GBs); `environments/` (global named envs like `v1.10`) = no — contains the user's global Project.toml/Manifest.toml; `registries` = yes; `scratchspaces` = may hold package-owned state (unverified)
- in_use_hazard: `Pkg.gc` decides "orphaned" from usage logs of known Manifest.toml files; projects whose Manifest was moved/not logged may lose packages (they will be re-downloaded on `instantiate`)
- sources: https://pkgdocs.julialang.org/v1/api/ ; https://docs.julialang.org/en/v1/manual/environment-variables/ ; https://docs.julialang.org/en/v1/base/constants/
- verification: discover = official-docs (not run); default paths = official-docs (`~/.julia`); clean = official-docs (`Pkg.gc`)

---

## Contract observations

Patterns the registry contract must express that a naive "path + rm -rf" entry cannot:

1. Path discoverable only by running the tool, with a per-OS default as a fallback only. `pip cache dir`, `uv cache dir`, `uv python dir`, `uv tool dir`, `poetry config cache-dir`, `jupyter --paths`, `pyenv root`, `hatch env find`, `pipx environment`, R `renv::paths$cache()`, Julia `DEPOT_PATH`, `torch.hub.get_dir()`. Needs fields: `discover` command, parse mode (single line / key: value / JSON), and a separate `default_paths` per OS used when the tool is absent. Defaults are not uniform: uv uses `~/.cache/uv` on macOS while pip/Poetry/Hatch/renv use `~/Library/Caches/...`; matplotlib on macOS is `~/.matplotlib`.
2. Override chain with precedence. Env var, CLI flag, config key, `XDG_*` variables, and shell-level files all move the same location (pip `PIP_CACHE_DIR`; Ruff `cache-dir` beats `RUFF_CACHE_DIR`; HF `HF_HUB_CACHE` > `HF_HOME/hub`; Torch `TORCH_HOME` > `XDG_CACHE_HOME`; renv path lists separated by `:`). The contract needs an ordered resolution list, and a multi-path value type.
3. More than one location per tool, each with its own scope, regenerability and clean action: uv (cache vs Pythons vs tools), Poetry (cache vs virtualenvs), Hatch (cache, data, envs, pythons), pyenv (versions vs cache vs sources), HF (hub vs xet vs assets), Julia (packages, artifacts, environments, compiled), Jupyter (config vs data vs runtime). A tool entry must be a list of locations, each carrying its own `regenerable` and `clean`.
4. Locations that mix cache with user-built or user-authored state, so the whole directory must never be a single delete target: Poetry virtualenvs live INSIDE the cache dir (`{cache-dir}/virtualenvs`); matplotlib config and cache share `~/.matplotlib` on macOS; Jupyter data holds kernelspecs; Julia `environments/`; Ollama custom models created with `ollama create`; conda `envs`. Needs `regenerable: no` per location with a data description, sub-path exclusions ("size/clean everything except `virtualenvs`"), and a refusal to run a whole-dir delete.
5. Per-project instances found by walking project trees, with a marker, and in several cases the marker is a file inside, not the dir name: `pyvenv.cfg` (any venv name), `.mypy_cache`, `.ruff_cache`, `.pytest_cache`, `.tox`, `.nox` (tied to `tox.ini`/`noxfile.py`), `__pycache__`, `renv.lock` + `renv/`. The walk must prune descent into venvs and `node_modules`, and handle huge counts of `__pycache__` dirs; `__pycache__` also lives inside every venv.
6. Size reported by the tool, not `du`: `pip cache info`, `uv cache size` (experimental; uv also has a more accurate physical-space preview feature), `pdm cache info`, `hf cache ls` (logical per-repo sizes, while on-disk is physical because blobs are shared via symlinks and xet shared blobs), `conda clean --all --dry-run`. `du` over symlink-heavy stores double-counts or misses; contract needs a `measure` kind (`du` | `command` with parse rule) and a dry-run variant of clean.
7. Clean has multiple variants with different safety levels: full wipe (`pip cache purge`, `uv cache clean`, `conda clean --all`) vs prune-unused (`uv cache prune`, `uv cache prune --ci`, `pre-commit gc`, `hf cache prune`, `Pkg.gc` with a time threshold, `conda clean --packages`) vs targeted removal needing an argument (`ollama rm <model>`, `hf cache rm <id>`, `pip cache remove <pattern>`, `poetry cache clear <repo>:<pkg>:<ver>`) vs destructive-flag variants (`conda clean --force-pkgs-dirs`, `hatch env prune`, `pdm venv purge`, `hatch python remove all`). Contract needs `clean.full`, `clean.prune`, `clean.targeted(args)`, a `danger` level, and a `dry_run` flag.
8. Native-only deletion: docs say never to modify the uv cache by hand; HF cache uses symlink/blob sharing so hand-deleting `blobs/` or `snapshots/` corrupts it; pip's layout "may change between any two versions"; renv cache is symlinked from project libraries (dangling links). An entry needs `native_clean_required: true` (fallback `delete directory` only when the tool is absent) and a note that fallback is unsafe.
9. Cross-location dependencies: removing a managed Python (uv, pyenv, hatch) breaks venvs whose `pyvenv.cfg` `home` points at it; deleting the renv cache breaks project libraries; conda pkgs removal affects envs. The contract needs a `dependents` relation (or at least an `in_use_hazard` that is machine-readable) and an ordering rule when cleaning several locations.
10. Locks, daemons and privileges: uv blocks up to 5 min on a lock (`UV_LOCK_TIMEOUT`, `--force`); Ollama is a daemon whose models dir on Linux is `/usr/share/ollama/.ollama/models` and needs sudo; `dmypy` daemon; Jupyter live kernels own runtime files. Need `requires_stopped: <process/daemon>` and `needs_sudo` flags.
11. Regenerability classes beyond yes/no: `yes-but-large-download` (HF, torch, TFDS, Ollama, conda pkgs, Julia artifacts, managed Pythons) with a bandwidth/size warning, plus "not re-downloadable" cases (Ollama custom models, HF models removed or gated, TFDS manual-download datasets). Needs a per-location `regenerable` enum and a free-text `loss_if_deleted`.
12. Environment-vs-binary detection: a tool's caches may exist although its binary is not installed (pre-commit, HF, matplotlib). `detect` needs "binary on PATH OR directory exists", and the discover command can only run if the binary exists; otherwise use the OS default list.
13. Same location shared by several tools: the HF cache is used by `transformers`, `datasets`, `diffusers` and others; `~/.local/bin` receives shims from uv, pipx and pyenv; pip cache dir is per user, not per Python. Registry entries should be keyed by location owner, with deduplication so the same dir is not measured twice in total reclaimable space.
14. Verification status is part of the data: several fields in this file are `unverified` (pipx, virtualenv, tox, conda/mamba defaults, Pipenv venvs, R library, `__pycache__`, project venv). The contract should carry a per-field `verified` tag and refuse to ship a clean command with `unverified` status without review.
