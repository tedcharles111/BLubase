import os, json, httpx, urllib.parse, asyncpg
from fastapi import FastAPI, Request

MISTRAL_API_KEY = os.getenv("MISTRAL_API_KEY")
MISTRAL_URL = "https://api.mistral.ai/v1/chat/completions"
MODEL = "mistral-large-latest"
DATABASE_URL = os.getenv("DATABASE_URL")

app = FastAPI()

# ---------- Memory Database (used by /history) ----------
async def get_memory(user_id: str, limit: int = 50):
    conn = await asyncpg.connect(DATABASE_URL)
    rows = await conn.fetch(
        "SELECT role, content, created_at FROM ai_memory WHERE user_id=$1 ORDER BY created_at ASC LIMIT $2",
        user_id, limit
    )
    await conn.close()
    return [dict(row) for row in rows]

async def add_memory(user_id: str, role: str, content: str):
    conn = await asyncpg.connect(DATABASE_URL)
    await conn.execute(
        "INSERT INTO ai_memory (user_id, role, content) VALUES ($1, $2, $3)",
        user_id, role, content
    )
    await conn.close()

# Ensure the memory table exists
@app.on_event("startup")
async def startup():
    conn = await asyncpg.connect(DATABASE_URL)
    await conn.execute('''CREATE TABLE IF NOT EXISTS ai_memory (
        id SERIAL PRIMARY KEY,
        user_id TEXT NOT NULL,
        role TEXT NOT NULL,
        content TEXT NOT NULL,
        created_at TIMESTAMPTZ DEFAULT now()
    )''')
    await conn.close()

# ---------- Tools ----------
TOOLS = [
    {"type":"function","function":{"name":"list_projects","description":"List all projects owned by the current user","parameters":{"type":"object","properties":{}}}},
    {"type":"function","function":{"name":"create_project","description":"Create a new project","parameters":{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}}},
    {"type":"function","function":{"name":"run_sql","description":"Execute a SQL query","parameters":{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}}},
    {"type":"function","function":{"name":"get_url_config","description":"Get URL configuration","parameters":{"type":"object","properties":{}}}},
    {"type":"function","function":{"name":"update_url_config","description":"Update URL configuration","parameters":{"type":"object","properties":{"site_url":{"type":"string"},"jwt_expiry_hours":{"type":"integer"},"redirect_urls":{"type":"array","items":{"type":"string"}}}}}},
    {"type":"function","function":{"name":"list_oauth_providers","description":"List all OAuth providers","parameters":{"type":"object","properties":{}}}},
    {"type":"function","function":{"name":"add_oauth_provider","description":"Add or update an OAuth provider","parameters":{"type":"object","properties":{"provider":{"type":"string"},"client_id":{"type":"string"},"client_secret":{"type":"string"},"enabled":{"type":"boolean"}},"required":["provider","client_id","client_secret"]}}},
    {"type":"function","function":{"name":"list_email_templates","description":"List email templates","parameters":{"type":"object","properties":{}}}},
    {"type":"function","function":{"name":"create_email_template","description":"Create or update an email template","parameters":{"type":"object","properties":{"name":{"type":"string"},"subject":{"type":"string"},"body":{"type":"string"}},"required":["name","subject","body"]}}},
    {"type":"function","function":{"name":"get_smtp_config","description":"Get SMTP configuration","parameters":{"type":"object","properties":{}}}},
    {"type":"function","function":{"name":"update_smtp_config","description":"Update SMTP configuration","parameters":{"type":"object","additionalProperties":{"type":"string"}}}},
    {"type":"function","function":{"name":"forgot_password","description":"Request a password reset OTP","parameters":{"type":"object","properties":{"email":{"type":"string"}},"required":["email"]}}},
    {"type":"function","function":{"name":"reset_password","description":"Reset password using OTP","parameters":{"type":"object","properties":{"email":{"type":"string"},"otp":{"type":"string"},"new_password":{"type":"string"}},"required":["email","otp","new_password"]}}},
    {"type":"function","function":{"name":"help_user","description":"Provide a helpful text response","parameters":{"type":"object","properties":{"message":{"type":"string"}},"required":["message"]}}}
]

