# Tool cache catalog: JVM, Android, Apple platforms, Flutter, .NET, Ruby, PHP, editors

Raw facts for the `ccc` registry contract. Sources are official documentation, plus read-only commands. Verification values per field: `official-docs`, `ran-command`, `unverified`. Where the official docs are silent (Xcode folders, Kotlin/Native, Android SDK subfolders, SwiftUI previews, VS Code cache folders, Metro, Carthage paths beyond the Carthage repo docs), the field is marked `unverified` and the secondary source is named. No clean/prune/delete command was executed. Tools covered: 24 (Gradle, Maven, sbt, Ivy, Coursier, Kotlin/Native, Android SDK, Android emulator AVDs, Android Gradle build cache, Xcode, iOS Simulator, CocoaPods, Carthage, SwiftPM, Flutter/Dart pub, React Native Metro, .NET NuGet, .NET workloads, RubyGems, Bundler, rbenv/RVM, Composer, JetBrains, VS Code).

Cross-cutting observations: `dotnet nuget locals all --list` can print `http-cache: ~/.local/share/NuGet/http-cache`, whereas the NuGet docs say `.../v3-cache`; the command output is therefore the source of truth, not the doc default. The help text of `xcrun simctl delete`, `xcrun simctl runtime delete` and `swift package purge-cache` / `reset` was read.

---

## Gradle
- `name`: Gradle; `ecosystem`: JVM
- `detect`: `gradle` on PATH, or `./gradlew` in a project, or directory `~/.gradle` (or `$GRADLE_USER_HOME`) exists.
- `locations`:
  1. **Dependency cache** `caches/modules-2` (downloaded POMs/JARs/metadata). discover: none native; resolve `${GRADLE_USER_HOME:-~/.gradle}`. Override: env `GRADLE_USER_HOME` (also `-g/--gradle-user-home`, not fetched, unverified). Default macOS/Linux: `~/.gradle/caches/modules-2`.
  2. **Other shared caches** `caches/jars-9`, `caches/transforms-3`, per-version `caches/<gradle-version>/{generated-gradle-jars,kotlin-dsl,scripts}` (numeric suffixes change between Gradle versions: glob). Same default root.
  3. **Local build cache** `caches/build-cache-1` (suffix versioned). Same root.
  4. **Wrapper distributions** `wrapper/dists/` (one dir per downloaded Gradle version). Default `~/.gradle/wrapper/dists`.
  5. **Daemon registry and logs** `daemon/<gradle-version>/daemon-<pid>.out.log`. Default `~/.gradle/daemon/`.
  6. **Toolchain JDKs** `jdks/` (JDKs downloaded by toolchain provisioning). Default `~/.gradle/jdks`.
  7. **Per-project** `<project>/.gradle/<version>` and `<project>/build` directories.
- `scope`: items 1-6 `global`; item 7 `per-project`, marker: sibling `settings.gradle` / `settings.gradle.kts` / `build.gradle(.kts)`.
- `measure`: `du -sk` of each resolved subdirectory (no native usage command).
- `clean`: no native full-wipe command. Gradle self-cleans: unused version caches removed after 30 days (release) / 7 days (snapshot), downloaded shared-cache files after 30 days, created ones after 7 days, daemon logs after 14 days, project `.gradle/<ver>` after 7 days; cleanup runs about once per 24h when the daemon stops. Retention is configurable via init script under `$GRADLE_USER_HOME/init.d` (`releasedWrappers`, `snapshotWrappers`, `downloadedResources`, `createdResources`, `buildCache`, `daemonLogs`; `cleanup` = DEFAULT/DISABLED/ALWAYS). Full wipe: `delete directory`, after `gradle --stop` (stops daemons of the same Gradle version only). Per-project: `gradle clean` (build/ only, not verified in fetched page).
- `regenerable`: modules-2 `yes-but-large-download`; transforms/jars/build-cache `yes-but-slow-rebuild`; wrapper/dists `yes-but-large-download` (re-downloaded on next `./gradlew`); daemon logs `yes` (logs are lost); jdks `yes-but-large-download`. Offline machines: `no` in effect.
- `in_use_hazard`: deleting modules-2 or transforms during a build/daemon run corrupts the running build (file locks held by daemons); removing wrapper/dists/<ver> during a wrapper run breaks that run. Stop daemons first (`gradle --stop`; other-version daemons need OS kill).
- `sources`: https://docs.gradle.org/current/userguide/directory_layout.html ; https://docs.gradle.org/current/userguide/gradle_daemon.html ; https://docs.gradle.org/current/userguide/build_cache.html
- `verification`: discover `unverified` (no discover command exists; env var is official-docs); default paths `official-docs`; clean (retention, `--stop`) `official-docs`; `delete directory` is inference.

