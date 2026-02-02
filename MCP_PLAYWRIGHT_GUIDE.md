# MCP Playwright Setup Guide for Windsurf

**Dato:** 2026-02-02  
**Forfatter:** Cascade AI  
**Formål:** Guide til at få MCP Playwright til at virke i Windsurf

---

## 🎯 Hvad er MCP Playwright?

MCP Playwright er en Model Context Protocol server der giver AI assistenter mulighed for at:
- Automatisk browse websites
- Tage screenshots
- Teste web applikationer
- Interagere med web elementer (klik, type, scroll)
- Verificere UI/UX forbedringer

---

## 📋 Forudsætninger

1. **Windsurf IDE** installeret
2. **Docker** installeret og kørende (til Playwright browser)
3. **Internet forbindelse** (til at hente MCP server)

---

## 🔧 Setup Guide

### 1. Tjek om MCP Playwright er konfigureret

Åbn Windsurf settings og tjek om MCP Playwright er i konfigurationen:

**Sti til settings:**
- Linux: `~/.codeium/windsurf/mcp_config.json` (faktisk sti på dette system)
- Alternativ: `~/.config/Windsurf/User/globalStorage/codeium.codeium/mcp_config.json`
- macOS: `~/Library/Application Support/Windsurf/User/globalStorage/codeium.codeium/mcp_config.json`
- Windows: `%APPDATA%\Windsurf\User\globalStorage\codeium.codeium\mcp_config.json`

**Find din config fil:**
```bash
find /home/$USER -maxdepth 6 -type f -name mcp_config.json 2>/dev/null
```

**Konfiguration skal indeholde:**

```json
{
  "mcpServers": {
    "mcp-playwright": {
      "command": "docker",
      "args": [
        "run",
        "-i",
        "--rm",
        "--init",
        "mcr.microsoft.com/playwright/mcp"
      ]
    }
  }
}
```

**VIGTIGT:** Server navnet er `mcp-playwright` (ikke bare `playwright`)

### 2. Genstart Windsurf

Efter ændringer i MCP konfiguration skal Windsurf genstartes:

1. **Gem alt arbejde**
2. **Luk Windsurf** (Ctrl+Q eller File → Exit)
3. **Åbn Windsurf igen**
4. **Vent på MCP servers starter** (se nederst i vinduet)

### 3. Verificer at MCP Playwright er aktiv

Når Windsurf starter, skal du se i status bar nederst:

```
MCP: mcp-playwright (connected)
```

**Verificer med Cascade AI:**
Bed Cascade om at teste forbindelsen:
```
Kan du liste resources fra mcp-playwright serveren?
```

Hvis Cascade svarer med "server name mcp-playwright not found", betyder det at:
- ❌ MCP serveren er ikke connected i denne session
- ❌ Windsurf skal genstartes
- ❌ Docker kører muligvis ikke

**Tjek MCP status:**
1. Åbn Command Palette (Ctrl+Shift+P)
2. Søg efter "MCP: Show Logs"
3. Vælg "mcp-playwright" server
4. Se om der er fejl i logs

---

## 🐛 Troubleshooting

### Problem 1: "broken pipe" fejl

**Symptom:**
```
transport error: failed to write request: write |1: broken pipe
```

**Løsning:**
1. Genstart Windsurf (Ctrl+Q → Åbn igen)
2. Vent på MCP servers starter (se status bar)
3. Prøv igen efter 10-20 sekunder

### Problem 2: "server name mcp-playwright not found"

**Symptom:**
```
MCP server mcp-playwright not found
```

Dette betyder at MCP serveren ikke er connected i denne Windsurf session, selvom den står i config.

**VIGTIGT:** Hvis Docker kører OG config er korrekt, men Cascade stadig ikke kan se serveren, er det et **Windsurf MCP runtime wiring problem** - ikke et Docker problem.

**Årsager:**
1. Windsurf MCP host har ikke registreret serveren i denne session
2. MCP server fejler ved startup (tjek logs)
3. Windsurf er ikke genstartet efter config ændring
4. Docker kører ikke eller fejler
5. Forkert config fil bruges

**Løsning (Hurtigste Fix):**

**Step 0: Tjek MCP Logs FØRST** ⚠️
Dette er det vigtigste step!

1. Åbn Command Palette (Ctrl+Shift+P)
2. Søg efter: `MCP: Show Logs`
3. Vælg `mcp-playwright` server
4. Se om der er fejl ved startup eller loading

**Typiske fejl i logs:**
- `Error: Cannot connect to Docker daemon`
- `Error: Image not found`
- `Error: Container failed to start`
- `Timeout waiting for server`

**Step 1: Tjek Docker kører**
```bash
docker ps
docker info
# Tjek om Playwright containers kører
docker ps | grep playwright
```

**Step 2: Verificer config er korrekt**
```bash
cat ~/.codeium/windsurf/mcp_config.json
# Tjek at "disabled" er false eller ikke sat
```

**Step 3: Genstart Windsurf HELT** 🔄
Dette er den mest effektive løsning for runtime wiring problemer!

```bash
# Luk ALLE Windsurf vinduer og processer
pkill -9 windsurf

# Vent 5 sekunder
sleep 5

# Åbn Windsurf igen
```

**Step 4: Vent på MCP servers starter**
- Vent 20-30 sekunder efter Windsurf åbner
- Tjek status bar nederst: "MCP: mcp-playwright (connected)"
- Hvis du ikke ser status, åbn Command Palette → "MCP: Show Status"