ENDPOINTS = {
    "list_projects":       (3002, "GET",  "/projects"),
    "create_project":      (3002, "POST", "/projects"),
    "run_sql":             (3007, "GET",  "/sql?query={query}"),
    "get_url_config":      (3001, "GET",  "/admin/url-config"),
    "update_url_config":   (3001, "PUT",  "/admin/url-config"),
    "list_oauth_providers":(3001, "GET",  "/admin/oauth-providers"),
    "add_oauth_provider":  (3001, "POST", "/admin/oauth-providers"),
    "list_email_templates":(3001, "GET",  "/admin/templates"),
    "create_email_template":(3001,"POST", "/admin/templates"),
    "get_smtp_config":     (3001, "GET",  "/admin/smtp"),
    "update_smtp_config":  (3001, "PUT",  "/admin/smtp"),
    "forgot_password":     (3001, "POST", "/forgot-password"),
    "reset_password":      (3001, "POST", "/reset-password"),
}

async def execute_tool(name, args, token):
    if name == "help_user":
        return {"answer": args["message"]}
    if name not in ENDPOINTS:
        return {"error": f"unknown tool {name}"}
    port, method, path = ENDPOINTS[name]
    url = f"http://127.0.0.1:{port}{path}"
    if "{query}" in url:
        url = url.replace("{query}", urllib.parse.quote(args.get("query", "")))
    headers = {"Authorization": f"Bearer {token}"} if token else {}
    async with httpx.AsyncClient(timeout=10) as c:
        if method == "GET":
            r = await c.get(url, headers=headers)
        elif method == "POST":
            r = await c.post(url, json=args, headers=headers)
        elif method == "PUT":
            r = await c.put(url, json=args, headers=headers)
        return r.json() if r.status_code == 200 else {"error": r.text}

@app.post("/assist")
async def assist(request: Request):
    token = request.headers.get("Authorization", "").removeprefix("Bearer ")
    data = await request.json()
    user_query = data.get("query", "")
    user_id = token.split(".")[0] if token else "anonymous"

    # Store user message
    await add_memory(user_id, "user", user_query)

    # Load recent memory
    memory = await get_memory(user_id, 10)
    messages = [{"role": "system", "content": "You are an agentic AI for Blubase. Use the provided tools to help the user. You have access to the conversation history below."}]
    for mem in memory:
        messages.append({"role": mem["role"], "content": mem["content"]})

    payload = {
        "model": MODEL,
        "messages": messages,
        "tools": TOOLS,
        "tool_choice": "auto",
        "temperature": 0.2
    }

    async with httpx.AsyncClient(timeout=20) as c:
        r = await c.post(MISTRAL_URL, json=payload,
                         headers={"Authorization": f"Bearer {MISTRAL_API_KEY}", "Content-Type": "application/json"})
        r.raise_for_status()
        resp_data = r.json()
        msg = resp_data["choices"][0]["message"]

        answer_text = msg.get("content", "")
        action = "chat"

        if "tool_calls" in msg and msg["tool_calls"]:
            tc = msg["tool_calls"][0]
            fname = tc["function"]["name"]
            fargs = json.loads(tc["function"]["arguments"]) if tc["function"]["arguments"] else {}
            result = await execute_tool(fname, fargs, token)
            answer_text = json.dumps(result, indent=2)
            action = fname

        await add_memory(user_id, "assistant", answer_text)
        return {"answer": answer_text, "action": action}

@app.get("/history")
async def history(request: Request):
    token = request.headers.get("Authorization", "").removeprefix("Bearer ")
    user_id = token.split(".")[0] if token else "anonymous"
    rows = await get_memory(user_id, 50)
    return rows

@app.get("/health")
def health():
    return {"status": "ok"}
