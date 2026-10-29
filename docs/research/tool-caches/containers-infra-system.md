# Containers, VMs, Kubernetes, IaC and system package managers: cache catalog

Raw facts for deriving the `ccc` registry contract. Verification tags: `official-docs` = confirmed on an official page; `ran-command` = confirmed by running a read-only command; `unverified` = from memory or not confirmable from fetched pages, to be re-checked before shipping. Several fields for lesser tools are `unverified` because the official pages fetched did not state them; none of those commands should ship without checking. On Docker Desktop, `Docker.raw` is a sparse file whose apparent size is far larger than its allocated size (`du -sh` shows the allocated one), so `ls`/apparent size must never be used.

Docker facts first, since they drive the contract.

Docker command matrix (all need a running daemon; on macOS that means Docker Desktop's VM):

| Command | Removes | Volumes? | Source |
|---|---|---|---|
| `docker system df` / `docker system df -v` / `docker system df --format json` | reports only. Columns TYPE, TOTAL, ACTIVE, SIZE, RECLAIMABLE for Images, Containers, Local Volumes, Build Cache. `-v` lists per item (images: SIZE, SHARED SIZE, UNIQUE SIZE; volumes: NAME, LINKS, SIZE). Reclaimable figure = RECLAIMABLE column. | n/a | official-docs + ran-command |
| `docker container prune` | all stopped containers | no | unverified (standard) |
| `docker image prune` | dangling images only; `-a` all images not used by a container | no | unverified (standard) |
| `docker network prune` | networks not used by any container (zero disk) | no | unverified (standard) |
| `docker builder prune` | dangling build cache; `-a/--all` all unused build cache; `--filter until=24h`; `--keep-storage`; `-f` | no | official-docs |
| `docker buildx prune` | same for a buildx builder; `--all`, `--filter`, `--max-used-space`, `--builder`, `--verbose` | no | official-docs |
| `docker volume prune` | unused ANONYMOUS volumes by default; `-a/--all` (API 1.42+) also unused NAMED volumes; `--filter label=...` | YES | official-docs |
| `docker system prune` | stopped containers, unused networks, dangling images, unused build cache | no | official-docs |
| `docker system prune -a` | above plus all unused (non-dangling) images | no | official-docs |
| `docker system prune --volumes` | above plus anonymous volumes (per the docs page wording) | YES | official-docs |
| `docker system prune -a --volumes` | everything unused | YES | official-docs |

Note: "unused" for images means no container (including stopped ones) references it, so pruning containers first makes more images reclaimable. Docker's own `RECLAIMABLE` for volumes counts volumes with no container link, i.e. exactly the ones a named-volume prune would delete. A safe ccc "docker caches" action is: `docker builder prune -f` plus `docker image prune -a -f` (plus optionally `docker container prune -f`), never any volume command.

## Docker Engine / Docker Desktop
- name: docker; ecosystem: containers
- detect: `docker` on PATH. Needs a daemon: on macOS the Docker Desktop VM must be running (also OrbStack/Colima provide a compatible socket); `docker system df` fails otherwise. Contexts: `docker context ls` (Docker Desktop uses the `desktop-linux` context).
- locations:
  - images: layers of pulled/built images. discover: `docker system df` (Images row), `docker system df -v`. override: `DOCKER_HOST`/context. path: inside the VM disk (macOS) or `/var/lib/docker` (Linux; `docker info` reports "Docker Root Dir", on Docker Desktop it reports `/var/lib/docker`, which is the path inside the VM).
  - containers (stopped): writable layers. discover: `docker system df` Containers row.
  - networks: no disk usage.
  - build cache: BuildKit cache. discover: `docker system df` Build Cache row; `docker buildx du` (prints ID, RECLAIMABLE, SIZE, LAST ACCESSED; ran-command).
  - volumes: named/anonymous volumes. discover: `docker system df -v` (Local Volumes). USER DATA (databases).
  - Docker Desktop disk image: macOS `~/Library/Containers/com.docker.docker/Data/vms/0/data/Docker.raw` (ran-command; sparse file: apparent size far larger than allocated size). Location is configurable in Docker Desktop Resources > Advanced (official-docs); Docker Desktop also has a "Disk usage limit". Linux Docker Desktop path: unverified.
- scope: global (per machine/daemon)
- measure: `docker system df` (RECLAIMABLE column per category; sized by the daemon). Disk file: `du -sk` on Docker.raw (allocated size, NOT `ls -l` apparent size). A prune frees space inside the VM; whether the host file shrinks depends on Docker Desktop trimming/compacting the sparse file (not documented on the fetched page).
- clean: no full-wipe native command that is safe; use the prune matrix above. Full wipe of everything including volumes: `docker system prune -a --volumes` plus `docker volume prune -a`. Docker Desktop UI "Clean / Purge data" and "Reset to factory defaults" delete the disk image (unverified; destroys volumes). Notes: needs daemon running; per-daemon not per-user.
- regenerable: images yes-but-large-download (registry pull) or yes-but-slow-rebuild (locally built); build cache yes-but-slow-rebuild; stopped containers: writable layer data lost, mostly yes; volumes: no — contains user data (databases, uploaded files).
- in_use_hazard: prune only touches unused objects, but stopped containers removed lose their writable layer; `docker volume prune -a` removes named volumes of stopped stacks (e.g. a compose project that is down), i.e. destroys that database. Build cache prune during a running build can fail or slow it. Images used by a stopped-but-wanted container are lost after `container prune` + `image prune -a`.
- sources: https://docs.docker.com/reference/cli/docker/system/prune/ ; https://docs.docker.com/reference/cli/docker/system/df/ ; https://docs.docker.com/reference/cli/docker/builder/prune/ ; https://docs.docker.com/reference/cli/docker/volume/prune/ ; https://docs.docker.com/desktop/settings-and-maintenance/settings/
- verification: discover=ran-command; default paths=ran-command (macOS Docker.raw) / unverified (Linux Desktop); clean=official-docs (system/builder/volume prune), unverified (container/image/network prune individual pages, Desktop purge)

## docker buildx
- name: docker buildx; ecosystem: containers
- detect: `docker buildx version`; needs daemon for the default `docker` driver builder; `docker-container` driver builders live in a separate container (`buildx_buildkit_*`) with their own cache.
- locations: builder build cache (discover: `docker buildx ls` lists builders, ran-command; `docker buildx du [--builder NAME]`); buildx config/instances `~/.docker/buildx` (ran-command; override `BUILDX_CONFIG`, unverified), small.
- scope: global
- measure: `docker buildx du` (RECLAIMABLE, SIZE columns per record; ran-command output header: ID RECLAIMABLE SIZE LAST ACCESSED). `buildx du` flags not documented on fetched page.
- clean: `docker buildx prune` (`--all`, `--filter until=...`, `--max-used-space 2gb`, `--builder`, `-f` unverified for -f on this page). Full wipe: `docker buildx prune --all -f`. Needs the builder running.
- regenerable: yes-but-slow-rebuild
- in_use_hazard: pruning during an active build; cache mounts (e.g. package caches) are cleaned too.
- sources: https://docs.docker.com/reference/cli/docker/buildx/prune/
- verification: discover=ran-command; default paths=ran-command (~/.docker/buildx); clean=official-docs

## Podman (and podman machine)
- name: podman; ecosystem: containers
- detect: `podman` on PATH. On macOS commands need a running `podman machine` (ran-command: `podman system df` fails "Cannot connect to Podman" when the machine is stopped). Linux native podman is daemonless.
- locations:
  - containers/images/volumes/build containers: storage in the VM (macOS) or `~/.local/share/containers/storage` (Linux rootless; unverified). discover: `podman system df` (unverified output format).
  - podman machine VM disk: discover `podman machine list` (ran-command: shows DISK SIZE as the configured limit, not allocated) and `podman machine inspect` (ran-command: shows `ConfigDir.Path` `~/.config/containers/podman/machine/applehv`, SSH identity `~/.local/share/containers/podman/machine/machine`). Disk image files under `~/.local/share/containers/podman/machine/<provider>` (ran-command: provider subdirectories such as `applehv`, `libkrun`).
- scope: global
- measure: `podman system df` (reclaimable column, unverified); VM disk: `du` of the machine image file.
- clean: `podman system prune` (stopped containers, pods, unused networks, dangling images); `-a/--all` (all unused images and recursive); `--volumes` (volumes not used by any container); `--build` (leftover build containers, unsafe during builds); `--external`; `--filter label=/until=`; `-f` (official-docs). Specific: `podman image prune -a`, `podman volume prune` (unverified). Machine wipe: `podman machine rm` (deletes the VM and its disk, volumes inside lost; unverified).
- regenerable: images yes-but-large-download; volumes no — contains user data; VM disk: no if it holds volumes.
- in_use_hazard: `--build` while builds run; `--volumes` deletes data volumes of stopped containers.
- sources: https://docs.podman.io/en/latest/markdown/podman-system-prune.1.html
- verification: discover=ran-command (machine list/inspect), unverified (system df); default paths=ran-command (macOS), unverified (Linux); clean=official-docs (system prune), unverified (machine rm)

## containerd / nerdctl
- name: containerd/nerdctl; ecosystem: containers
- detect: `nerdctl` or `ctr` on PATH; needs containerd daemon (usually root, Linux or inside Lima/Colima VM).
- locations: image/content store `/var/lib/containerd` (unverified); nerdctl data `/var/lib/nerdctl` (unverified).
- scope: global, system-wide, sudo.
- measure: `nerdctl system df` (unverified).
- clean: `nerdctl system prune [-a] [--volumes]` (unverified flags, mirrors docker).
- regenerable: images yes-but-large-download; volumes no — user data.
- in_use_hazard: as Docker.
- sources: none fetched.
- verification: discover=unverified; default paths=unverified; clean=unverified

## Colima
- name: colima; ecosystem: VMs/containers (Lima-based)
- detect: `colima` on PATH; `colima status`/`colima list` (unverified); commands inside need the VM running.
- locations: `~/.colima` (config and VM disk) and `~/.lima/colima` (instance dir including disk) (unverified: README page fetched did not cover this); Lima download cache `~/Library/Caches/lima` (official-docs for Lima).
- scope: global
- measure: `du` of the instance directory (the disk is sparse; use allocated size); inside, `docker system df`.
- clean: `colima delete` (deletes the VM; destroys containers and volumes in it, unverified); `colima prune` removes cached assets (unverified). Prefer pruning through docker inside the VM.
- regenerable: VM yes-but-slow-rebuild; volumes inside no — user data.
- in_use_hazard: deleting while running destroys everything inside.
- sources: https://github.com/abiosoft/colima/blob/main/README.md (does not cover these)
- verification: all unverified

## Lima
- name: lima; ecosystem: VMs
- detect: `limactl` on PATH.
- locations: instances `$LIMA_HOME` default `~/.lima` (official-docs); each instance has a `disk` file (legacy `diffdisk`); download cache macOS `~/Library/Caches/lima`, Linux `$XDG_CACHE_HOME/lima` (usually `~/.cache/lima`) under `download/by-url-sha256/` (official-docs).
- scope: global
- measure: `du` of the cache dir; `limactl list` shows instance disk sizes (unverified).
- clean: cache: delete directory `~/Library/Caches/lima`; `limactl prune` exists (listed in docs nav; behavior not described on fetched page, unverified: believed to remove cached downloads); instances: `limactl delete NAME` (unverified, destroys VM disk).
- regenerable: download cache yes-but-large-download; instance disks: no if they hold data.
- in_use_hazard: deleting the cache while an instance is starting breaks the download.
- sources: https://lima-vm.io/docs/dev/internals/
- verification: discover=unverified; default paths=official-docs; clean=unverified (delete directory for cache is plain fs)

## OrbStack
- name: orbstack; ecosystem: containers/VM (macOS only)
- detect: `orb` / `orbctl` on PATH, `/Applications/OrbStack.app`; docker socket via context `orbstack`. Needs app running.
- locations: data disk image under `~/Library/Group Containers/HUAQ24HBR6.dev.orbstack/data` (unverified; the OrbStack docs page fetched did not cover storage).
- scope: global
- measure: `docker system df` against the orbstack context; OrbStack reports disk usage in its UI.
- clean: `docker system prune` family inside; OrbStack reclaims host space automatically as the image is dynamically sized (unverified).
- regenerable / in_use_hazard: as Docker; volumes no — user data.
- sources: https://docs.orbstack.dev/docker/ (not helpful)
- verification: all unverified

## minikube
- name: minikube; ecosystem: kubernetes
- detect: `minikube` on PATH; `~/.minikube` exists; cluster VM/container may be stopped.
- locations: `~/.minikube` (default; override `MINIKUBE_HOME`, unverified; `--purge` deletes the `.minikube` folder in the user directory, official-docs) holding `cache/` (ISO, preloaded tarballs, images), `machines/`, `profiles/`.
- scope: global
- measure: `du ~/.minikube/cache` (cache only, unverified path); `du ~/.minikube/machines` for cluster disks.
- clean: `minikube delete` (current profile VM and all associated files); `minikube delete --all`; `--purge` additionally deletes `.minikube` (official-docs). Cache-only: delete directory `~/.minikube/cache` (unverified).
- regenerable: cache yes-but-large-download; cluster state no (workloads inside the cluster are lost).
- in_use_hazard: `delete` destroys the running cluster and any PVC data in it.
- sources: https://minikube.sigs.k8s.io/docs/commands/delete/
- verification: discover=unverified; default paths=unverified (partly official: .minikube); clean=official-docs

## kind
- name: kind; ecosystem: kubernetes
- detect: `kind` on PATH; needs a container runtime daemon (Docker/Podman).
- locations: no cache of its own; clusters are containers (`kind-control-plane`...) and use image `kindest/node` in Docker's image store; kubeconfig entries in `${HOME}/.kube/config` unless `KUBECONFIG` (official-docs). discover: `kind get clusters` (official-docs).
- scope: global (via Docker)
- measure: `docker system df` (node image) ; clusters are small containers.
- clean: `kind delete cluster [--name NAME]` (official-docs; deleting a nonexistent cluster is not an error). `kind delete clusters --all` is not confirmed. Node images are removed via docker image prune.
- regenerable: node image yes-but-large-download; cluster state no.
- in_use_hazard: deleting a cluster destroys its workloads.
- sources: https://kind.sigs.k8s.io/docs/user/quick-start/
- verification: discover=official-docs; default paths=official-docs (kubeconfig); clean=official-docs (`delete cluster`)

## k3d
- name: k3d; ecosystem: kubernetes
- detect: `k3d` on PATH; needs Docker daemon.
- locations: clusters are Docker containers/volumes/networks (`k3d-<name>-*`); discover `k3d cluster list` (unverified).
- measure: `docker system df`.
- clean: `k3d cluster delete NAME` / `--all` (unverified).
- regenerable: images yes; cluster state no.
- sources: none fetched.
- verification: all unverified

## Helm
- name: helm; ecosystem: kubernetes
- detect: `helm` on PATH.
- locations:
  - cache (downloaded charts, repo index): discover `helm env HELM_CACHE_HOME` (unverified command, official page lists the variables); override `HELM_CACHE_HOME`; macOS `$HOME/Library/Caches/helm`; Linux `$HOME/.cache/helm` (official-docs). Subdirs `repository`, `content` (flags `--repository-cache`, `--content-cache` default `~/.cache/helm/repository`, `~/.cache/helm/content`).
  - config (repositories.yaml with repo list): `HELM_CONFIG_HOME`; macOS `$HOME/Library/Preferences/helm`; Linux `$HOME/.config/helm`. NOT cache.
  - data (plugins): `HELM_DATA_HOME`; macOS `$HOME/Library/helm`; Linux `$HOME/.local/share/helm`. NOT cache (installed plugins).
- scope: global
- measure: `du` of cache dir.
- clean: no native command (official docs list none): delete directory (cache dir only).
- regenerable: yes (re-fetched by `helm repo update`/`helm pull`)
- in_use_hazard: none for running releases; releases live in the cluster.
- sources: https://helm.sh/docs/helm/helm/
- verification: discover=unverified; default paths=official-docs; clean=official-docs (no native command)

## kubectl
- name: kubectl; ecosystem: kubernetes
- detect: `kubectl` on PATH.
- locations: `~/.kube/cache` (discovery and HTTP caches, subdirs `discovery`, `http`; unverified from docs); `~/.kube/config` is NOT cache (cluster credentials). Override: `--cache-dir` flag / `KUBECACHEDIR` (unverified).
- scope: global
- measure: `du ~/.kube/cache`.
- clean: delete directory `~/.kube/cache` only.
- regenerable: yes
- in_use_hazard: none (re-discovers API resources, slower next call).
- sources: none fetched.
- verification: discover=unverified; default paths=unverified; clean=unverified

## Terraform
- name: terraform; ecosystem: IaC
- detect: `terraform` on PATH.
- locations:
  - global plugin cache: off unless `plugin_cache_dir` (CLI config) or `TF_PLUGIN_CACHE_DIR` is set; no default location; directory must already exist; Terraform never deletes from it (official-docs). Conventional `~/.terraform.d/plugin-cache` (docs example). discover: env var, else parse CLI config `~/.terraformrc` (unverified path) for `plugin_cache_dir`.
  - per-project `.terraform/`: contains `providers/` (pure cache, re-downloaded by `terraform init`), `terraform.tfstate` (backend binding / cached backend config; not cache), `modules/` (downloaded modules, regenerable via init), `environment`. Override `TF_DATA_DIR` (not covered by fetched page, unverified).
- scope: global (plugin cache) and per-project (`.terraform`, marker: directory `.terraform` next to `*.tf` files; `.terraform.lock.hcl` should be kept).
- measure: `du`.
- clean: no native command: delete directory. For per-project clean only `.terraform/providers` (and optionally `.terraform/modules`); never delete the whole `.terraform` blindly (backend binding is lost, re-init needed and with local state this is data loss risk if state is stored inside, e.g. workspaces/local `terraform.tfstate` are in the project root, not `.terraform`, but `.terraform/terraform.tfstate` stores backend config).
- regenerable: yes-but-large-download (providers can be hundreds of MB)
- in_use_hazard: removing providers during an apply/plan breaks the running operation; next run needs `terraform init`.
- sources: https://developer.hashicorp.com/terraform/cli/config/config-file
- verification: discover=official-docs (env/config key); default paths=official-docs (no default; local confirms); clean=official-docs (no native command stated; `.terraform` internals unverified)

## Terragrunt cache
- name: terragrunt; ecosystem: IaC
- detect: `terragrunt` on PATH.
- locations: per-project `.terragrunt-cache/` directories next to `terragrunt.hcl` (contents: downloaded module sources and `.terraform` copies; unverified). Override: `TERRAGRUNT_DOWNLOAD` (unverified).
- scope: per-project; marker: `.terragrunt-cache` directory beside `terragrunt.hcl`.
- measure: `du`.
- clean: delete directory (commonly `find . -name .terragrunt-cache -type d -prune -exec rm -rf {} +`, but ccc should do the walk itself). No native command.
- regenerable: yes-but-large-download
- in_use_hazard: a running terragrunt run is working from this directory.
- sources: none fetched.
- verification: all unverified

## Pulumi
- name: pulumi; ecosystem: IaC
- detect: `pulumi` on PATH.
- locations: plugins `~/.pulumi/plugins` (override `PULUMI_HOME`; both unverified, fetched page does not state storage). discover: `pulumi plugin ls` (unverified).
- scope: global
- measure: `du ~/.pulumi/plugins`; `pulumi plugin ls` lists size column (unverified).
- clean: `pulumi plugin rm --all --yes` (official page is `pulumi plugin remove`; `rm` alias not confirmed; `-a/--all`, `-y/--yes`; with no args clears the entire download cache; `pulumi plugin rm KIND NAME [VERSION]` for targeted; downloaded again automatically when needed — official-docs). Warning: `~/.pulumi/credentials.json` (login) is not cache; do not delete `~/.pulumi` wholesale.
- regenerable: yes-but-large-download
- in_use_hazard: none for deployed stacks; next `pulumi up` redownloads.
- sources: https://www.pulumi.com/docs/iac/cli/commands/pulumi_plugin_rm/
- verification: discover=unverified; default paths=unverified; clean=official-docs

## Packer
- name: packer; ecosystem: IaC/images
- detect: `packer` on PATH.
- locations: `PACKER_CACHE_DIR` default current directory `./packer_cache` (docs page contradicts itself: `./packer_cache/` vs `.packer_cache`) — per-project, holds ISOs/large downloads; plugins in `plugins` subdirectory of `PACKER_CONFIG_DIR` (default `${HOME}/.config/packer/` on Unix) or `PACKER_PLUGIN_PATH` (official-docs). Docs do not say the cache is auto-cleaned.
- scope: both: plugins global; `packer_cache` per-project (marker: dir `packer_cache` next to `*.pkr.hcl`).
- measure: `du`.
- clean: no native command: delete directory.
- regenerable: yes-but-large-download (ISOs)
- in_use_hazard: running build uses cached ISO.
- sources: https://developer.hashicorp.com/packer/docs/configure
- verification: discover=official-docs (env vars); default paths=official-docs (with noted ambiguity); clean=official-docs (none)

## Vagrant
- name: vagrant; ecosystem: VMs
- detect: `vagrant` on PATH.
- locations: boxes in `VAGRANT_HOME` (default `~/.vagrant.d`, `boxes/` subdir; unverified, fetched page did not cover). discover: `vagrant box list` (official-docs).
- scope: global; per-project `.vagrant/` holds machine state (not cache; do not delete).
- measure: `du ~/.vagrant.d/boxes`.
- clean: `vagrant box prune` (old versions; `--dry-run`, `--provider`, `--name`, `--force`, `--keep-active-boxes`); `vagrant box remove NAME [--all] [--box-version V] [--force]` (official-docs). VM disks (VirtualBox/VMware) live outside Vagrant home and are removed by `vagrant destroy` (unverified).
- regenerable: boxes yes-but-large-download; VM disks no — may hold data.
- in_use_hazard: `--force` removes a box used by an active environment; prune asks when in use.
- sources: https://developer.hashicorp.com/vagrant/docs/cli/box
- verification: discover=official-docs; default paths=unverified; clean=official-docs

## Ansible
- name: ansible; ecosystem: IaC/config
- detect: `ansible` / `ansible-galaxy` on PATH.
- locations: `ANSIBLE_HOME` default `~/.ansible` (official-docs); collections `~/.ansible/collections` plus `/usr/share/ansible/collections` (`COLLECTIONS_PATHS`, env `ANSIBLE_COLLECTIONS_PATH`); local temp `~/.ansible/tmp` (`ANSIBLE_LOCAL_TEMP`); fact cache: plugin `ANSIBLE_CACHE_PLUGIN` (docs show default `memory`), location from `ANSIBLE_CACHE_PLUGIN_CONNECTION` (default None), timeout 86400 s. discover: `ansible-config dump` (unverified for these keys).
- scope: global; fact cache path user-configured.
- measure: `du`.
- clean: no native command: delete directory (`~/.ansible/tmp`, fact-cache dir). Collections are installed content, not cache: deleting needs reinstall via requirements; treat as `yes-but-large-download` only if a requirements file exists.
- regenerable: tmp and fact cache yes; collections yes-if-listed-in-requirements.
- in_use_hazard: playbook run in progress uses tmp.
- sources: https://docs.ansible.com/ansible/latest/reference_appendices/config.html
- verification: discover=unverified; default paths=official-docs; clean=official-docs (none)

## Homebrew
- name: brew; ecosystem: system packages (macOS/Linux)
- detect: `brew` on PATH.
- locations:
  - downloads cache: discover `brew --cache` (ran-command; prints the cache path); override `HOMEBREW_CACHE`; macOS `~/Library/Caches/Homebrew`; Linux `~/.cache/Homebrew` (unverified, docs page did not state).
  - old versions (Cellar) and stale lock files: reported by `brew cleanup -n`.
  - portable-ruby old versions under `/opt/homebrew/Library/Homebrew/vendor/portable-ruby` (reported by the dry run, ran-command).
  - orphaned dependencies: `brew autoremove` (uninstalls formulae installed only as dependencies and no longer needed).
- scope: global (per user cache; the Cellar is system-wide per prefix)
- measure: `brew cleanup -n` / `--dry-run` prints "Would remove: ..." and a final line "This operation would free approximately <size>" (ran-command). That figure is the reclaimable size. Cache dir: `du`.
- clean: `brew cleanup` (removes stale lock files, outdated downloads, old versions of installed formulae; downloads older than 120 days, `HOMEBREW_CLEANUP_MAX_AGE_DAYS`); `brew cleanup --prune=all` removes everything older than days (all); `-s/--scrub` also latest-version downloads except for installed; `--prune-prefix`; full wipe of cache `rm -rf "$(brew --cache)"` (official-docs, as stated on the manpage). `brew autoremove -n` for dry run (official-docs).
- regenerable: yes-but-large-download (bottles redownloaded when reinstalling); old versions are only needed for `brew switch`-style rollback.
- in_use_hazard: `brew cleanup` removes old formula versions; a running process from an old Cellar version can break; do not run concurrently with `brew install/upgrade`. Do not run autoremove automatically (it uninstalls packages).
- sources: https://docs.brew.sh/Manpage
- verification: discover=ran-command+official-docs; default paths=ran-command (macOS), unverified (Linux); clean=official-docs; dry-run output=ran-command

## MacPorts
- name: macports; ecosystem: system packages (macOS)
- detect: `port` on PATH, `/opt/local`.
- locations: distfiles `/opt/local/var/macports/distfiles`, build dirs `/opt/local/var/macports/build`, inactive versions (all unverified).
- scope: global, system-wide, sudo.
- measure: `du`; `port installed inactive` lists old versions (unverified).
- clean: `sudo port clean --all installed` (work dirs/dist), `sudo port uninstall inactive`, `sudo port reclaim` (unverified).
- regenerable: yes-but-large-download
- in_use_hazard: not concurrently with port operations.
- sources: none fetched.
- verification: all unverified

## Nix
- name: nix; ecosystem: system packages
- detect: `nix` on PATH, `/nix/store` exists; daemon (`nix-daemon`) typically needed on multi-user installs.
- locations: store `/nix/store` (all garbage collected paths); profile generations under `/nix/var/nix/profiles` (unverified); per-user `~/.cache/nix` (eval/tarball cache, unverified).
- scope: global, system-wide, often needs root for the system profile.
- measure: `nix store gc --dry-run` (official-docs: shows what would be done) ; `du -sh /nix/store`.
- clean: `nix store gc` ("deletes unreachable paths in the Nix store"; `--max N` stops after freeing N bytes; `--dry-run`; experimental command, official-docs). Classic: `nix-collect-garbage` and `nix-collect-garbage -d` (also deletes old profile generations; not covered on fetched page, unverified).
- regenerable: yes-but-large-download/rebuild; but `-d` removes rollback generations (not regenerable as a rollback target).
- in_use_hazard: GC only removes unreachable paths, i.e. not referenced by profiles, GC roots or running processes; a store path used only by an un-rooted dev shell can vanish.
- sources: https://nix.dev/manual/nix/stable/command-ref/new-cli/nix3-store-gc.html
- verification: discover=official-docs (gc dry-run); default paths=unverified; clean=official-docs (nix store gc), unverified (collect-garbage -d)

## apt
- name: apt; ecosystem: system packages (Debian/Ubuntu, Linux only)
- detect: `apt-get` on PATH.
- locations: downloaded .deb `/var/cache/apt/archives`; package lists `/var/lib/apt/lists` (re-fetched by `apt update`) (unverified: man page fetch failed).
- scope: global, system-wide, sudo.
- measure: `du -sh /var/cache/apt/archives`; `apt-get clean -s` simulate (unverified).
- clean: `sudo apt-get clean` (removes all cached .debs), `sudo apt-get autoclean` (only no-longer-downloadable), `sudo apt-get autoremove` (uninstalls unneeded packages; not cache).
- regenerable: yes-but-large-download
- in_use_hazard: do not run during an apt/dpkg operation (lock).
- sources: https://man7.org/linux/man-pages/man8/apt-get.8.html (fetch failed)
- verification: all unverified

## dnf / yum
- name: dnf; ecosystem: system packages (Fedora/RHEL, Linux only)
- detect: `dnf` or `yum`.
- locations: `/var/cache/dnf` (or `/var/cache/yum`) (unverified).
- scope: global, sudo.
- measure: `du -sh /var/cache/dnf`.
- clean: `sudo dnf clean all` (also `dnf clean packages`, `metadata`); `sudo dnf autoremove` is uninstall, not cache (unverified).
- regenerable: yes
- in_use_hazard: not during transactions.
- sources: none fetched.
- verification: all unverified

## pacman
- name: pacman; ecosystem: system packages (Arch, Linux only)
- detect: `pacman`.
- locations: `/var/cache/pacman/pkg` (unverified; Arch wiki blocked).
- scope: global, sudo.
- measure: `du -sh /var/cache/pacman/pkg`; `paccache -d` dry run (from pacman-contrib, unverified).
- clean: `sudo paccache -r` (keeps last 3 versions), `sudo pacman -Sc` (removes uninstalled packages from cache), `sudo pacman -Scc` (removes all; loses downgrade ability) (unverified).
- regenerable: yes-but-large-download; but cached old versions are the only offline downgrade source.
- in_use_hazard: not during pacman operation.
- sources: https://wiki.archlinux.org/title/Pacman (blocked)
- verification: all unverified

## snap
- name: snap; ecosystem: system packages (Linux only)
- detect: `snap`.
- locations: old revisions kept (default 3 revisions of each snap) under `/var/lib/snapd/snaps`; per-user `~/snap/*/` data (user data, not cache).
- scope: global, sudo.
- measure: `snap list --all` lists disabled revisions; `du /var/lib/snapd/snaps`.
- clean: for each disabled revision `sudo snap remove NAME --revision=N`; `sudo snap set system refresh.retain=2` to lower retention (all unverified).
- regenerable: yes (disabled revisions are rollback copies).
- in_use_hazard: only remove disabled revisions.
- sources: none fetched.
- verification: all unverified

## flatpak
- name: flatpak; ecosystem: system packages (Linux only)
- detect: `flatpak`.
- locations: installs in `/var/lib/flatpak` (system) and `~/.local/share/flatpak` (user); app data `~/.var/app` is user data.
- scope: global (system and user).
- measure: `du`.
- clean: `flatpak uninstall --unused` (removes unused runtimes), `flatpak repair` (unverified). `~/.var/app` is not cache.
- regenerable: yes-but-large-download (runtimes)
- in_use_hazard: unused runtimes only.
- sources: none fetched.
- verification: all unverified

## asdf
- name: asdf; ecosystem: version manager
- detect: `asdf` on PATH, `~/.asdf`.
- locations: `$ASDF_DATA_DIR` default `~/.asdf`: `installs/` (installed tool versions: not cache), `downloads/` (cached downloads), `plugins/` (unverified).
- scope: global
- measure: `du ~/.asdf/downloads`.
- clean: no native clean command known (unverified): delete `downloads/` directory; uninstall versions with `asdf uninstall NAME VERSION`.
- regenerable: downloads yes; installs yes-but-slow-rebuild.
- in_use_hazard: do not delete `installs` of versions pinned in `.tool-versions`.
- sources: none fetched.
- verification: all unverified

## mise
- name: mise; ecosystem: version manager
- detect: `mise` on PATH.
- locations: cache directory (`MISE_CACHE_DIR`; defaults macOS `~/Library/Caches/mise`, Linux `~/.cache/mise` — unverified); data dir (installs) `~/.local/share/mise` (unverified). discover: `mise doctor` or `mise cache` subcommands (unverified).
- scope: global
- measure: `du` of the cache dir.
- clean: `mise cache clear` (aliases `c`, `clean`; no args deletes the whole cache directory, cached environment and task artifacts; `mise cache clear node` only that tool's cached data; installed tools are NOT removed — official-docs). `mise prune [-n|--dry-run] [--tools] [--configs] [TOOL]` removes installed tool versions no tracked config uses; keeps versions of running processes (official-docs); `mise ls --prunable` lists them.
- regenerable: cache yes; pruned installs yes-but-large-download/slow rebuild.
- in_use_hazard: `prune` removes versions that no tracked config selects, including ones used ad hoc via `mise exec`.
- sources: https://mise.jdx.dev/cli/cache/clear.html ; https://mise.jdx.dev/cli/prune.html
- verification: discover=unverified; default paths=unverified; clean=official-docs

## AWS CLI / SAM
- name: aws, sam; ecosystem: cloud CLI
- detect: `aws`, `sam` on PATH.
- locations: `~/.aws/cli/cache` (assume-role credential cache, short-lived JSON) and `~/.aws/sso/cache` (SSO tokens: deleting forces re-login) (unverified; docs page fetched did not confirm). Override: none known. `~/.aws/credentials` and `~/.aws/config` are NOT cache. SAM build artifacts `.aws-sam/` per project (marker: dir `.aws-sam` beside `template.yaml`; contains `build/` and `cache/`, unverified).
- scope: global (aws cache); per-project (`.aws-sam`).
- measure: `du` (tiny).
- clean: delete directory; no native command. Practically negligible size; low value.
- regenerable: yes (re-authentication needed for SSO)
- in_use_hazard: deleting SSO cache logs the user out.
- sources: https://docs.aws.amazon.com/cli/v1/userguide/cli-configure-files.html
- verification: all unverified. Candidate to drop (no real growth problem).

## gcloud
- name: gcloud; ecosystem: cloud CLI
- detect: `gcloud` on PATH.
- locations: config dir `~/.config/gcloud` (`CLOUDSDK_CONFIG`) with `logs/` (grows per invocation) and credentials; SDK install dir with component cache/backups (`.install/.backup`) (unverified; fetch was redirected and not read).
- scope: global
- measure: `du ~/.config/gcloud/logs`.
- clean: `gcloud components` has no cache clean (unverified); delete `logs/` directory. Never delete credentials/config.
- regenerable: logs not needed; credentials no (re-login).
- in_use_hazard: none for logs.
- sources: https://cloud.google.com/sdk/gcloud/reference/topic/configurations (redirect not followed)
- verification: all unverified

## Serverless Framework
- name: serverless; ecosystem: cloud tooling
- detect: `serverless` / `sls`.
- locations: per-project `.serverless/` (build packages, zip artifacts; marker `serverless.yml` beside it) and `~/.serverless` (global config/cache) (all unverified).
- scope: per-project
- clean: delete directory `.serverless`. No native command known.
- regenerable: yes-but-slow-rebuild (re-package)
- in_use_hazard: deployment in progress.
- sources: none fetched.
- verification: all unverified

## Git LFS
- name: git-lfs; ecosystem: VCS
- detect: `git lfs version`.
- locations: per-repository `.git/lfs/objects` (default; `lfs.storage` config key overrides — official-docs reference to the key, default path not stated on fetched page); also tmp `.git/lfs/tmp`.
- scope: per-project; marker: `.git/lfs` directory inside a repo.
- measure: `du .git/lfs`; `git lfs prune --dry-run` reports what would be deleted (official-docs).
- clean: `git lfs prune` (deletes old local LFS files; keeps files referenced by current checkout, stashes, recent branches/commits, unpushed commits, other worktrees); `-d/--dry-run`, `-f/--force` (prunes everything except unpushed, implies `--recent`), `--recent`, `-c/--verify-remote`, `--when-unverified=<halt,continue>`, `-v`. Config: `lfs.pruneoffsetdays` (default 3), `lfs.fetchrecentrefsdays`, `lfs.pruneverifyremotealways`, `lfs.pruneremotetocheck` (default origin). Warning in docs: do not prune when repositories share a custom `lfs.storage` directory (official-docs). Without `--verify-remote`, if the only copy of an LFS object is local and it is unpushed it is kept; otherwise prune assumes the remote has it.
- regenerable: yes-but-large-download (git lfs fetch), provided remote has the objects; unpushed LFS objects: no.
- in_use_hazard: shared custom storage across repos.
- sources: https://github.com/git-lfs/git-lfs/blob/main/docs/man/git-lfs-prune.adoc
- verification: discover=official-docs; default paths=unverified (`.git/lfs/objects`); clean=official-docs

## Trivy
- name: trivy; ecosystem: security scanner
- detect: `trivy` on PATH.
- locations: cache dir (vulnerability DB, Java index DB, misconfiguration checks, VEX repositories, scan cache); set by `--cache-dir` (official-docs); default `~/.cache/trivy` on Linux and macOS (unverified); env `TRIVY_CACHE_DIR` (unverified).
- scope: global
- measure: `du`.
- clean: `trivy clean` with `--scan-cache`, `--vuln-db`, `--all` (official-docs); other flags (`--java-db`, `--checks-bundle`) not confirmed.
- regenerable: yes-but-large-download (DB tens to hundreds of MB; rate-limit failures possible when re-downloading from registry).
- in_use_hazard: concurrent scan; air-gapped machines cannot re-download the DB.
- sources: https://trivy.dev/latest/docs/configuration/cache/
- verification: discover=official-docs; default paths=unverified; clean=official-docs

## act
- name: act; ecosystem: CI tooling
- detect: `act` on PATH; needs Docker daemon.
- locations: cached actions and checkouts in `~/.cache/act` (unverified; docs page fetched did not state it); runner images in Docker's image store (`catthehacker/ubuntu:*`, `nektos/act-environments-*`), cleaned by docker image prune; job containers/volumes named `act-*`.
- scope: global
- measure: `du ~/.cache/act`; `docker system df`.
- clean: delete directory; `docker volume rm` for `act-*` volumes (unverified); no native command.
- regenerable: yes-but-large-download
- in_use_hazard: running `act` job.
- sources: https://nektosact.com/usage/index.html (does not cover)
- verification: all unverified

## Contract observations
1. Size is sometimes only known by asking the tool: `docker system df`, `docker buildx du`, `brew cleanup -n` (free-form text with a trailing "approximately X" line), `git lfs prune --dry-run`, `nix store gc --dry-run`, `mise ls --prunable`. `du` of a path is wrong or absent for these. The contract needs a `measure` kind of `command` with a parser (column name, regex or JSON `--format json`) and a unit.
2. One tool, several independently cleanable "parts" that are not directories: Docker images / containers / build cache / networks / volumes each have their own size, own prune command and own safety class. Parts need ids, per-part `measure`, per-part `clean`, and a per-part `safety`/`data_class` (cache vs user-data); the default selection must exclude user-data parts, and a "wipe everything" command (`docker system prune --volumes`, `volume prune -a`) must be a separately flagged, never default action.
3. Commands that can delete user data must be declared on the command, not only on the part: `docker system prune --volumes`, `docker volume prune -a`, `podman system prune --volumes`, `colima delete`, `minikube delete`, `vagrant box remove --force`, `nix-collect-garbage -d`. The contract needs `may_delete_user_data: true` per command so a runner can refuse composite commands.
4. A command needs a precondition: daemon/VM running (docker, podman machine, buildx, kind, k3d, act, colima), or sudo/root (apt, dnf, pacman, snap, flatpak system scope, nix multi-user, MacPorts). Contract fields: `requires_running: [daemon|vm]` with a probe command (`docker info`, `podman machine list`), `requires_privilege: sudo`, and a defined outcome "not measurable / skipped" rather than size 0 (a stopped Podman machine must not look like an empty cache).
5. Reported size is not freed size: `Docker.raw` is sparse (apparent size far larger than allocated size), VM disks only shrink if the app compacts them, and Podman `DISK SIZE` is a limit. Measure must use allocated blocks (`du`, not apparent), and the registry must distinguish `inside_disk_image` size (daemon-reported) from `host_file` size, with a note that freeing inside does not guarantee host shrink.
6. Mixed directories with cache next to state: `.terraform` (only `providers/` and `modules/` are cache; the backend binding is not), `~/.pulumi` (plugins vs credentials), `~/.aws` (cli/cache vs credentials/config), `~/.config/gcloud` (logs vs credentials), `~/.config/helm`/`Library/Preferences/helm` vs cache, `~/.asdf/installs` vs `downloads`, `~/.minikube` (cache vs machines), `~/.kube` (cache vs config). Locations must be subpaths with per-subpath class, and the walker must never offer the parent.
7. Per-project instances found by marker: `.terraform` (beside `*.tf`), `.terragrunt-cache`, `packer_cache`, `.aws-sam`, `.serverless`, `.vagrant` (not cache), `.git/lfs`. The contract needs `scope: per-project`, a marker rule (dir name plus sibling file glob), and a walk-exclusion policy; and the clean action must be able to be per instance (`git lfs prune` runs inside each repo with cwd).
8. Path discovery has an order: explicit env/config override (`HELM_CACHE_HOME`, `HOMEBREW_CACHE`, `TF_PLUGIN_CACHE_DIR`, `PACKER_CACHE_DIR`, `ANSIBLE_HOME`, `LIMA_HOME`, `lfs.storage`, `--cache-dir`), then a discover command (`brew --cache`), then OS defaults. A cache can be disabled by default (Terraform plugin cache, Ansible fact cache): discovery may legitimately return "none".
9. Dry-run is native for many tools (`brew cleanup -n`, `git lfs prune -d`, `nix store gc --dry-run`, `mise prune -n`, `vagrant box prune --dry-run`): the contract should have an optional `preview` command separate from `measure` and `clean`.
10. Two clean tiers (prune-unused vs full wipe) exist per tool and have different safety: `brew cleanup` vs `--scrub`/`rm -rf $(brew --cache)`, `docker system prune` vs `-a`, `mise cache clear` vs `mise prune` (the latter removes installs). Cleaning also may remove things whose absence is not a cache miss (rollback generations in Nix, old versions in pacman cache, Homebrew old versions).
11. Not-a-cache actions mixed into "cleanup" commands must be excluded: `brew autoremove`, `apt-get autoremove`, `dnf autoremove` uninstall packages; `snap`/`flatpak` unused runtime removal changes installed state.
12. Machine-level resources (VMs, clusters) are state, not cache: `podman machine`, `colima`, `minikube`, `kind`, `k3d`, `lima` instances, `vagrant` machines. Only their download caches (`~/Library/Caches/lima`, `~/.minikube/cache`) are cache; instance deletion belongs behind an explicit destructive tier.
13. Dropping candidates with no real cache problem: AWS CLI/SAM (tiny), gcloud (logs only), kubectl (tiny), Serverless (per-project build output only). Several fields for these and for the less-documented tools (OrbStack, k3d, containerd, MacPorts, apt, dnf, pacman, snap, flatpak, asdf, Colima) remain `unverified` and should be re-checked against the tool before they ship.
