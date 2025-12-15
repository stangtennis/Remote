# Implementeringsplan: Auto-opdatering for Controller + Agent (Windows EXE)

## Mål

Gøre både `controller.exe` og `remote-agent.exe` i stand til at:
1) Tjekke om der findes en nyere version (stable/beta).
2) Downloade den rigtige release-asset.
3) Verificere download (mindst SHA256).
4) Installere opdateringen sikkert på Windows (kan ikke overskrive en kørende `.exe` direkte).

## Afgrænsning (v1)

- Opdateringer hentes fra **GitHub Releases** for repoet `stangtennis/Remote`.
- Verifikation via `.sha256` assets i releasen.
- Controller opdaterer sig selv via en lille “updater helper” exe.
- Agent opdaterer enten:
  - **Service mode:** stop service → byt exe → start service, eller
  - **Run Once mode:** byt exe og relaunch.
- Ingen “delta updates”/patching (kun fuld exe).
- Ingen kode-signatur verifikation i v1 (kan tilføjes senere).

## Terminologi

- **App**: controller eller agent (den der opdager ny version).
- **Updater helper**: en separat exe der udfører fil-bytningen, fordi app’en ikke kan overskrive sig selv mens den kører.

---

# Del 1 — Release-format (krævet før kode)

## Task 1.1: Standardisér release assets

For hver release `vX.Y.Z`, upload disse assets:
- `controller-vX.Y.Z.exe`
- `remote-agent-vX.Y.Z.exe`
- `controller-vX.Y.Z.exe.sha256`
- `remote-agent-vX.Y.Z.exe.sha256`

**`.sha256` format** (én linje):
```
<hex_sha256>  <filename>
```

## Task 1.2: Opdatér release script til at generere sha256

**Fil:**
- Modify: `release-github.sh`

**Ændringer:**
- Generér SHA256 for hver exe i build output.
- Upload også `.sha256` assets via `gh release create ...`.

**Verifikation:**
- Kør release-script lokalt på en test-tag.
- Tjek at GitHub releasen indeholder exe + sha256-filer.

---

# Del 2 — Fælles “update feed” klient (i Go)

## Task 2.1: Definér en lille API-model for GitHub Releases

Brug `GET https://api.github.com/repos/stangtennis/Remote/releases/latest` (stable).

Parse kun det nødvendige:
- `tag_name`
- `assets[].name`
- `assets[].browser_download_url`
- `prerelease` (til beta strategi, se Task 2.3)

## Task 2.2: Version parsing/compare

Regler:
- Accepter både `v2.60.0` og `2.60.0` internt.
- Sammenlign SemVer (major/minor/patch).
- Hvis remote version > local version → update available.

## Task 2.3: Kanaler (stable/beta)

**v1 forslag (simpelt):**
- `stable`: `releases/latest` hvor `prerelease=false`.
- `beta`: hent seneste release hvor `prerelease=true` (kræver `releases` liste endpoint og filtrering).

**Endpoint til beta:**
- `GET https://api.github.com/repos/stangtennis/Remote/releases?per_page=10`
- vælg første `prerelease==true`.

---

# Del 3 — Controller (check + download + install)

## Task 3.1: Controller updater library

**Filer:**
- Create: `controller/internal/updater/github.go`
- Create: `controller/internal/updater/version.go`
- Create: `controller/internal/updater/download.go`
- Create: `controller/internal/updater/state.go`

**Ansvar:**
- Fetch release-info (stable/beta).
- Find korrekt asset navn for controller:
  - `controller-vX.Y.Z.exe` + `controller-vX.Y.Z.exe.sha256`
- Download til:
  - `%LOCALAPPDATA%\\RemoteDesktopController\\updates\\<version>\\`
- Verificér SHA256 før “ready to install”.
- Gem state:
  - `last_check`, `ignored_version`, `downloaded_version`, `download_path`.

## Task 3.2: Controller updater helper exe

**Filer:**
- Create: `controller/cmd/controller-updater/main.go`

**CLI flags (forslag):**
- `--target` (sti til nuværende controller exe)
- `--source` (sti til ny exe)
- `--backup` (backup sti, fx `controller.exe.old`)
- `--start` (valgfri: start controller efter install)

**Flow:**
1) Vent på at `target` ikke længere er låst (controller har lukket).
2) Rename `target` → `backup`.
3) Move `source` → `target`.
4) Start `target` igen (hvis `--start`).

**Windows-detaljer:**
- Brug retries (fx 50–100 forsøg med 100ms) for file locks.
- Log til en fil i updates-mappen (for fejlsøgning).

## Task 3.3: UI i controller til “Check for updates”

**Filer:**
- Modify: `controller/main.go` (eller relevant UI-fil hvor Settings dialog bygges)
- Modify: `controller/internal/settings/settings.go` (tilføj update settings)

