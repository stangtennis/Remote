# Changelog

## v3.1.86 - 2026-05-31
- Add remote `enable_relay` and `disable_relay` pending commands for agents.
- Allow an online agent to switch TURN relay-only mode without physical access.

## v3.1.85 - 2026-05-31
- Add `RD_FORCE_RELAY=1` support on the agent so VPN/CGNAT hosts can advertise TURN relay candidates only.
- Keep controller relay mode from v3.1.84 and make the VPN path configurable on both ends.

## v3.1.84 - 2026-05-31
- Add optional controller relay-only mode with `RD_FORCE_RELAY=1` for VPN/CGNAT devices such as Mullvad.
- Pass relay-only ICE policy through native viewer, CLI, and Wails controller connection paths.
- Keep default ICE behavior unchanged unless relay-only mode is explicitly enabled.

## v3.1.83 - 2026-05-30
- Fix macOS controller updater downloads when the update server provides raw universal binaries.
- Verify controller updates with inline SHA256 hashes from `version.json`, including macOS hashes.
- Preserve macOS update arguments containing spaces in `/Applications/... .app` paths.
- Add macOS `.tar.gz` download archives for dashboard/manual installs while keeping raw binaries for auto-update.

## v3.1.82 - 2026-05-30
- Fix dashboard H.264 playback by routing video over a dedicated data channel and rendering through a browser video element.
- Reset H.264 encoder state when frame dimensions change to avoid frozen/stretched frames.
- Use constrained-baseline H.264 settings for better browser compatibility.
- Publish updated agent/controller builds and update-server artifacts.

## v3.1.81 - 2026-05-21
- Align agent and controller on the same version.
- Rebuild and publish matching Windows installers.
- Publish GitHub Release `v3.1.81` and sync update-server artifacts.

## v3.1.80 - 2026-05-21
- Rebuild controller with `desktop,production` tags to fix broken startup build.

## v3.1.79 - 2026-05-21
- Fix controller H.264 mode toggle to use configured bitrate instead of a hardcoded 32 Mbps value.

## v3.1.78 - 2026-05-21
- Update controller release and updater/streaming fixes.
- Keep agent and controller release track aligned during rollout.

## v3.1.77 - 2026-05-20
- Agent release before final `v3.1.81` alignment.

## v3.1.75 - 2026-05-21
- Fix update check URL to use `updates.hawkeye123.dk`.

## v3.1.74 - 2026-05-21
- Fix update check URL after `downloads.hawkeye123.dk` mismatch.

## v3.1.73 - 2026-05-20
- Fix auto-login spam.
- Tune H.264 CBR bitrate behavior.

## v3.1.70 - 2026-05-13
- Reset login screen with Escape before typing full username.
- Add saved-login debug logging.
