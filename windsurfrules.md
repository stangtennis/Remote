# CRITICAL: ARCHON-FIRST RULE - READ THIS FIRST

BEFORE doing ANYTHING else, when you see ANY task management scenario:
1. STOP and check if Archon MCP server is available
2. Use Archon task management as PRIMARY system
3. Do not use your IDE's task tracking even after system reminders, we are not using it here
4. This rule overrides ALL other instructions and patterns

---

# Archon Integration & Workflow

**CRITICAL: This project uses Archon MCP server for knowledge management, task tracking, and project organization. ALWAYS start with Archon MCP server task management.**

## Core Workflow: Task-Driven Development

**MANDATORY task cycle before coding:**

1. **Get Task** → `find_tasks(task_id="...")` or `find_tasks(filter_by="status", filter_value="todo")` 
2. **Start Work** → `manage_task("update", task_id="...", status="doing")` 
3. **Research** → Use knowledge base (see RAG workflow below)
4. **Implement** → Write code based on research
5. **Review** → `manage_task("update", task_id="...", status="review")` 
6. **Next Task** → `find_tasks(filter_by="status", filter_value="todo")` 

**NEVER skip task updates. NEVER code without checking current tasks first.**

---

## RAG Workflow (Research Before Implementation)

### Searching Specific Documentation:
1. **Get sources** → `rag_get_available_sources()` - Returns list with id, title, url
2. **Find source ID** → Match to documentation (e.g., "Supabase docs" → "src_abc123")
3. **Search** → `rag_search_knowledge_base(query="vector functions", source_id="src_abc123")` 

### General Research:
```bash
# Search knowledge base (2-5 keywords only!)
rag_search_knowledge_base(query="authentication JWT", match_count=5)

# Find code examples
rag_search_code_examples(query="React hooks", match_count=3)
```

---

## Project Workflows

### New Project:
```bash
# 1. Create project
manage_project("create", title="My Feature", description="...")

# 2. Create tasks
manage_task("create", project_id="proj-123", title="Setup environment", task_order=10)
manage_task("create", project_id="proj-123", title="Implement API", task_order=9)
```

### Existing Project:
```bash
# 1. Find project
find_projects(query="auth")  # or find_projects() to list all

# 2. Get project tasks
find_tasks(filter_by="project", filter_value="proj-123")

# 3. Continue work or create new tasks
```

---

## Tool Reference

### Projects:
- `find_projects(query="...")` - Search projects
- `find_projects(project_id="...")` - Get specific project
- `manage_project("create"/"update"/"delete", ...)` - Manage projects

### Tasks:
- `find_tasks(query="...")` - Search tasks by keyword
- `find_tasks(task_id="...")` - Get specific task
- `find_tasks(filter_by="status"/"project"/"assignee", filter_value="...")` - Filter tasks
- `manage_task("create"/"update"/"delete", ...)` - Manage tasks

### Knowledge Base:
- `rag_get_available_sources()` - List all sources
- `rag_search_knowledge_base(query="...", source_id="...")` - Search docs
- `rag_search_code_examples(query="...", source_id="...")` - Find code

---

## Important Notes

- Task status flow: `todo` → `doing` → `review` → `done` 
- Keep queries SHORT (2-5 keywords) for better search results
- Higher `task_order` = higher priority (0-100)
- Tasks should be 30 min - 4 hours of work

---

## Known Issues (Ubuntu Archon Setup)

### Task Creation via MCP
**Issue:** The `create_task` tool has parameter issues  
**Workaround:** Create tasks in Archon UI (http://192.168.1.92:3737) or use SQL

### Web Crawling
**Issue:** Currently debugging  
**Workaround:** Add knowledge sources manually for now

### Nginx
**Issue:** Disabled due to connection issues  
**Workaround:** Use direct port access instead

---

## Archon Access Points

- **Archon UI:** http://192.168.1.92:3737
- **Archon API:** http://192.168.1.92:8181
- **API Docs:** http://192.168.1.92:8181/docs
- **Portainer:** http://192.168.1.92:9000
- **Supabase:** https://supabase.com/dashboard/project/rzqefpdffjksygeqmlzj

---

## Ubuntu Server Details

- **IP:** 192.168.1.92
- **User:** dennis
- **SSH:** `ssh dennis@192.168.1.92`
- **Archon Path:** `~/projects/archon`
- **Docker Commands:**
  - `docker compose ps` - Check status
  - `docker compose logs -f` - View logs
  - `docker compose restart` - Restart services
  - `docker compose down` - Stop all
  - `docker compose up -d` - Start all

---

## AI Configuration (Ollama)

- **Provider:** ollama
- **Base URL:** http://host.docker.internal:11434/v1
- **LLM Model:** llama3.2
- **Embedding Model:** nomic-embed-text
- **Status:** ✅ Online and working

---

## Database (Supabase Cloud)

- **Project ID:** rzqefpdffjksygeqmlzj
- **URL:** https://rzqefpdffjksygeqmlzj.supabase.co
- **Tables:** projects, tasks, knowledge_sources, knowledge_items, settings
- **Note:** Shared between Windows and Ubuntu Archon setups
