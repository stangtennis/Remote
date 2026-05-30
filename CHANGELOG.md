# Changelog

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
