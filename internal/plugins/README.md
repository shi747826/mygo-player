# Executable source plugin host (protocol v2)

`mygo-player/internal/plugins` discovers, installs and runs independent executable plugins. It contains no builtin source workers or service SDKs. The public models, JSONL protocol and optional Go server SDK live in `pluginapi`; package layout and wire methods are documented in [the plugin protocol](../../plugins/README.md).

## Discovery and installation

`LoadManager(dataDir)` reads `dataDir/plugins/enabled.json` and discovers packages beside the application executable, in macOS `Resources/plugins`, and in `dataDir/plugins/<id>`. User packages override external distribution packages with the same ID. `Reload()` discovers manifests without starting processes and closes clients whose package or manifest changed. The current application build does not ship source plugins.

A manifest must declare `{"type":"exec","protocolVersion":2,"executable":"my-source"}` as its runtime. IDs are lowercase alphanumeric slugs separated by single hyphens; `detail` and `resolve` capabilities are required. The executable is a relative path within the package; fixed `args` are optional. Invalid manifests, path traversal and symbolic links are rejected. `Manifest.Has` checks a capability. Navigation and filter options use label/value pairs; filters select `channel` or `sort`.

`Manager.Install(path)` accepts a local directory or ZIP. `InstallFrom(ctx,input)` also recognizes GitHub links, and `InstallGitHub(ctx,url)` installs a compatible Release asset. Installation validates the package, does not run it, and persists it as disabled. Package limits are 256 MiB and 10,000 entries. `Uninstall(id)` removes a user package while retaining its separate plugin data. See the protocol document for supported GitHub URL forms and credential handling.

`Manifests()`, `Enabled(id)` and `SetEnabled(id,enabled)` expose configuration. `Manager.Open(ctx,id)` requires an enabled package, starts it on demand and reuses a live client. Disabling closes its process; `Manager.Close()` closes all managed clients. The lower-level `Start(ctx,id,dataDir)` and `Open(ctx,id,dataDir)` launch a discovered package directly and leave client lifetime to the caller.

## Process and protocol lifecycle

The host executes the declared program directly with no shell, using the package directory as its working directory. It provides `MYGO_PLUGIN_DATA_DIR=<dataDir>/plugin-data/<id>` for private state. The process inherits the host environment, including the host-selected `MYGO_PLAYER_FFMPEG` path when available. The plugin writes only JSONL to stdout and logs to stderr. It runs with the current user's permissions.

Startup sends `Ping` and requires protocol version 2, with a ten-second handshake timeout bounded by the caller's context. Requests have unique nonzero IDs; calls may run concurrently and replies may arrive out of order. Lines must be smaller than 8 MiB. A canceled call returns immediately and sends a cancellation message. Process exit or a malformed response fails pending requests. `Client.Close()` closes stdin, allows up to two seconds for cleanup, then terminates and reaps the process; repeated or concurrent closes are supported.

`Source` exposes `Discover`, `Search`, `Rank`, `Detail`, `Seasons`, `Covers`, `ResetDevice`, `Calendar`, `Resolve`, `Release` and `Danmaku` using public `pluginapi` types. `Client` adds `DiscoverWithOptions`, `SearchWithOptions` and `Invoke` for optional extensions. Series and episode IDs are opaque strings. `Resolve` returns a WebView-playable media URL and an optional session ID; `Release` lets the plugin clean up that session. Authentication, decryption, media preparation and proxies stay inside the plugin. Protocol error kinds support `errors.Is`; concrete service SDK errors are not transported.

## Host verification

Offline tests use deterministic protocol workers and temporary packages to cover manifest validation, discovery, installation, enable/disable state, concurrency, cancellation, crashes, message limits, malformed lines, errors and cleanup. The external-module fixture builds a separate executable after the test host has been compiled and exercises the public protocol without rebuilding the host. GitHub fixtures cover asset selection, credential forwarding boundaries, network errors and cancellation.

```sh
go test -race ./internal/plugins
go test . -run '^TestExternalExecutableModule' -v
```

These checks verify host behavior and protocol compatibility. Source-specific service behavior and upstream availability require separate plugin validation.
