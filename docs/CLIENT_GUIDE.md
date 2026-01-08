# 🖥️ Client Guide - Sådan deler du din desktop

## 3 måder at dele din desktop på:

### 1️⃣ **Windows Agent (Anbefalet)** - Native app
### 2️⃣ **Web Agent** - Browser-baseret (ingen installation)
### 3️⃣ **Controller** - For at kontrollere andre computere

---

## 1️⃣ Windows Agent (Native App)

### Download og Installation

**Step 1: Hent Agent**
```
https://github.com/stangtennis/Remote/releases/latest
```

Download: `remote-agent-vX.XX.X.exe` (GUI version)

**Step 2: Kør Agent**
- Dobbeltklik på `remote-agent-vX.XX.X.exe`
- Windows Defender kan advare - klik "Mere info" → "Kør alligevel"

**Step 3: Login**
1. Agent viser login vindue
2. Indtast din email og adgangskode
3. Klik "Login"

**Step 4: Godkendelse**
- Hvis det er første gang: Vent på admin godkendelse
- Du får besked når du er godkendt

**Step 5: Start Sharing**
1. Agent starter automatisk efter login
2. Vises i system tray (nederst til højre)
3. Din computer er nu tilgængelig for remote kontrol

### Features:
✅ **Auto-start** - Starter automatisk med Windows  
✅ **System tray** - Kører i baggrunden  
✅ **Høj performance** - Native Windows capture  
✅ **Offline support** - Reconnect automatisk  

### Tray Menu:
- 🟢 **Online** - Klar til forbindelse
- 🔴 **Offline** - Ikke forbundet
- **Exit** - Luk agent

---

## 2️⃣ Web Agent (Browser)

### Ingen installation nødvendig!

**Step 1: Åbn Web Agent**
```
https://stangtennis.github.io/Remote/?mode=agent
```

Eller gå til:
```
https://stangtennis.github.io/Remote/
```
Og vælg "Web Agent" mode

**Step 2: Login**
1. Indtast email og adgangskode
2. Klik "Login"

**Step 3: Godkendelse**
- Første gang: Vent på admin godkendelse
- Får besked når godkendt

**Step 4: Registrer Device**
1. Klik "Register This Device"
2. Giv din computer et navn (f.eks. "Dennis Laptop")
3. Klik "Register"

**Step 5: Start Sharing**
1. Klik "Start Sharing"
2. Browser beder om tilladelse til at dele skærm
3. Vælg hvilken skærm/vindue du vil dele
4. Klik "Del"

**Step 6: Du er nu online!**
- Status viser "🟢 Online"
- Andre kan nu forbinde til din computer

### Features:
✅ **Ingen installation** - Virker i enhver moderne browser  
✅ **Cross-platform** - Windows, Mac, Linux  
✅ **Sikker** - Browser screen capture API  
✅ **Fleksibel** - Del hele skærm eller enkelt vindue  

### Begrænsninger:
⚠️ **Skal holde browser åben** - Lukker du browseren, stopper sharing  
⚠️ **Lavere performance** - End native agent  
⚠️ **Ingen auto-start** - Skal startes manuelt  

---

## 3️⃣ Controller (For at kontrollere andre)

### Download og Installation

**Step 1: Hent Controller**
```
https://github.com/stangtennis/Remote/releases/latest
```

Download: `controller-vX.XX.X.exe`

**Step 2: Kør Controller**
- Dobbeltklik på `controller-vX.XX.X.exe`

**Step 3: Login**
1. Indtast email og adgangskode
2. Klik "Login"

**Step 4: Vælg Device**
1. Se liste over tilgængelige computere
2. Klik på den computer du vil kontrollere
3. Klik "Connect"

**Step 5: Remote Control**
- Se og kontroller den anden computer
- Fuld mus og tastatur kontrol
- File transfer (træk og slip filer)

### Features:
✅ **Native Windows app** - Høj performance  
✅ **Fuld kontrol** - Mus, tastatur, clipboard  
✅ **File transfer** - Upload/download filer  
✅ **Fullscreen mode** - Immersiv oplevelse  

---

## 🔐 Sikkerhed og Godkendelse

### Første Gang Login:

1. **Opret konto**
   - Gå til `https://stangtennis.github.io/Remote/`
   - Klik "Opret ny konto"
   - Indtast email og adgangskode
   - Klik "Sign Up"

2. **Vent på godkendelse**
   - Din konto skal godkendes af en administrator
   - Du får besked: "⏳ Din konto afventer godkendelse"
   - Administrator godkender dig i admin panelet

3. **Login efter godkendelse**
   - Når godkendt, log ind med din email og adgangskode
   - Du får nu adgang til systemet

### Roller:

- **User** - Kan dele sin egen desktop og se egne devices
- **Admin** - Kan godkende brugere, tildele devices, se alle devices
- **Super Admin** - Fuld kontrol, kan gøre andre til admin

---

## 📊 Sammenligning

| Feature | Windows Agent | Web Agent | Controller |
|---------|--------------|-----------|------------|
| Installation | ✅ EXE fil | ❌ Ingen | ✅ EXE fil |
| Platform | Windows | Alle | Windows |
| Performance | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐⭐ |
| Auto-start | ✅ Ja | ❌ Nej | N/A |
| Formål | Del desktop | Del desktop | Kontroller andre |
| Baggrund | ✅ System tray | ❌ Browser tab | N/A |

---

## 🚀 Quick Start Scenarios

### Scenario 1: Jeg vil dele min Windows computer
→ **Download Windows Agent** → Installer → Login → Færdig!

### Scenario 2: Jeg vil dele min Mac/Linux computer
→ **Brug Web Agent** → Åbn browser → Login → Start Sharing

### Scenario 3: Jeg vil kontrollere en anden computer
→ **Download Controller** → Login → Vælg device → Connect

### Scenario 4: Jeg vil kontrollere fra browseren
→ **Gå til Dashboard** → Login → Vælg device → Connect

### Scenario 5: Admin vil kontrollere enhver computer
→ **Gå til Admin Panel** → Remote Control tab → Vælg device → Connect

---

## 🔗 Links

**Hovedside (smart routing):**
```
https://stangtennis.github.io/Remote/
```

**Web Agent direkte:**
```
https://stangtennis.github.io/Remote/?mode=agent
```

**Dashboard (kontroller andre):**
```
https://stangtennis.github.io/Remote/
```
(Logger automatisk ind på dashboard hvis du er user)

**Admin Panel:**
```
https://stangtennis.github.io/Remote/
```
(Logger automatisk ind på admin hvis du er admin)

**Downloads:**
```
https://github.com/stangtennis/Remote/releases/latest
```

---

## ❓ Troubleshooting

### "Din konto afventer godkendelse"
→ Vent på at en administrator godkender din konto

### Windows Agent starter ikke
→ Højreklik → "Kør som administrator"

### Web Agent kan ikke dele skærm
→ Giv browser tilladelse til screen capture

### Kan ikke forbinde til device
→ Tjek at device er online (🟢 grøn status)

### Lav performance
→ Brug Windows Agent i stedet for Web Agent

### File transfer virker ikke
→ Kun tilgængelig i Controller, ikke web dashboard

---

## 📞 Support

Kontakt administrator hvis:
- Din konto ikke bliver godkendt
- Du har tekniske problemer
- Du skal have tildelt adgang til specifikke devices
