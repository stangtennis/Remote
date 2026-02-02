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
- Linux: `~/.config/Windsurf/User/globalStorage/codeium.codeium/mcp_config.json`
- macOS: `~/Library/Application Support/Windsurf/User/globalStorage/codeium.codeium/mcp_config.json`
- Windows: `%APPDATA%\Windsurf\User\globalStorage\codeium.codeium\mcp_config.json`

**Konfiguration skal indeholde:**

```json
{
  "mcpServers": {
    "playwright": {
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

### 2. Genstart Windsurf

Efter ændringer i MCP konfiguration skal Windsurf genstartes:

1. **Gem alt arbejde**
2. **Luk Windsurf** (Ctrl+Q eller File → Exit)
3. **Åbn Windsurf igen**
4. **Vent på MCP servers starter** (se nederst i vinduet)

### 3. Verificer at MCP Playwright er aktiv

Når Windsurf starter, skal du se i status bar nederst:

```
MCP: playwright (connected)
```

Eller åbn Command Palette (Ctrl+Shift+P) og søg efter "MCP" for at se status.

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

### Problem 2: MCP Playwright vises ikke i settings

**Løsning:**
1. Åbn `mcp_config.json` manuelt
2. Tilføj Playwright konfiguration (se ovenfor)
3. Gem filen
4. Genstart Windsurf

### Problem 3: Docker ikke installeret

**Symptom:**
```
Error: docker: command not found
```

**Løsning:**
1. Installer Docker Desktop
2. Start Docker
3. Verificer med: `docker --version`
4. Genstart Windsurf

### Problem 4: MCP server starter ikke

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
