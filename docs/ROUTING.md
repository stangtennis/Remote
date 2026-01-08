# 🔀 Unified Routing System

## Én URL til alt

Nu kan du bruge **ét enkelt link** til hele systemet:
```
https://stangtennis.github.io/Remote/
```

## Hvordan det virker

### 1️⃣ **Root URL (`index.html`)**
Automatisk routing baseret på login status og brugerrolle:

- **Ikke logget ind** → `login.html`
- **Logget ind som admin** → `admin.html`
- **Logget ind som bruger** → `dashboard.html`
- **Ikke godkendt** → `login.html?status=pending`

### 2️⃣ **Special URLs**

**Web Agent:**
```
https://stangtennis.github.io/Remote/?mode=agent
```
Redirecter til agent.html (kræver login)

**Invitation:**
```
https://stangtennis.github.io/Remote/?invite=TOKEN
```
Redirecter til login med invitation token

### 3️⃣ **Login Page (`login.html`)**

**Status beskeder:**
- `?status=pending` - Viser "Afventer godkendelse"
- `?status=logout` - Viser "Du er nu logget ud"

**Redirect efter login:**
- `?redirect=agent` - Går til agent.html efter login
- Ellers går til index.html for rolle-baseret routing

## Filer

```
docs/
├── index.html          # Unified entry point (rolle-baseret routing)
├── login.html          # Login/signup side (tidligere index.html)
├── dashboard.html      # Bruger dashboard
├── admin.html          # Admin panel
└── agent.html          # Web agent
```

## Eksempler

### Almindelig bruger
1. Går til `https://stangtennis.github.io/Remote/`
2. Ikke logget ind → redirecter til `login.html`
3. Logger ind
4. Redirecter til `index.html`
5. Rolle = "user" → redirecter til `dashboard.html`

### Administrator
1. Går til `https://stangtennis.github.io/Remote/`
2. Ikke logget ind → redirecter til `login.html`
3. Logger ind
4. Redirecter til `index.html`
5. Rolle = "admin" → redirecter til `admin.html`

### Web Agent
1. Går til `https://stangtennis.github.io/Remote/?mode=agent`
2. Ikke logget ind → redirecter til `login.html?redirect=agent`
3. Logger ind
4. Redirecter direkte til `agent.html`

### Pending bruger
1. Går til `https://stangtennis.github.io/Remote/`
2. Logger ind
3. Ikke godkendt → logger ud og redirecter til `login.html?status=pending`
4. Ser besked: "⏳ Din konto afventer godkendelse"

## Fordele

✅ **Ét link** - Kun én URL at huske og dele  
✅ **Smart routing** - Automatisk til den rigtige side  
✅ **Rolle-baseret** - Admin/bruger får forskellige sider  
✅ **Status beskeder** - Pending, logout, osv.  
✅ **Deep linking** - Direkte til agent med `?mode=agent`  
✅ **Invitation support** - `?invite=token` virker stadig  

## Migration

Gamle links virker stadig:
- `dashboard.html` → Virker (kræver login)
- `admin.html` → Virker (kræver admin rolle)
- `agent.html` → Virker (kræver login)

Men brug nu bare:
```
https://stangtennis.github.io/Remote/
```
