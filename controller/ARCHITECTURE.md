# Controller Architecture

## Primary Controller

The maintained desktop controller is the Wails application launched from `main.go`.
Its UI lives in `frontend/`, with the active remote viewer implemented in
`frontend/js/viewer.js` and Go methods exposed through `app.go`.

Wails is the release path for current Windows controller builds and the place
where new dashboard-like viewer work should land.

## Legacy Native Fyne Code

The `internal/viewer/`, `internal/webrtc/`, `internal/ui/`, `internal/filebrowser/`,
and `internal/filetransfer/` packages still contain the older native Fyne
controller implementation. That code remains buildable because some tests and
supporting packages still reference it, but it is not the primary product path.

Treat Fyne viewer H.264 work as legacy maintenance:

- fix narrow bugs that break builds or tests;
- keep small shared helpers correct;
- avoid large H.264 pipeline rewrites unless the native controller is explicitly
  promoted back to maintained status.

## H.264 Paths

The Wails viewer receives the browser/WebView H.264 video track directly and
renders through standard web media/canvas behavior.

The legacy Fyne viewer receives H.264 RTP, depacketizes it, feeds FFmpeg, and
currently receives MJPEG frames back for display on a Fyne canvas. That double
transcode costs CPU and quality, so it should not be the target for major new
performance work while Wails is the primary controller.