**Step 5: Test forbindelse**
Bed Cascade: "Kan du liste resources fra mcp-playwright serveren?"

**Hvis det STADIG ikke virker:**

**Step 6: Test Playwright Docker image manuelt**
```bash
docker run -i --rm --init mcr.microsoft.com/playwright/mcp
```
(Tryk Ctrl+C for at stoppe)

Hvis denne kommando fejler, er problemet Docker/image - ikke Windsurf.

**Step 7: Pull image igen**
```bash
docker pull mcr.microsoft.com/playwright/mcp
```

**Step 8: Genstart Windsurf igen**
Efter image pull, genstart Windsurf helt igen.

### Problem 3: MCP Playwright vises ikke i settings

**Løsning:**
1. Åbn `mcp_config.json` manuelt
2. Tilføj Playwright konfiguration (se ovenfor)
3. Gem filen
4. Luk ALLE Windsurf vinduer
5. Åbn Windsurf igen
6. Vent 20-30 sekunder

### Problem 4: Docker ikke installeret

**Symptom:**
```
Error: docker: command not found
```

**Løsning:**
1. Installer Docker Desktop
2. Start Docker
3. Verificer med: `docker --version`
4. Genstart Windsurf

### Problem 5: MCP server starter ikke

**Løsning:**
1. Tjek Docker kører: `docker ps`
2. Pull Playwright image manuelt:
   ```bash
   docker pull mcr.microsoft.com/playwright/mcp
   ```
3. Test Docker kommando manuelt:
   ```bash
   docker run -i --rm --init mcr.microsoft.com/playwright/mcp
   ```
4. Genstart Windsurf

---

## ✅ Test at det virker

Bed Cascade AI om at teste Playwright:

```
Test dashboardet med Playwright på https://stangtennis.github.io/Remote/
```

Cascade skal kunne:
- ✅ Navigere til URL
- ✅ Tage screenshots
- ✅ Klikke på elementer
- ✅ Udfylde formularer
- ✅ Læse console logs

---

## 🎭 Eksempel på Playwright kommandoer

### Naviger til website:
```
Gå til https://example.com med Playwright
```

### Tag screenshot:
```
Tag et screenshot af siden
```

### Klik på element:
```
Klik på login knappen
```

### Udfyld formular:
```
Udfyld email feltet med test@example.com
```

### Test mobile view:
```
Resize browser til mobile størrelse (375x667)
```

---

## 📊 MCP Playwright i andre Windsurf vinduer

Hvis du har flere Windsurf vinduer åbne og MCP Playwright ikke virker i alle:

### Løsning 1: Global konfiguration
MCP konfiguration er global, så alle Windsurf vinduer skal bruge samme settings fil.

### Løsning 2: Genstart alle vinduer
1. Luk **alle** Windsurf vinduer
2. Åbn Windsurf igen
3. Åbn dine projekter igen

### Løsning 3: Tjek workspace settings
Nogle workspaces kan have lokale settings der overskriver globale MCP settings.

---

## 🔍 Debugging

### Se MCP server logs:

1. Åbn Command Palette (Ctrl+Shift+P)
2. Søg efter "MCP: Show Logs"
3. Vælg "playwright" server
4. Se logs for fejl

### Tjek Docker containers:

```bash
# Se kørende containers
docker ps

# Se alle containers (inkl. stoppede)
docker ps -a

# Se Playwright logs
docker logs <container_id>
```

### Tjek Playwright processer:

```bash
# Linux/macOS
ps aux | grep playwright

# Se om MCP server kører
pgrep -af mcp
```

---

## 💡 Tips & Tricks

### Tip 1: Vent efter genstart
Efter Windsurf genstart, vent 10-20 sekunder før du bruger Playwright. MCP serveren skal starte først.

### Tip 2: Brug beskrivende kommandoer
I stedet for: "test siden"  
Brug: "Naviger til https://example.com og tag et screenshot"

### Tip 3: Tjek console logs
Bed Cascade om at tjekke console logs for JavaScript errors:
```
Tjek console logs for fejl
```

### Tip 4: Test mobile først
Start altid med desktop view, derefter resize til mobile for at teste responsiveness.

---

## 🚀 Avanceret brug

### Test flow:
```
1. Naviger til login siden
2. Udfyld email og password
3. Klik på login knap
4. Tag screenshot af dashboard
5. Resize til mobile (375x667)
6. Tag screenshot af mobile view
7. Test keyboard navigation med ? key
```

### Automatisk test suite:
Bed Cascade om at køre en komplet test suite:
```
Kør en fuld test af dashboardet:
1. Login
2. Test alle empty states
3. Test mobile responsiveness
4. Test keyboard shortcuts
5. Verificer ingen console errors
6. Tag screenshots af alt
```

---

## 📚 Ressourcer

- **MCP Playwright GitHub:** https://github.com/microsoft/playwright-mcp
- **Playwright Docs:** https://playwright.dev/
- **Windsurf Docs:** https://docs.codeium.com/windsurf
- **Docker Docs:** https://docs.docker.com/

---

## ✅ Checklist for nye Windsurf vinduer

- [ ] Tjek at Docker kører
- [ ] Verificer MCP config findes i `mcp_config.json`
- [ ] Genstart Windsurf
- [ ] Vent på MCP servers starter (se status bar)
- [ ] Test med simpel kommando: "Naviger til google.com"
- [ ] Verificer screenshot virker
- [ ] Klar til at teste!

---

**Held og lykke med MCP Playwright!** 🎭✨