**UI-elementer:**
- Knap: “Check for updates”
- Checkbox: “Auto-check on startup”
- Dropdown: “Update channel: stable/beta”
- Status: “Up to date / Update available / Downloading / Ready to install”
- Knap: “Download & install” (viser progress)

**Install trigger:**
- Når download er verificeret:
  - spawn `controller-updater.exe` med args
  - luk controller app (graceful)

## Task 3.4: Minimal tests (controller)

**Filer:**
- Create: `controller/internal/updater/version_test.go`
- Create: `controller/internal/updater/github_test.go`

Test:
- SemVer parsing/compare.
- GitHub JSON parsing (med sample JSON i test).
- SHA256 parsing (linjeformat).

**Verifikation (lokalt):**
- `cd controller && go test ./...`

---

# Del 4 — Agent (check + download + service-safe install)

## Task 4.1: Agent updater library

**Filer:**
- Create: `agent/internal/updater/github.go`
- Create: `agent/internal/updater/version.go`
- Create: `agent/internal/updater/download.go`
- Create: `agent/internal/updater/state.go`

Samme ansvar som controller, men asset-navne:
- `remote-agent-vX.Y.Z.exe` + `.sha256`

Download sti (forslag):
- `%PROGRAMDATA%\\RemoteDesktopAgent\\updates\\<version>\\` (service-friendly)
  - fallback til exe-dir hvis ingen adgang.

## Task 4.2: Agent updater helper exe (service mode)

**Filer:**
- Create: `agent/cmd/agent-updater/main.go`

**CLI flags (forslag):**
- `--service-name` (default `RemoteDesktopAgent`)
- `--target` (sti til `remote-agent.exe`)
- `--source` (sti til ny exe)
- `--backup` (backup sti)
- `--restart` (bool)

**Flow (service):**
1) `sc stop RemoteDesktopAgent` (wait til STOPPED).
2) Rename target → backup.
3) Move source → target.
4) `sc start RemoteDesktopAgent` (hvis `--restart`).

**Fallback:**
- Hvis service ikke findes (Run Once mode), skip `sc` og byt fil, og relaunch.

## Task 4.3: Agent UI (Fyne) “Check for updates” → rigtig funktion

**Filer:**
- Modify: `agent/cmd/remote-agent/gui.go` (er pt. stub)

**UI flow:**
- Vis current version.
- “Check now” viser:
  - “Up to date” eller “Update available: vX.Y.Z”
- “Download & install”:
  - Download + verify
  - Launch `agent-updater.exe` (eller samme exe med `--update` mode hvis I vil undgå ekstra binary)
  - Exit GUI (så updater kan bytte filen)

## Task 4.4: Auto-check i agent (valgfrit i v1, men anbefalet)

**Filer:**
- Modify: `agent/cmd/remote-agent/main.go` eller relevant startup

Tilføj:
- env/flag: `AUTO_UPDATE=true/false`
- schedule: check hver 6–12 timer + jitter
- install kun hvis:
  - service mode og ingen aktiv session, eller
  - “safe window” (defineret som: ingen WebRTC connection i gang)

## Task 4.5: Minimal tests (agent)

**Filer:**
- Create: `agent/internal/updater/version_test.go`
- Create: `agent/internal/updater/github_test.go`

**Verifikation:**
- `cd agent && go test ./...`

---

# Del 5 — Dokumentation & drift

## Task 5.1: Dokumentér update system

**Filer:**
- Modify: `README.md` (kort: hvordan updates virker)
- Modify: `agent/README.md` (service-safe update)
- Modify: `controller/README.md` (controller update)
- Modify: `CONFIGURATION.md` (nye env/settings)

## Task 5.2: Rollback plan

Ved update-fejl:
- behold `*.old` backup.
- UI: “Rollback to previous version” (valgfrit v1.1).
- agent: hvis service ikke starter efter update → forsøg auto-rollback (v1.1).

---

# Verifikationsmatrix (Windows)

## Controller
1) Start controller vX.Y.Z
2) Simulér ny release (eller brug en rigtig ny tag)
3) “Check for updates” → ser ny version
4) “Download & install” → controller lukker → updater bytter exe → controller starter i ny version

## Agent (service)
1) Install service (`agent/install-service.bat`)
2) Kør update fra GUI eller auto-check
3) Updater stopper service → bytter exe → starter service
4) Verificér i logs at version er ny

---

# Sikkerhedsnoter (v1 vs v2)

**v1 minimum:**
- SHA256 verificering mod `.sha256` asset.

**v2 anbefaling:**
- Signeret update-manifest (Ed25519) embedded public key i apps.
- Eller Authenticode-signatur verifikation af exe før install.

