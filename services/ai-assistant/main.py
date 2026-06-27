import os, json, httpx, urllib.parse
from fastapi import FastAPI, Request

MISTRAL_API_KEY = os.getenv("MISTRAL_API_KEY")
MISTRAL_URL = "https://api.mistral.ai/v1/chat/completions"
MODEL = "mistral-large-latest"
API_BASE = "http://127.0.0.1"

app = FastAPI()

TOOLS = [
    {
        "type": "function",
        "function": {
            "name": "create_project",
            "description": "Create a new Blubase project. Use this when the user asks to create, make, or start a project.",
            "parameters": {
                "type": "object",
                "properties": {
                    "name": {"type": "string", "description": "The project name"}
                },
                "required": ["name"]
            }
        }
    },
    {
        "type": "function",
        "function": {
            "name": "list_projects",
            "description": "List all projects owned by the user",
            "parameters": {"type": "object", "properties": {}}
        }
    },
    {
        "type": "function",
        "function": {
            "name": "run_sql",
            "description": "Execute a SQL query on the user's database",
            "parameters": {
                "type": "object",
                "properties": {
                    "query": {"type": "string", "description": "The SQL query to run"}
                },
                "required": ["query"]
            }
        }
    },
    {
        "type": "function",
        "function": {
            "name": "get_url_config",
            "description": "Get the URL configuration",
            "parameters": {"type": "object", "properties": {}}
        }
    },
    {
        "type": "function",
        "function": {
            "name": "update_url_config",
            "description": "Update the URL configuration",
            "parameters": {
                "type": "object",
                "properties": {
                    "site_url": {"type": "string"},
                    "jwt_expiry_hours": {"type": "integer"},
                    "redirect_urls": {"type": "array", "items": {"type": "string"}}
                }
            }
        }
    },
    {
        "type": "function",
        "function": {
            "name": "help_user",
            "description": "Provide a helpful text response to the user",
            "parameters": {
                "type": "object",
                "properties": {
                    "message": {"type": "string"}
                },
                "required": ["message"]
            }
        }
    }
]

ENDPOINTS = {
    "create_project":      (3002, "POST", "/projects"),
    "list_projects":       (3002, "GET",  "/projects"),
    "run_sql":             (3007, "GET",  "/sql?query={query}"),
    "get_url_config":      (3001, "GET",  "/admin/url-config"),
    "update_url_config":   (3001, "PUT",  "/admin/url-config"),
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
    query = data.get("query", "")

    messages = [
        {
            "role": "system",
            "content": (
                "You are an agentic AI for Blubase. Use the provided tools to help the user. "
                "IMPORTANT: If the user asks to create, make, or start a project, you MUST call the create_project function. "
                "Do NOT use run_sql for project creation. "
                "For SQL operations, use run_sql. "
                "For URL configuration changes, use update_url_config. "
                "Only use help_user for casual conversation."
            )
        },
        {"role": "user", "content": query}
    ]

    payload = {
        "model": MODEL,
        "messages": messages,
        "tools": TOOLS,
        "tool_choice": "auto",
        "temperature": 0.2
    }

    try:
        async with httpx.AsyncClient(timeout=20) as c:
            r = await c.post(MISTRAL_URL, json=payload,
                             headers={"Authorization": f"Bearer {MISTRAL_API_KEY}", "Content-Type": "application/json"})
            r.raise_for_status()
            resp_data = r.json()
            msg = resp_data["choices"][0]["message"]

            if "tool_calls" in msg and msg["tool_calls"]:
                tc = msg["tool_calls"][0]
                fname = tc["function"]["name"]
                fargs = json.loads(tc["function"]["arguments"]) if tc["function"]["arguments"] else {}
                result = await execute_tool(fname, fargs, token)
                return {"answer": json.dumps(result, indent=2), "action": fname}
            return {"answer": msg.get("content", "I'm not sure how to help.")}
    except Exception as e:
        return {"answer": f"Agent error: {str(e)}"}

@app.get("/health")
def health():
    return {"status": "ok"}
