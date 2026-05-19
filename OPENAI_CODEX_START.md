# OpenAI Codex CLI

Opdateret til den nuvaerende Codex CLI flow pr. 2026-05-19.

## Hurtig start uden installation
```bash
npx -y @openai/codex@latest
```

## Installer Codex globalt
```bash
npm install -g @openai/codex@latest
codex --version
```

## Login
```bash
codex login
```

Hvis du bruger API-noegle i stedet for interaktiv login:
```bash
export OPENAI_API_KEY="sk-..."
```

## Start i dette repo
```bash
codex -C "/home/dennis/projekter/Remote Desktop"
```

## Koer uden approvals/sandbox
Brug kun dette i et miljoe du selv stoler paa:
```bash
codex --dangerously-bypass-approvals-and-sandbox -C "/home/dennis/projekter/Remote Desktop"
```

## Opdater Codex
Hvis Codex allerede er installeret:
```bash
codex update
```

Hvis du starter via `npx`, faar du automatisk seneste version:
```bash
npx -y @openai/codex@latest
```

## Fejlsoegning
```bash
codex doctor --summary
```

## Aktuel versionsstatus
- Lokal installation paa denne maskine: `codex-cli 0.131.0`
- Seneste npm stable pr. 2026-05-19: `@openai/codex 0.131.0`

## Med Archon MCP (valgfrit)
Hvis du vil bruge Archon MCP server, se `~/ARCHON_CODEX_WORKING_SOLUTION.md`
