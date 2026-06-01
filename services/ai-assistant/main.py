import os, json, httpx
from fastapi import FastAPI, Request
from pydantic import BaseModel

MISTRAL_API_KEY = os.getenv("MISTRAL_API_KEY")
MISTRAL_URL = "https://api.mistral.ai/v1/chat/completions"
MODEL = "mistral-large-latest"

app = FastAPI()

# Define tools in Mistral format
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
            "description": "Get the URL configuration (site URL, redirect URLs, JWT expiry)",
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

API_BASE = "http://127.0.0.1"

async def call_internal(method, path, token=None, body=None):
    headers = {}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    async with httpx.AsyncClient(timeout=15) as client:
        if method == "GET":
            resp = await client.get(f"{API_BASE}{path}", headers=headers)
        elif method == "POST":
            resp = await client.post(f"{API_BASE}{path}", json=body, headers=headers)
        elif method == "PUT":
            resp = await client.put(f"{API_BASE}{path}", json=body, headers=headers)
        return resp.json() if resp.status_code == 200 else {"error": resp.text}

async def execute_tool(name, args, token):
    if name == "list_projects":
        return await call_internal("GET", "/projects", token=token)
    elif name == "create_project":
        return await call_internal("POST", "/projects", token=token, body={"name": args["name"]})
    elif name == "run_sql":
        query = args["query"]
        # URL-encode query
        import urllib.parse
        encoded = urllib.parse.quote(query)
        return await call_internal("GET", f"/sql?query={encoded}", token=token)
    elif name == "get_url_config":
        return await call_internal("GET", "/auth/admin/url-config", token=token)
    elif name == "update_url_config":
        return await call_internal("PUT", "/auth/admin/url-config", token=token, body=args)
    elif name == "help_user":
        return {"answer": args["message"]}
    return {"error": f"unknown tool {name}"}

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

    headers = {
        "Authorization": f"Bearer {MISTRAL_API_KEY}",
        "Content-Type": "application/json"
    }
    payload = {
        "model": MODEL,
        "messages": messages,
        "tools": TOOLS,
        "tool_choice": "auto",
        "temperature": 0.2
    }

    async with httpx.AsyncClient(timeout=30) as client:
        try:
            resp = await client.post(MISTRAL_URL, json=payload, headers=headers)
            resp.raise_for_status()
            resp_data = resp.json()
            msg = resp_data["choices"][0]["message"]

            if "tool_calls" in msg and msg["tool_calls"]:
                # Take the first tool call
                tool_call = msg["tool_calls"][0]
                func_name = tool_call["function"]["name"]
                args = json.loads(tool_call["function"]["arguments"]) if tool_call["function"]["arguments"] else {}
                result = await execute_tool(func_name, args, token)
                return {"answer": json.dumps(result, indent=2), "action": func_name}
            else:
                # No tool call – return the model's text response
                return {"answer": msg.get("content", "I'm not sure how to help with that.")}
        except Exception as e:
            return {"answer": f"Agent error: {str(e)}"}

@app.get("/health")
def health():
    return {"status": "ok"}
