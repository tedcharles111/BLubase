import os, json, httpx
from fastapi import FastAPI, Request

MISTRAL_API_KEY = os.getenv("MISTRAL_API_KEY")
MISTRAL_URL = "https://api.mistral.ai/v1/chat/completions"
MODEL = "mistral-large-latest"

app = FastAPI()

TOOLS = [
    {
        "type": "function",
        "function": {
            "name": "list_projects",
            "description": "List all projects owned by the current user",
            "parameters": {"type": "object", "properties": {}}
        }
    },
    {
        "type": "function",
        "function": {
            "name": "create_project",
            "description": "Create a new project for the current user",
            "parameters": {
                "type": "object",
                "properties": {
                    "name": {"type": "string", "description": "Project name"}
                },
                "required": ["name"]
            }
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
                    "query": {"type": "string", "description": "SQL query to execute"}
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

# Internal endpoints – all services are on localhost with different ports
ENDPOINTS = {
    "list_projects":      "http://127.0.0.1:3002/projects",
    "create_project":     "http://127.0.0.1:3002/projects",
    "run_sql":            "http://127.0.0.1:3007/sql?query={query}",
    "get_url_config":     "http://127.0.0.1:3001/admin/url-config",
    "update_url_config":  "http://127.0.0.1:3001/admin/url-config",
}

async def execute_tool(name, args, token):
    headers = {"Authorization": f"Bearer {token}"} if token else {}
    try:
        if name == "list_projects":
            async with httpx.AsyncClient() as c:
                r = await c.get(ENDPOINTS[name], headers=headers)
                return r.json() if r.status_code == 200 else {"error": r.text}
        elif name == "create_project":
            async with httpx.AsyncClient() as c:
                r = await c.post(ENDPOINTS[name], json={"name": args["name"]}, headers=headers)
                return r.json() if r.status_code == 200 else {"error": r.text}
        elif name == "run_sql":
            import urllib.parse
            encoded = urllib.parse.quote(args["query"])
            url = ENDPOINTS[name].replace("{query}", encoded)
            async with httpx.AsyncClient() as c:
                r = await c.get(url, headers=headers)
                return r.json() if r.status_code == 200 else {"error": r.text}
        elif name == "get_url_config":
            async with httpx.AsyncClient() as c:
                r = await c.get(ENDPOINTS[name], headers=headers)
                return r.json() if r.status_code == 200 else {"error": r.text}
        elif name == "update_url_config":
            async with httpx.AsyncClient() as c:
                r = await c.put(ENDPOINTS[name], json=args, headers=headers)
                return r.json() if r.status_code == 200 else {"error": r.text}
        elif name == "help_user":
            return {"answer": args["message"]}
    except Exception as e:
        return {"error": str(e)}

@app.post("/assist")
async def assist(request: Request):
    token = ""
    auth_header = request.headers.get("Authorization", "")
    if auth_header.startswith("Bearer "):
        token = auth_header[7:]

    data = await request.json()
    query = data.get("query", "")

    messages = [
        {"role": "system", "content": "You are an agentic AI for Blubase. Use the provided tools to help the user. When the user asks to do something, decide which tool to call. If you just want to chat, use the help_user tool."},
        {"role": "user", "content": query}
    ]

    payload = {
        "model": MODEL,
        "messages": messages,
        "tools": TOOLS,
        "tool_choice": "auto",
        "temperature": 0.2
    }

    async with httpx.AsyncClient(timeout=20) as client:
        try:
            resp = await client.post(MISTRAL_URL, json=payload,
                                     headers={"Authorization": f"Bearer {MISTRAL_API_KEY}", "Content-Type": "application/json"})
            resp.raise_for_status()
            resp_data = resp.json()
            msg = resp_data["choices"][0]["message"]

            if "tool_calls" in msg and msg["tool_calls"]:
                tool_call = msg["tool_calls"][0]
                func_name = tool_call["function"]["name"]
                args = json.loads(tool_call["function"]["arguments"]) if tool_call["function"]["arguments"] else {}
                result = await execute_tool(func_name, args, token)
                return {"answer": json.dumps(result, indent=2), "action": func_name}
            else:
                return {"answer": msg.get("content", "I'm not sure how to help.")}
        except Exception as e:
            return {"answer": f"Agent error: {str(e)}"}

@app.get("/health")
def health():
    return {"status": "ok"}
