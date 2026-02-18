---
name: build
description: Byg alle 3 exe-filer (version injiceres via ldflags), deploy til Caddy, opdater version.json, commit+push+tag
argument-hint: <version> f.eks. v2.70.0
---

# Build & Deploy Remote Desktop

Du skal udføre en fuld build+deploy cycle for version `$ARGUMENTS`.

## Forudsætninger
- Argumentet SKAL være et versionsnummer i formatet `vX.XX.X` (f.eks. v2.70.0)
- Hvis intet argument er givet, spørg brugeren om versionsnummer

## Trin (udfør i rækkefølge)

### 1. Byg alle 3 exe-filer
Version injiceres automatisk via `-ldflags -X` i build-scriptet — ingen source-code ændringer nødvendige.
Kør build-scriptet fra projekt-root:
```bash
./build-local.sh $ARGUMENTS
```
Timeout: 10 minutter. Vis resultatet og bekræft alle 3 filer er bygget.

### 2. Deploy til Caddy
Kopiér builds til Caddy downloads-mappen:
```bash
# Generiske navne (bruges af auto-update)
cp builds/remote-agent-$ARGUMENTS.exe ~/caddy/downloads/remote-agent.exe
cp builds/remote-agent-console-$ARGUMENTS.exe ~/caddy/downloads/remote-agent-console.exe
cp builds/controller-$ARGUMENTS.exe ~/caddy/downloads/controller.exe

# Behold også versionerede kopier
cp builds/remote-agent-$ARGUMENTS.exe ~/caddy/downloads/
cp builds/remote-agent-console-$ARGUMENTS.exe ~/caddy/downloads/
cp builds/controller-$ARGUMENTS.exe ~/caddy/downloads/

# Deploy installere (generiske navne til download-links)
cp builds/RemoteDesktopAgent-$ARGUMENTS-Setup.exe ~/caddy/downloads/RemoteDesktopAgent-Setup.exe
cp builds/RemoteDesktopAgentConsole-$ARGUMENTS-Setup.exe ~/caddy/downloads/RemoteDesktopAgentConsole-Setup.exe
cp builds/RemoteDesktopController-$ARGUMENTS-Setup.exe ~/caddy/downloads/RemoteDesktopController-Setup.exe
```

### 3. Opdater version.json (trigger auto-update)
Skriv ny version.json:
```bash
cat > ~/caddy/downloads/version.json << EOF
{
  "agent_version": "$ARGUMENTS",
  "controller_version": "$ARGUMENTS",
  "agent_url": "https://updates.hawkeye123.dk/remote-agent.exe",
  "controller_url": "https://updates.hawkeye123.dk/controller.exe"
}
EOF
```

### 4. Git commit + push + tag
- Stage ændrede filer (version bumps + eventuelle andre ændringer)
- Commit med besked på dansk der beskriver hvad der er nyt i denne version
- Opret git tag med versionsnummeret
- Push commit og tag sammen: `git push origin main --tags`

### 5. Opdater MEMORY.md
Opdater "Current version" i `/home/dennis/.claude/projects/-home-dennis-projekter-Remote-Desktop/memory/MEMORY.md` til den nye version.

## Vigtige noter
- Sprog: Dansk i commit messages og UI-tekst
- Byg ALTID alle 3 exe-filer (controller, agent GUI, agent console)
- Sørg for at version.json matcher den byggede version
- Vent med push til brugeren har godkendt commit-beskeden