## Maven
- `name`: Maven local repository; `ecosystem`: JVM
- `detect`: `mvn` on PATH or `~/.m2` exists.
- `locations`: **Local repository** (downloaded artifacts and locally `install`ed artifacts). discover: none printing it directly; read `<localRepository>` in `~/.m2/settings.xml`, else default (`mvn help:evaluate -Dexpression=settings.localRepository -q -DforceStdout` is commonly used, `unverified`). Override: `<localRepository>` in settings.xml (must be absolute); `-Dmaven.repo.local` (not confirmed on the fetched page, `unverified`). Default macOS/Linux: `~/.m2/repository`.
- `scope`: `global`.
- `measure`: `du -sk` of the path.
- `clean`: no full-wipe command; `delete directory`. Prune variant native: `mvn dependency:purge-local-repository` (removes the current project's dependencies and re-resolves them by default; `-DreResolve=false` to only delete; `-DmanualInclude=g:a`; `-DsnapshotsOnly=true`; `-DresolutionFuzziness=file|version|artifactId|groupId`). It is project-scoped, not a whole-repo wipe.
- `regenerable`: `yes-but-large-download`, BUT it also holds artifacts installed locally with `mvn install` that exist nowhere else and private-repo artifacts that may no longer be downloadable: partially `no — contains user data` (locally built artifacts under your own groupIds, deployed-nowhere SNAPSHOTs).
- `in_use_hazard`: running Maven builds fail mid-resolution; IDE (IntelliJ/Eclipse) indexes referencing the repo go stale.
- `sources`: https://maven.apache.org/guides/mini/guide-configuring-maven.html ; https://maven.apache.org/plugins/maven-dependency-plugin/purge-local-repository-mojo.html
- `verification`: discover `unverified`; default path `official-docs`; clean `official-docs` (purge goal), delete-directory inference.

## sbt
- `name`: sbt; `ecosystem`: JVM/Scala
- `detect`: `sbt` on PATH or `~/.sbt` exists; per-project marker `build.sbt` / `project/build.properties`.
- `locations`:
  1. **Global dir** `~/.sbt` (contains `1.0/global.sbt`, `1.0/plugins/`, plus boot dir `~/.sbt/boot` holding downloaded sbt/Scala launchers, boot path from memory so `unverified`). Overrides `sbt.global.base`, `sbt.boot.directory` (property names `unverified`; not on the fetched page). Default `~/.sbt` on both OSes.
  2. **Dependency resolution** is done by Coursier in sbt 1.3+, see Coursier; legacy Ivy cache see Ivy.
  3. **Per-project** `target/` and `project/target`, `project/project`.
- `scope`: 1 global; 3 per-project (marker `build.sbt`).
- `measure`: `du -sk`; in sbt `show csrCacheDirectory` prints the Coursier cache (official-docs).
- `clean`: `sbt clean` (project `target/`); no native global wipe: `delete directory`.
- `regenerable`: `yes-but-slow-rebuild` (boot dir re-downloaded). `~/.sbt/1.0/global.sbt` and `plugins/` are user configuration: `no — contains user data` and must not be deleted.
- `in_use_hazard`: a running sbt server/shell holds `~/.sbt/boot` and `target` locks.
- `sources`: https://www.scala-sbt.org/1.x/docs/Global-Settings.html
- `verification`: discover `official-docs` (`show csrCacheDirectory`, from the Coursier page); default paths: `~/.sbt/1.0` `official-docs`, boot dir `unverified`; clean `unverified` for `sbt clean` (standard, not fetched).

## Ivy
- `name`: Ivy cache (legacy, sbt < 1.3 and Ant/Ivy builds); `ecosystem`: JVM
- `detect`: `~/.ivy2` exists.
- `locations`: `~/.ivy2/cache` (downloaded artifacts), `~/.ivy2/local` (locally published artifacts via `publishLocal`). Override: `sbt.ivy.home` / `-Divy.home` (names `unverified`). Default `~/.ivy2` on both OSes.
- `scope`: `global`. `measure`: `du -sk ~/.ivy2/cache`.
- `clean`: `delete directory` for `cache/` only.
- `regenerable`: `cache/` `yes-but-large-download`; `local/` `no — contains user data` (artifacts published with `publishLocal` exist only there).
- `in_use_hazard`: running sbt/Ivy builds fail resolution.
- `sources`: none found in official docs fetched (the sbt Global-Settings page does not mention Ivy home).
- `verification`: discover `unverified`; default paths `unverified`; clean `unverified`.

## Coursier
- `name`: Coursier (`cs`); `ecosystem`: JVM
- `detect`: `cs` or `coursier` on PATH, or cache dir exists.
- `locations`: **Artifact cache** `.../v1`. discover: inside sbt `show csrCacheDirectory`; no standalone CLI command documented. Override precedence: CLI `--cache`, sbt `coursierCache`, env `COURSIER_CACHE`, Java property `coursier.cache`; sbt also `-Dsbt.coursier.home`. Default macOS: `~/Library/Caches/Coursier/v1`; Linux: `~/.cache/coursier/v1`. Former location `~/.coursier/cache/v1` may still exist on older installs. JVM/launcher caches under the same root (`jvm/` subfolder) are from memory, `unverified`.
- `scope`: `global`. `measure`: `du -sk`.
- `clean`: not documented; `delete directory`.
- `regenerable`: `yes-but-large-download`.
- `in_use_hazard`: running sbt/mill/scala-cli resolution fails or re-downloads.
- `sources`: https://get-coursier.io/docs/cache
- `verification`: discover `official-docs` (sbt only); default paths `official-docs`; clean `unverified`.

## Kotlin/Native (`~/.konan`)
- `name`: Kotlin/Native data dir; `ecosystem`: JVM/Kotlin
- `detect`: `~/.konan` exists.
- `locations`: `$KONAN_DATA_DIR` (default `~/.konan`) holding `dependencies/` (LLVM and sysroot toolchains downloaded from JetBrains CDN), `kotlin-native-prebuilt-<os>-<arch>-<version>/` compilers (one per Kotlin version: glob), and `cache/` (compiler caches). Subdir names from search results of Kotlin discussion/HACKING.md, not the official docs. Default macOS/Linux: `~/.konan`. One user report says the env var is not honored in some Gradle setups.
- `scope`: `global`. `measure`: `du -sk`.
- `clean`: `delete directory` (per-version compiler dirs can be deleted individually).
- `regenerable`: `yes-but-large-download` (multi-GB toolchains).
- `in_use_hazard`: Gradle builds of native targets fail if toolchain removed mid-build.
- `sources`: https://kotlinlang.org/docs/native-get-started.html (confirms CDN download of `konanc` dependencies but gives no path); search result https://discuss.kotlinlang.org/t/change-konan-folder-location/18309
- `verification`: discover `unverified`; default paths `unverified`; clean `unverified`.

## Kotlin daemon
- `name`: Kotlin compile daemon; `ecosystem`: JVM
- Decision: no persistent cache of its own that was verifiable. Logs/runfiles go to a temp dir (`/tmp` or `java.io.tmpdir`), from memory, `unverified`. Treat as dropped from the registry; Gradle `--stop` handles the daemon process. `verification`: all `unverified`.

## Android SDK
- `name`: Android SDK; `ecosystem`: Android
- `detect`: `sdkmanager` or `adb` on PATH, or `$ANDROID_HOME` exists, or `~/Library/Android/sdk` exists (macOS).
- `locations` (all under the SDK root): `system-images/android-<api>/<tag>/<abi>`, `build-tools/<version>`, `platforms/android-<api>`, `ndk/<version>`, `platform-tools`, `emulator`, `cmdline-tools`. discover: env `ANDROID_HOME` (`ANDROID_SDK_ROOT` deprecated); `sdkmanager --list` (with `--sdk_root=<path>`) lists installed and available. Default: macOS `~/Library/Android/sdk` (docs show it only as an example); Linux `~/Android/Sdk` (Android Studio default, `unverified`).
- `scope`: `global`. `measure`: `du -sk` per subdirectory (`sdkmanager --list` shows packages but no sizes).
- `clean`: native uninstall: `sdkmanager --uninstall "<sdk-path>"` e.g. `"system-images;android-34;google_apis;arm64-v8a"` (syntax official; the sample package id is illustrative). `sdkmanager` is marked deprecated in favor of `android sdk remove`. No full wipe.
- `regenerable`: `yes-but-large-download`; but older `platforms`/`build-tools` may be pinned by projects (builds then try to auto-download).
- `in_use_hazard`: Android Studio / Gradle builds and running emulators using the component.
- `sources`: https://developer.android.com/tools/sdkmanager ; https://developer.android.com/tools/variables
- `verification`: discover `official-docs`; default path macOS `official-docs` (example only), Linux `unverified`; clean `official-docs`.

## Android emulator AVDs
- `name`: Android Virtual Devices; `ecosystem`: Android
- `detect`: `avdmanager` / `emulator` on PATH, or `~/.android/avd` exists.
- `locations`: `$ANDROID_AVD_HOME` else `$ANDROID_USER_HOME/avd` else `$HOME/.android/avd` (lookup order official). `*.avd/` directories with `userdata-qemu.img`, `sdcard.img`, snapshots, plus `*.ini` files next to them. discover: `avdmanager list avd` (official; prints path per AVD). Default macOS/Linux `~/.android/avd`. `ANDROID_USER_HOME` defaults to `$HOME/.android`.
- `scope`: `global`. `measure`: `du -sk` per `*.avd`.
- `clean`: `avdmanager delete avd -n <name>` (official; `avdmanager` tagged deprecated in favor of Android CLI `android emulator remove`). Deleting the directory alone leaves the `<name>.ini` pointer behind.
- `regenerable`: `no — contains user data`: installed apps, app data, account state, saved snapshots. The AVD configuration can be recreated; the data cannot.
- `in_use_hazard`: running emulator holds the image locks (`*.lock` dirs); deleting corrupts a running emulator.
- `sources`: https://developer.android.com/studio/command-line/avdmanager ; https://developer.android.com/tools/variables
- `verification`: discover `official-docs`; default paths `official-docs`; clean `official-docs`.

## Android Gradle build cache
- `name`: Android Gradle plugin build cache; `ecosystem`: Android
- Decision: the legacy plugin cache `~/.android/build-cache` was replaced by the Gradle build cache (`caches/build-cache-1`), see Gradle. The official page returned 404 so this is `unverified`; treat as part of the Gradle entry. Also `~/.android/cache` (SDK manager download cache, `unverified`) is regenerable.
- `verification`: all `unverified`.

## Xcode
- `name`: Xcode; `ecosystem`: Apple
- `detect`: `xcode-select -p` prints a path (e.g. `/Applications/Xcode.app/Contents/Developer`, `ran-command`) or `/Applications/Xcode.app` exists. macOS only.
- `locations` (macOS only; none exist on Linux):
  1. **DerivedData** `~/Library/Developer/Xcode/DerivedData` (per-project dirs `<ProjectName>-<hash>`; also `ModuleCache.noindex`, `CompilationCache.noindex` inside it, `ran-command`). Relocatable in Xcode Settings > Locations; the actual configured path is stored in the Xcode defaults (key `IDECustomDerivedDataLocation`, `unverified`). discover: none native.
  2. **Archives** `~/Library/Developer/Xcode/Archives/<date>/*.xcarchive`.
  3. **iOS DeviceSupport** `~/Library/Developer/Xcode/iOS DeviceSupport/<version (build)>` (symbols copied from physical devices); also `watchOS DeviceSupport`, `tvOS DeviceSupport`, `iOS Device Logs` (names from Apple Developer Forums, `unverified`).
  4. **SwiftUI previews / UserData** `~/Library/Developer/Xcode/UserData` (contains `Previews`, `Provisioning Profiles`, `CodeSnippets`, `KeyBindings`, ... from memory, `unverified`). Preview simulators live in CoreSimulator (see iOS Simulator).
  5. **Module cache** is inside DerivedData (`ModuleCache.noindex`); older `~/Library/Caches/com.apple.dt.Xcode*` and `com.apple.dt.xcodebuild` are caches.
  6. **XCTestDevices** `~/Library/Developer/XCTestDevices` and **DVTDownloads** `~/Library/Developer/DVTDownloads`.
- `scope`: `global`. `measure`: `du -sk` per directory; DerivedData per-project subdirs individually.
- `clean`: no native wipe. `xcodebuild clean` cleans a project's build products, not the directory. Delete directory (per-project subdirectory in DerivedData preferred).
- `regenerable`: DerivedData `yes-but-slow-rebuild` (reindexing included); module cache `yes`; DeviceSupport `yes-but-large-download` (re-copied from device on next connect; needs the device); Archives `no — contains user data`: release builds and dSYMs needed to symbolicate crash reports, Apple says to retain the archive of every distributed build; UserData `no — contains user data` (snippets, key bindings, provisioning profiles).
- `in_use_hazard`: Xcode and `xcodebuild` running: deleting DerivedData while building produces cryptic errors and loses indexes; Xcode must be quit first.
- `sources`: no official Apple documentation page found for these folders; secondary: https://developer.apple.com/forums/thread/768385 (DeviceSupport), web search results about Archives/dSYM retention.
- `verification`: discover `ran-command` for `xcode-select -p`; default paths DerivedData `ran-command` (contents verified), DeviceSupport/Archives `unverified`, clean `unverified`.

## iOS Simulator
- `name`: iOS Simulator (CoreSimulator); `ecosystem`: Apple
- `detect`: `xcrun simctl` works (`ran-command`).
- `locations` (macOS only):
  1. **Devices** `~/Library/Developer/CoreSimulator/Devices/<UUID>/` (each holds `device.plist` and `data/` with installed app containers and user data). Also `~/Library/Developer/CoreSimulator/Caches`. discover: `xcrun simctl list devices` (`ran-command`).
  2. **Runtimes** on recent macOS: stored as disk images, under `/Library/Developer/CoreSimulator/` (`Images/`, `Volumes/`, `Cryptex/`, `Profiles/`, `Caches/`, `ran-command` listing). `xcrun simctl runtime list` prints them (output not run).
  3. Older runtimes as bundles under `/Library/Developer/CoreSimulator/Profiles/Runtimes/*.simruntime` (from memory, `unverified`).
- `scope`: `global`. `measure`: `du -sk` on Devices; for runtimes `xcrun simctl runtime list` (reports sizes in recent Xcode, `unverified`) or `du` of `/Library/Developer/CoreSimulator/Images` and `Volumes`.
- `clean` (all read from help text, `ran-command`):
  - `xcrun simctl delete unavailable` deletes devices not supported by the current Xcode SDK (usage line: `simctl delete <device> [...] | unavailable | all`).
  - `xcrun simctl delete all` deletes every device (destroys data).
  - `xcrun simctl erase <device|all>` resets a device to factory state.
  - `xcrun simctl runtime delete <identifier>` deletes a runtime; `--notUsedSinceDays <days>` (alias `-d`) deletes runtimes unused for N days; `--dry-run` / `-n` prints what would be deleted; `--keep-asset` / `-k`; alias `all` deletes all images. Shuts down booted simulators on the runtime. May need to run as the user who owns the runtime; sudo not stated in help.
- `regenerable`: runtimes `yes-but-large-download` (several GB each); devices `no — contains user data`: apps installed in the simulator and their data (databases, keychain, test accounts); `delete unavailable` only touches devices of runtimes no longer present, but those are still user data.
- `in_use_hazard`: booted simulators; Xcode running tests/previews; `runtime delete` shuts down booted devices.
- `sources`: no Apple web page fetched; the commands' own `help` output was used. 
- `verification`: discover `ran-command` (`list` exists; `xcrun simctl help delete` and `runtime help` read); default paths `ran-command` for `~/Library/Developer/CoreSimulator` and `/Library/Developer/CoreSimulator`; clean `ran-command` (help text only, never executed).

## CocoaPods
- `name`: CocoaPods; `ecosystem`: Apple
- `detect`: `pod` on PATH or `~/.cocoapods` / `~/Library/Caches/CocoaPods` exists.
- `locations`:
  1. **Pod cache** `~/Library/Caches/CocoaPods` (downloaded pod sources/specs). Override: env `CP_HOME_DIR` moves both caches and repos. discover: `pod cache list` (shows tree; `--short` prints relative paths). macOS default `~/Library/Caches/CocoaPods`; Linux `~/.cache/CocoaPods` (`unverified`).
  2. **Spec repos** `~/.cocoapods/repos` (git clones of specs; the CDN `trunk` is lightweight; legacy `master` clone is multi-GB). discover: `pod repo list`.
  3. **Per-project** `Pods/` directory next to `Podfile` (checked-in or not by choice), `Podfile.lock` (never delete).
- `scope`: 1-2 `global`; 3 `per-project`, marker sibling `Podfile`.
- `measure`: `du -sk`; `pod cache list` for contents; `pod repo list --count-only` counts repos only.
- `clean`: `pod cache clean --all` (removes all cached pods without asking); `pod cache clean <NAME>` for one pod (interactive when several versions); `pod repo remove <NAME>` deletes a spec repo; `pod repo update` refreshes. Per-project: `rm -rf Pods` then `pod install`.
- `regenerable`: cache `yes-but-large-download`; spec repos `yes-but-large-download`; `Pods/` `yes` (needs `pod install` and network); `Podfile.lock` `no — contains user data` (pinned versions).
- `in_use_hazard`: running `pod install`; Xcode open on a workspace referencing `Pods/`.
- `sources`: https://guides.cocoapods.org/terminal/commands.html ; https://guides.cocoapods.org/using/faq.html
- `verification`: discover `official-docs`; default paths `official-docs` (macOS), Linux `unverified`; clean `official-docs`.

## Carthage
- `name`: Carthage; `ecosystem`: Apple
- `detect`: `carthage` on PATH or `~/Library/Caches/org.carthage.CarthageKit` exists.
- `locations`: global `~/Library/Caches/org.carthage.CarthageKit` (bare git repos and downloaded binaries; safe to delete whole dir or any subfolder, repopulated by next `carthage checkout`); per-project `Carthage/Checkouts` and `Carthage/Build`. macOS only. No override documented.
- `scope`: global + per-project (marker: sibling `Cartfile` / `Cartfile.resolved`).
- `measure`: `du -sk`. `clean`: `delete directory` (no native command).
- `regenerable`: global cache `yes-but-large-download`; `Carthage/Build` `yes-but-slow-rebuild`; `Carthage/Checkouts` `yes` unless it uses git submodules (then it holds the user's submodule state).
- `in_use_hazard`: running `carthage` operations.
- `sources`: https://github.com/Carthage/Carthage/blob/master/Documentation/Artifacts.md
- `verification`: discover `unverified`; default paths `official-docs`; clean `official-docs` (manual delete stated).

## Swift Package Manager
- `name`: SwiftPM; `ecosystem`: Apple/Swift (also Linux)
- `detect`: `swift` on PATH.
- `locations`:
  1. **Global cache** macOS `~/Library/Caches/org.swift.swiftpm` (`ran-command`: contains `artifacts/`, `manifests/`, `repositories/`); Linux `~/.cache/org.swift.swiftpm` (`unverified`). Override: `swift package --cache-path <path>` (listed in help). 
  2. **Global config/security** macOS `~/Library/org.swift.swiftpm` (`ran-command`: `configuration/`, `security/`, `collections.json`, `fingerprints/`); Linux `~/.swiftpm` (`unverified`). Configuration and security data is user settings, not cache.
  3. **Per-project** `.build` (default scratch path per help, `--scratch-path`), also `.swiftpm/` and `Package.resolved` in project.
- `scope`: 1-2 `global`; 3 `per-project`, marker: sibling `Package.swift`.
- `measure`: `du -sk`.
- `clean` (read from `swift package --help`, `ran-command`): `swift package purge-cache` ("Purge the global repository cache"); `swift package reset` ("Reset the complete cache/build directory"); `swift package clean` (clears build artifacts, name from help list not fully read). Xcode's File > Packages > Reset Package Caches is the GUI equivalent.
- `regenerable`: caches `yes-but-large-download`; `.build` `yes-but-slow-rebuild`; `Package.resolved` `no — contains user data` (pinned versions); `~/Library/org.swift.swiftpm/configuration|security` `no` (user settings, trust fingerprints).
- `in_use_hazard`: running `swift build`/Xcode resolving packages; Xcode keeps its own package checkouts inside DerivedData (`SourcePackages`, `unverified`).
- `sources`: `swift package --help`. Official repo docs fetch returned 404; secondary: Swift forums pitch giving `~/Library/Caches/org.swift.swiftpm`.
- `verification`: discover `unverified` (no discover command; `--cache-path` only sets); default paths macOS `ran-command`, Linux `unverified`; clean `ran-command` (help text).

## Flutter / Dart pub cache
- `name`: Dart pub cache; `ecosystem`: Flutter/Dart
- `detect`: `flutter` or `dart` on PATH, or `~/.pub-cache` exists.
- `locations`:
  1. **Pub cache** `hosted/`, `git/`, `bin/` (activated global tool launchers), `global_packages/`. Override env `PUB_CACHE`. Default macOS/Linux `$HOME/.pub-cache` (official); Windows `%LOCALAPPDATA%\Pub\Cache`.
  2. **Flutter SDK cache** `<flutter-sdk>/bin/cache` (engine artifacts; populated by `flutter precache`) (`unverified` path).
  3. **Per-project** `build/` and `.dart_tool/`, marker sibling `pubspec.yaml`.
- `scope`: 1-2 `global`; 3 `per-project`.
- `measure`: `du -sk`. discover: none documented printing it (resolve `$PUB_CACHE` else `$HOME/.pub-cache`).
- `clean`: `dart pub cache clean` (or `flutter pub cache clean`): "empty the entire system cache" (since Dart 2.14; official; the `--force` flag is `unverified`, may prompt); `dart pub cache repair` re-downloads all packages; `flutter clean` deletes `build/` and `.dart_tool/` (official). `dart pub cache gc` appears in search/general knowledge only: `unverified`.
- `regenerable`: `hosted/` `yes-but-large-download`; but `pub cache clean` also removes `bin/` and `global_packages/` i.e. globally activated tools (`dart pub global activate`), which need re-activation: partial `no` (the activation list is lost). `git/` deps `yes` unless repository gone.
- `in_use_hazard`: running `dart`/`flutter` commands, analysis server (IDE) may break until `pub get`.
- `sources`: https://dart.dev/tools/pub/cmd/pub-cache ; https://dart.dev/tools/pub/environment-variables ; https://docs.flutter.dev/reference/flutter-cli
- `verification`: discover `unverified`; default paths `official-docs`; clean `official-docs` (clean, repair, flutter clean), `gc` and `--force` `unverified`.

## React Native Metro
- `name`: Metro bundler cache; `ecosystem`: React Native/JS
- `detect`: `package.json` containing `react-native` and `metro.config.js`.
- `locations`: Metro's file cache store (`FileStore`) under `os.tmpdir()` e.g. `$TMPDIR/metro-cache` on macOS (in docs only as an example `root`, not stated as default: `unverified`). Also `$TMPDIR/haste-map-*` and `$TMPDIR/react-native-packager-cache-*` (older versions, from memory). Also `<project>/node_modules/.cache`. Per-project config `cacheStores` can redirect.
- `scope`: `global` (in temp dir) but tied to project through config.
- `measure`: `du -sk` of temp dir matches (`metro-cache`, `haste-map-*`).
- `clean`: `npx react-native start --reset-cache` / `npx expo start --clear` (reset on next start; flag names from memory `unverified`; Metro's own docs did not mention `resetCache`). Otherwise delete directory.
- `regenerable`: `yes-but-slow-rebuild`.
- `in_use_hazard`: running Metro dev server (cache dir rewritten).
- `sources`: https://metrobundler.dev/docs/caching/ (example config only)
- `verification`: discover `unverified`; default paths `unverified`; clean `unverified`.

## .NET NuGet
- `name`: NuGet locals; `ecosystem`: .NET
- `detect`: `dotnet` (or `nuget`) on PATH; directory `~/.nuget` exists.
- `locations` (macOS/Linux; each independent):
  1. **global-packages** (expanded packages used directly by PackageReference restores). Override env `NUGET_PACKAGES`, config `globalPackagesFolder`, MSBuild `RestorePackagesPath`. Default `~/.nuget/packages`.
  2. **http-cache** (feed responses, 30-minute expiry). Override `NUGET_HTTP_CACHE_PATH`. Default per docs `~/.local/share/NuGet/v3-cache`, but the actual value on some SDK versions is `~/.local/share/NuGet/http-cache` (`ran-command`).
  3. **temp** (also inter-process lock coordination). Override `NUGET_SCRATCH`. Default macOS `$TMPDIR/NuGetScratch` (observed `/var/folders/.../T/NuGetScratch`); Linux `/tmp/NuGetScratch<username>`.
  4. **plugins-cache** (NuGet 4.8+). Override `NUGET_PLUGINS_CACHE_PATH`. Default `~/.local/share/NuGet/plugins-cache` (macOS and Linux).
- `discover`: `dotnet nuget locals all --list` (or one of `http-cache|global-packages|temp|plugins-cache` instead of `all`); `ran-command` output: `http-cache: ...`, `global-packages: ...`, `temp: ...`, `plugins-cache: ...`. Note some SDK versions prefix lines with `info :`.
- `scope`: `global`. `measure`: `du -sk` of each discovered path.
- `clean`: `dotnet nuget locals all --clear` or per location: `dotnet nuget locals {http-cache|global-packages|temp|plugins-cache} --clear`. No native "prune unused" (NuGet GitHub issue 4980 open). Notes: after clearing global-packages every project must restore again; locks fail if VS or a build holds files.
- `regenerable`: `yes-but-large-download` (global-packages); http-cache and plugins-cache `yes`; temp `yes`. Local feeds / `packages.config` offline packages may be unrecoverable: `no` for packages that exist only from a local source feed.
- `in_use_hazard`: running restore/build/IDE gives "file in use" errors, and parallel clearing can break a concurrent restore.
- `sources`: https://learn.microsoft.com/en-us/nuget/consume-packages/managing-the-global-packages-and-cache-folders
- `verification`: discover `ran-command`; default paths `official-docs` + `ran-command` (mismatch noted for http-cache); clean `official-docs`.

## .NET workloads
- `name`: .NET SDK workloads; `ecosystem`: .NET
- `locations`: workload packs under the dotnet root `packs/`, `sdk-manifests/`, `metadata/workloads/`, `template-packs/` (from memory; dotnet root on macOS typically `/usr/local/share/dotnet` or `/opt/homebrew/...`). 
- `discover`: `dotnet --info` (root) and `dotnet workload list` (`unverified`: not fetched).
- `clean`: `dotnet workload clean` (removes all workload packs, `unverified`; may need `sudo` when dotnet is installed system-wide). `dotnet workload uninstall <id>`.
- `regenerable`: `yes-but-large-download`.
- `in_use_hazard`: Visual Studio for Mac / MAUI/iOS builds in progress.
- `verification`: all `unverified` (no workload doc fetched).

## RubyGems
- `name`: RubyGems; `ecosystem`: Ruby
- `detect`: `gem` on PATH.
- `locations`: **Gem install dir** (installed gems, `cache/*.gem` files, docs); discover: `gem env gemdir` (alias for `gem environment home`) and `gem env gempath` (official); `ran-command` (e.g. macOS system Ruby: `/Library/Ruby/Gems/2.6.0` and user dir `~/.gem/ruby/2.6.0`). Override env `GEM_HOME`, `GEM_PATH`. Defaults vary by Ruby manager (system, rbenv `~/.rbenv/versions/<v>/lib/ruby/gems/<abi>`, RVM `~/.rvm/gems/...`, Homebrew Ruby), so always run discover.
- `scope`: `global` (per Ruby version).
- `measure`: `du -sk $(gem env gemdir)`; separate `cache/` subdir holds `.gem` archives.
- `clean`: prune variant `gem cleanup` removes old versions of gems not required by dependencies from GEM_HOME; `gem cleanup -n` is dry run (`-n, -d, --dryrun` printed by help; docs list `--dry-run`); `gem cleanup <name>` for one gem; `--[no-]user-install`. Full wipe: `delete directory` (breaks installed gems and bundler apps).
- `regenerable`: `yes-but-large-download`/native extensions need compile: `yes-but-slow-rebuild`.
- `in_use_hazard`: running Ruby apps/Rails servers load gems from the dir.
- `sources`: https://guides.rubygems.org/command-reference/#gem-cleanup
- `verification`: discover `ran-command`; default paths `ran-command`/`official-docs`; clean `official-docs` + `ran-command` (help text).

## Bundler
- `name`: Bundler; `ecosystem`: Ruby
- `detect`: `bundle` on PATH; per-project marker `Gemfile`/`Gemfile.lock`.
- `locations`: gems normally install into the RubyGems dir (shared); with `bundle config path` gems go to a project dir (commonly `vendor/bundle` or `.bundle`); `vendor/cache` holds `.gem` files from `bundle cache` (checked-in by choice). discover: `bundle config get path` (`ran-command`: command runs; output is empty when unset), `bundle exec gem env gemdir`.
- `scope`: `per-project` for `vendor/bundle`, `vendor/cache`, `.bundle`; marker sibling `Gemfile`.
- `clean`: `bundle clean` ("Cleans up unused gems in your bundler directory"; options `--dry-run`, `--force`; `ran-command` help). Without a configured `path` it refuses unless `--force`, because it would uninstall every gem in the system dir not used by this Gemfile: hazard for shared gem dirs.
- `regenerable`: `vendor/bundle` `yes-but-slow-rebuild`; `vendor/cache` `no` if relied on for offline deploys.
- `in_use_hazard`: running app servers; with `--force` on the system gem dir other projects' gems vanish.
- `sources`: `bundle help clean`; official man pages bundler.io returned empty content.
- `verification`: discover `ran-command`; default paths `unverified`; clean `ran-command` (help text).

## rbenv / RVM installed Ruby versions
- `name`: rbenv, RVM; `ecosystem`: Ruby
- `detect`: `rbenv` or `rvm` on PATH; `~/.rbenv` or `~/.rvm` exists.
- `locations`:
  - rbenv: `$(rbenv root)/versions/<version>` (each is a full Ruby install with its gems inside; glob by version name). Override env `RBENV_ROOT`. Default `~/.rbenv` (both OSes). discover: `rbenv root`, `rbenv versions`. ruby-build tarball cache `~/.rbenv/cache` (from memory, `unverified`; env `RUBY_BUILD_CACHE_PATH`).
  - RVM: `~/.rvm/rubies/`, `~/.rvm/src/`, `~/.rvm/archives/`, `~/.rvm/gems/` (gemsets), `~/.rvm/log/` (only `rubies`, `src` confirmed by docs). System-wide install at `/usr/local/rvm` (`unverified`).
- `scope`: `global`.
- `clean`: rbenv: `rbenv uninstall <version>` (provided by the ruby-build plugin, official; `-f` flag unverified). RVM: `rvm remove <version>` (preferred; removes ruby, source, aliases, wrappers, env; `--archive` also deletes the downloaded archive, `--gems` deletes gemsets) versus `rvm uninstall <version>` (only the rubies folder). `rvm cleanup all` mentioned in no fetched docs: `unverified`.
- `regenerable`: `yes-but-slow-rebuild` (compile from source). Gems installed only in that version are lost; the global default version file `~/.rbenv/version` / `.ruby-version` references remain (dangling pointer).
- `in_use_hazard`: removing the version a running process, shim or a `.ruby-version` is using breaks that shell/app; removing the currently global version breaks every shim.
- `sources`: https://github.com/rbenv/rbenv/blob/master/README.md ; https://rvm.io/rubies/removing
- `verification`: discover `official-docs` (`rbenv root`/`versions`); default paths `official-docs`; clean `official-docs`.

## PHP Composer
- `name`: Composer; `ecosystem`: PHP
- `detect`: `composer` on PATH or `~/.composer` / `~/.cache/composer` exists.
- `locations`:
  1. **Cache dir** (downloaded dist archives + VCS clones + repo metadata). discover: `composer config cache-dir -g` (the page says cache dir is "configurable via the `cache-dir` option"; the exact CLI call is `unverified` in the fetched page). Override env `COMPOSER_CACHE_DIR`. Default: macOS `$COMPOSER_HOME/cache` i.e. `~/.composer/cache`; Linux with XDG `$XDG_CACHE_HOME/composer` (usually `~/.cache/composer`), otherwise `~/.composer/cache`.
  2. **Composer home** (global config, global packages) holds user data: discover `composer config --global home` (official). Override `COMPOSER_HOME`. Default macOS `~/.composer`; Linux XDG `$XDG_CONFIG_HOME/composer` else `~/.composer`.
  3. **Per-project** `vendor/`, marker sibling `composer.json`.
- `scope`: 1-2 `global`; 3 `per-project`.
- `measure`: `du -sk` of cache dir.
- `clean`: `composer clear-cache` (aliases `clearcache`, `cc`): "Deletes all content from Composer's cache directories"; `composer clear-cache --gc`: garbage collection only (prune variant). Home dir has no native clean.
- `regenerable`: cache `yes-but-large-download`; `COMPOSER_HOME` `no — contains user data` (`config.json`, `auth.json` credentials, globally installed packages).
- `in_use_hazard`: running `composer install/update` concurrently; low.
- `sources`: https://getcomposer.org/doc/03-cli.md
- `verification`: discover `official-docs` (home) / `unverified` (cache-dir exact CLI); default paths `official-docs`; clean `official-docs`.

## JetBrains IDEs
- `name`: JetBrains IDEs (IntelliJ IDEA, PyCharm, WebStorm, GoLand, DataGrip, Android Studio uses Google-named dir); `ecosystem`: Editors
- `detect`: `~/Library/Caches/JetBrains` (macOS) / `~/.cache/JetBrains` (Linux) exists; products not necessarily on PATH.
- `locations` (per product and per version, directory name `<Product><Year.Minor>` e.g. `IntelliJIdea2026.2`, glob required):
  1. **System dir (caches, indexes, local history)**: macOS `~/Library/Caches/JetBrains/<Product><ver>`; Linux `~/.cache/JetBrains/<Product><ver>`. Override `idea.system.path`.
  2. **Logs**: macOS `~/Library/Logs/JetBrains/<Product><ver>`; Linux `~/.cache/JetBrains/<Product><ver>/log`. Override `idea.log.path`.
  3. **Config**: macOS `~/Library/Application Support/JetBrains/<Product><ver>`; Linux `~/.config/JetBrains/<Product><ver>`. Override `idea.config.path`. USER DATA (settings, keymaps).
  4. **Plugins**: macOS `.../Application Support/JetBrains/<Product><ver>/plugins`; Linux `~/.local/share/JetBrains/<Product><ver>`. Override `idea.plugins.path`.
- discover: IDE-only: Help > Diagnostic Tools > Special Files and Folders (no CLI).
- `scope`: `global`. `measure`: `du -sk` per product/version directory.
- `clean`: no CLI. GUI: File > Invalidate Caches (removes cache files for all projects in that IDE version; Local History kept unless ticked). `delete directory` for caches and logs of IDE versions no longer installed (stale-version directories are the main space win). 
- `regenerable`: system dir `yes-but-slow-rebuild` (reindexing); BUT Local History lives in system dir (`no — contains user data` for uncommitted history); logs `yes`; config and plugins must not be deleted (settings, installed plugins).
- `in_use_hazard`: IDE running holds indexes open; delete only when IDE is closed. Orphaned versions are safe to remove.
- `sources`: https://www.jetbrains.com/help/idea/directories-used-by-the-ide-to-store-settings-caches-plugins-and-logs.html ; https://www.jetbrains.com/help/idea/invalidate-caches.html
- `verification`: discover `official-docs` (GUI only); default paths `official-docs` + `ran-command` (macOS); clean `official-docs`.

## VS Code
- `name`: Visual Studio Code; `ecosystem`: Editors
- `detect`: `code` on PATH or user-data dir exists.
- `locations`:
  1. **User data root** macOS `~/Library/Application Support/Code`; Linux `~/.config/Code` (official via `settings.json` path `.../Code/User/settings.json`). Override `--user-data-dir <dir>`. Contains `User/` (settings, snippets, keybindings: user data) and many cache-like subdirs: `Cache`, `CachedData`, `CachedExtensionVSIXs`, `Code Cache`, `GPUCache`, `logs`, `Crashpad`, `Service Worker`, `User/workspaceStorage/<hash>`, `User/globalStorage`, `User/History` (names from memory/knowledge: all `unverified`).
  2. **Extensions** `~/.vscode/extensions` (official, both OSes). Override `--extensions-dir` or env `VSCODE_EXTENSIONS`. discover: `code --list-extensions [--show-versions]` (official).
  3. **Workspace storage** `User/workspaceStorage/<hash>` per opened workspace: UNSAVED/hot-exit state, search history, extension per-workspace state: `no — contains user data`. Hash dirs map to workspaces via `workspace.json` inside (`unverified`), allowing orphan detection.
- `scope`: `global`.
- `measure`: `du -sk` per subdirectory.
- `clean`: no native cache clean. Extensions: `code --uninstall-extension <publisher.extension>` (official); extension list is user data. Cache subfolders: `delete directory` while VS Code is closed.
- `regenerable`: `Cache`, `CachedData`, `Code Cache`, `GPUCache`, `CachedExtensionVSIXs`, `logs`: `yes`; extensions `yes-but-large-download` (reinstall by id; marketplace needed); `User/` settings, snippets, keybindings `no — contains user data`; `workspaceStorage` and `History` `no — contains user data` (unsaved editor state, local history).
- `in_use_hazard`: running VS Code: deleting cache dirs while running can crash windows or lose unsaved hot-exit state; deleting extensions in use breaks language servers.
- `sources`: https://code.visualstudio.com/docs/configure/settings ; https://code.visualstudio.com/docs/editor/command-line ; https://code.visualstudio.com/docs/editor/extension-marketplace
- `verification`: discover `official-docs`; default paths: root and extensions `official-docs`, cache subfolders `unverified`; clean `official-docs` (extension uninstall), cache deletion `unverified`.

---

## Contract observations

1. **One tool, many locations of different safety.** Xcode: DerivedData (safe), Archives (user data: release builds and dSYMs), DeviceSupport (large, needs device to regenerate), UserData (snippets, key bindings). JetBrains: caches (safe), local history inside the same dir (user data), config and plugins (never). VS Code: Cache/CachedData (safe), workspaceStorage and History (user data), `User/` settings. Each location needs its own `safety` class, `regenerable` value and `default_selected` flag, and a tool-level "clean all" must not be the union.
2. **Discover command vs. documented default can disagree.** NuGet docs say `v3-cache`, the command can print `http-cache`; gem dir depends on the Ruby manager; Composer/Coursier/pub differ per OS and XDG. The registry must prefer a `discover` command, and fall back to ordered candidate paths with env-var override, and record the source (`discover`, `env`, `default`) per resolved path. Several tools (Gradle, Maven, Kotlin/Native, JetBrains) have NO discover command: need `env`+`config key`+default resolution (Maven: parse `settings.xml`).
3. **Versioned/glob directory names.** `caches/modules-2`, `build-cache-1`, `jars-9`, `transforms-3` (numeric suffixes change); `wrapper/dists/<ver>`; JetBrains `<Product><Year.Minor>`; `~/.rbenv/versions/<v>`; Android `system-images/android-<api>/...`, `build-tools/<ver>`; Xcode DerivedData `<Project>-<hash>`; VS Code `workspaceStorage/<hash>`; Kotlin/Native per-version compiler dirs. Entries need glob patterns plus per-child granularity (size and clean per matched child, e.g. delete only versions not in use, or not used for N days).
4. **Per-project locations with markers.** `.build` + `Package.swift`, `Pods` + `Podfile`, `Carthage/Build` + `Cartfile`, `build`/`.dart_tool` + `pubspec.yaml`, `vendor` + `composer.json`, `vendor/bundle` + `Gemfile`, `target` + `build.sbt`, project `.gradle` + `settings.gradle(.kts)`. Needs `walk_markers` with sibling-file match, and directories named `build`/`vendor`/`target` that are NOT caches without the marker. Also files that must never be deleted next to them (`Package.resolved`, `Podfile.lock`, `Gemfile.lock`).
5. **Native commands with scope, dry-run and age filters.** `xcrun simctl runtime delete --notUsedSinceDays N --dry-run`, `simctl delete unavailable`, `gem cleanup -n`, `bundle clean --dry-run`, `composer clear-cache --gc`, `pod cache clean <name>`, `mvn dependency:purge-local-repository` (project-scoped, re-downloads by default), `dotnet nuget locals <name> --clear`. The contract needs a list of clean actions per location (`full`, `prune`, with `args`, `dry_run_args`, optional `min_age_days` parameter), `needs_sudo`, `needs_stopped: [process names]`, and an action whose effect is not a deletion of the measured path (e.g. `rbenv uninstall`, `avdmanager delete avd`, `sdkmanager --uninstall`).
6. **Destructive-by-name commands that wipe user data.** `simctl delete all`, `simctl erase`, `avdmanager delete avd`, `pub cache clean` (also removes globally activated tools), `bundle clean --force` on a shared gem dir, `pub cache clean` and `rvm remove --gems`. The registry needs a `destructive_confirm` requirement and a flag that the command is `not_cache` (user data) so `ccc` can refuse in non-interactive mode.
7. **Platform-only entries.** Xcode, iOS Simulator, CocoaPods cache path, Carthage, `~/Library/*` are macOS only; SwiftPM and JetBrains have different Linux paths; Linux uses XDG vars. Each location needs per-OS path lists, with `macOS only` allowed, and a `detect` that is OS-aware.
8. **Process-in-use hazards are first-class data.** Gradle daemons (`gradle --stop` only stops same-version daemons), Xcode/xcodebuild, running simulators, emulators, IDEs, Metro, sbt, Ruby servers. The contract needs `blocking_processes` for refusing or warning, and a `stop_command` where one exists.
9. **Tools without any native command and tools that self-clean.** Gradle self-prunes by age with its own retention; Maven, Coursier, Ivy, Kotlin/Native, Carthage, VS Code caches, Metro have none. `native_clean: none -> delete directory` must be explicit, and the entry may carry `self_cleaning: true` to deprioritize it.
10. **Shared/system locations outside the home directory.** Simulator runtimes at `/Library/Developer/CoreSimulator/*`, system Ruby gems `/Library/Ruby/Gems`, dotnet workloads under the dotnet root, rbenv/RVM system installs. Needs `owner: user|system` and `needs_sudo`, and these should be reported with ownership evidence rather than deleted by directory.
11. **Redirected locations.** Because tools can be relocated by env vars, `ccc` must treat the discovered path as authoritative and must never assume `~`. Several tools have the SAME data accessible at two paths (Coursier old `~/.coursier/cache` vs new). The registry needs path de-duplication across entries (sbt/Ivy/Coursier share the dependency problem; Android build cache folds into Gradle).
12. **Weak evidence is a first-class field.** Many fields here are `unverified` (Xcode folders, Kotlin/Native, Metro, workloads, VS Code cache subfolders, Linux SwiftPM). The registry should carry a `verification` status per field so unverified clean actions are shown but not enabled by default.
