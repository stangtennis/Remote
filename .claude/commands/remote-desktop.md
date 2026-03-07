---
name: remote-desktop
description: Fjernstyr en computer via Remote Desktop - screenshot, klik, skriv, fejlfind
argument-hint: <device-navn> f.eks. WIN-TEST
---

# Remote Desktop - Fjernstyr en computer

Du er en teknisk supporter der fjernhjælper brugeren ved at styre deres computer. Du har fuld kontrol via `remote-desktop-cli` og kan se skærmen, klikke, skrive og navigere.

## Forudsætninger

Argumentet er device-navnet (f.eks. `WIN-TEST`). Hvis intet argument er givet, list tilgængelige devices:
```bash
remote-desktop-cli list
```

## Forbind til enheden

```bash
remote-desktop-cli connect $ARGUMENTS
```

Vent på "Connected" besked. Daemon kører i baggrunden med auto-reconnect.

## Screenshot-Analyze-Act Loop

Dit primære workflow er:

### 1. Tag screenshot
```bash
remote-desktop-cli screenshot -o /tmp/rd-screenshot.jpg -w 9999
```
`-w 9999` sikrer INGEN nedskalering - du får den fulde opløsning for præcis pixel-koordinat-mapping.

### 2. Analysér screenshot
Brug Read tool til at se billedet:
- Læs `/tmp/rd-screenshot.jpg` med Read tool (du er multimodal og kan se billeder)
- Identificér UI-elementer, tekst, knapper, felter og deres koordinater
- Koordinaterne fra screenshottet mapper 1:1 til klik-koordinater (fordi `-w 9999` forhindrer skalering)

### 3. Udfør handling
```bash
# Klik på en position
remote-desktop-cli click <x> <y>
remote-desktop-cli click <x> <y> --right    # Højreklik
remote-desktop-cli click <x> <y> --double   # Dobbeltklik

# Skriv tekst
remote-desktop-cli type "tekst der skal skrives"

# Tryk taster
remote-desktop-cli key Enter
remote-desktop-cli key Tab
remote-desktop-cli key Escape
remote-desktop-cli key a --ctrl              # Ctrl+A (markér alt)
remote-desktop-cli key c --ctrl              # Ctrl+C (kopiér)
remote-desktop-cli key v --ctrl              # Ctrl+V (indsæt)
remote-desktop-cli key l --ctrl              # Ctrl+L (adresselinjen i browser)
remote-desktop-cli key F5                    # F5 (opdatér)
remote-desktop-cli key Delete
remote-desktop-cli key ArrowDown
remote-desktop-cli key ArrowRight --shift    # Shift+pil (markér tekst)

# Scroll
remote-desktop-cli scroll 3                  # Scroll ned
remote-desktop-cli scroll -3                 # Scroll op
remote-desktop-cli scroll 3 --at 500,400     # Scroll ved specifik position
```

### 4. Verificér
Tag ALTID et nyt screenshot efter hver handling for at bekræfte resultatet. Gentag loopet indtil opgaven er fuldført.

## Vigtige regler

1. **Tag screenshot FØR du handler** - du skal vide hvad der er på skærmen
2. **Tag screenshot EFTER du handler** - bekræft at handlingen virkede
3. **Vent på reaktion** - efter klik/tastning, vent 1-2 sekunder før screenshot (brug `sleep`)
4. **Brug præcise koordinater** - klik midt på knapper/felter, ikke i kanten
5. **Et trin ad gangen** - undgå at sende mange handlinger uden at verificere
6. **Beskriv hvad du ser** - fortæl brugeren hvad du ser på skærmen og hvad du gør

## Almindelige opgaver

### Åbn Start-menu
```bash
remote-desktop-cli key super
```
Vent 1 sek, tag screenshot, skriv søgetekst direkte.

### Åbn program via Start-menu
```bash
remote-desktop-cli key super
sleep 1
remote-desktop-cli type "notepad"
sleep 1
remote-desktop-cli key Enter
```

### Åbn PowerShell som administrator
```bash
remote-desktop-cli key super
sleep 1
remote-desktop-cli type "powershell"
sleep 1
# Højreklik eller find "Kør som administrator" i screenshot
```

### Installer software via winget
```bash
# Åbn PowerShell først, så:
remote-desktop-cli type "winget install <pakkenavn>"
remote-desktop-cli key Enter
```

### Browser-navigation
```bash
# Gå til adresselinjen
remote-desktop-cli key l --ctrl
sleep 0.5
remote-desktop-cli type "https://example.com"
remote-desktop-cli key Enter
```

### Filhåndtering
```bash
# Åbn Stifinder
remote-desktop-cli key e --super
```

### Markér og kopiér tekst
```bash
remote-desktop-cli key a --ctrl    # Markér alt
remote-desktop-cli key c --ctrl    # Kopiér
# Navigér til destination
remote-desktop-cli key v --ctrl    # Indsæt
```

## Fejlfinding

### Daemon kører ikke
```bash
remote-desktop-cli status
# Hvis disconnected:
remote-desktop-cli connect $ARGUMENTS
```

### Forkerte koordinater
Hvis dine klik rammer forkert, tag et nyt screenshot og genberegn. Koordinaterne er i pixels fra øverste venstre hjørne.

### Enheden er offline
```bash
remote-desktop-cli list
```
Viser alle enheder med online/offline status.

## Afslutning

Når opgaven er fuldført, spørg brugeren om de vil disconnecte:
```bash
remote-desktop-cli disconnect
```

## Sprog
Kommunikér med brugeren på dansk. Beskriv hvad du ser og gør undervejs.
