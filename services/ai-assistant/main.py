import os, json, httpx
from fastapi import FastAPI, Request
from pydantic import BaseModel

MISTRAL_API_KEY = os.getenv("MISTRAL_API_KEY")
MISTRAL_URL = "https://api.mistral.ai/v1/chat/completions"
MODEL = "mistral-large-latest"

app = FastAPI()

# Allowed agent functions
FUNCTIONS = [
    {
        "name": "list_projects",
        "description": "List all projects owned by the current user",
        "parameters": {}
    },
    {
        "name": "create_project",
        "description": "Create a new project for the current user",
        "parameters": {
            "type": "object",
            "properties": {
                "name": {"type": "string", "description": "Project name"}
            },
            "required": ["name"]
        }
    },
    {
        "name": "run_sql",
        "description": "Execute a SQL query on the user's database",
        "parameters": {
            "type": "object",
            "properties": {
                "query": {"type": "string", "description": "SQL query to execute"}
            },
            "required": ["query"]
        }
    },
    {
        "name": "get_url_config",
        "description": "Get the URL configuration (site URL, redirect URLs, JWT expiry)",
        "parameters": {}
    },
    {
        "name": "update_url_config",
        "description": "Update the URL configuration. Provide at least one field: site_url, jwt_expiry_hours, redirect_urls.",
        "parameters": {
            "type": "object",
            "properties": {
                "site_url": {"type": "string"},
                "jwt_expiry_hours": {"type": "integer"},
                "redirect_urls": {"type": "array", "items": {"type": "string"}}
            }
        }
    },
    {
        "name": "list_oauth_providers",
        "description": "List all configured OAuth providers",
        "parameters": {}
    },
    {
        "name": "add_oauth_provider",
        "description": "Add or update an OAuth provider",
        "parameters": {
            "type": "object",
            "properties": {
                "provider": {"type": "string"},
                "client_id": {"type": "string"},
                "client_secret": {"type": "string"},
                "enabled": {"type": "boolean"}
            },
            "required": ["provider", "client_id", "client_secret"]
        }
    },
    {
        "name": "list_email_templates",
        "description": "List all email templates",
        "parameters": {}
    },
    {
        "name": "create_email_template",
        "description": "Create or update an email template",
        "parameters": {
            "type": "object",
            "properties": {
                "name": {"type": "string"},
                "subject": {"type": "string"},
                "body": {"type": "string"}
            },
            "required": ["name", "subject", "body"]
        }
    },
    {
        "name": "get_smtp_config",
        "description": "Get SMTP configuration",
        "parameters": {}
    },
    {
        "name": "update_smtp_config",
        "description": "Update SMTP configuration (provide key-value pairs)",
        "parameters": {
            "type": "object",
            "additionalProperties": {"type": "string"}
        }
    },
    {
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
]

# Internal API base (services inside the same container)
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
        else:
            return {"error": f"unsupported method {method}"}
        return resp.json() if resp.status_code == 200 else {"error": resp.text}

async def execute_function(func_name, args, token):
    if func_name == "list_projects":
        return await call_internal("GET", "/projects", token=token)
    elif func_name == "create_project":
        return await call_internal("POST", "/projects", token=token, body={"name": args["name"]})
    elif func_name == "run_sql":
        query = args["query"]
        return await call_internal("GET", f"/sql?query={httpx.QueryParam('query', query)}", token=token)
    elif func_name == "get_url_config":
        return await call_internal("GET", "/auth/admin/url-config", token=token)
    elif func_name == "update_url_config":
        return await call_internal("PUT", "/auth/admin/url-config", token=token, body=args)
    elif func_name == "list_oauth_providers":
        return await call_internal("GET", "/auth/admin/oauth-providers", token=token)
    elif func_name == "add_oauth_provider":
        return await call_internal("POST", "/auth/admin/oauth-providers", token=token, body=args)
    elif func_name == "list_email_templates":
        return await call_internal("GET", "/auth/admin/templates", token=token)
    elif func_name == "create_email_template":
        return await call_internal("POST", "/auth/admin/templates", token=token, body=args)
    elif func_name == "get_smtp_config":
        return await call_internal("GET", "/auth/admin/smtp", token=token)
    elif func_name == "update_smtp_config":
        return await call_internal("PUT", "/auth/admin/smtp", token=token, body=args)
    elif func_name == "help_user":
        return {"answer": args["message"]}
    else:
        return {"error": f"unknown function {func_name}"}

@app.post("/assist")
async def assist(request: Request):
    # Extract token from Authorization header
    auth_header = request.headers.get("Authorization", "")
    token = ""
    if auth_header.startswith("Bearer "):
        token = auth_header[7:]

    data = await request.json()
    query = data.get("query", "")
    messages = [
        {
            "role": "system",
            "content": (
                "You are an agentic AI assistant for Blubase, an open‑source backend platform. "
                "You have access to the user's account and can perform actions on their behalf. "
                "When the user asks you to do something, choose the appropriate function from the list below and output ONLY a JSON object with 'function' and 'args'. "
                "If the user just wants to chat, respond with a JSON object containing 'function': 'help_user', 'args': {'message': 'your helpful answer'}. "
                "NEVER include any extra text outside the JSON."
            )
        },
        {"role": "user", "content": query}
    ]

    headers = {
        "Authorization": f"Bearer {MISTRAL_API_KEY}",
        "Content-Type": "application/json"
    }
    payload = {
        "model": MODEL,
        "messages": messages,
        "functions": FUNCTIONS,
        "function_call": "auto",
        "temperature": 0.2
    }

    async with httpx.AsyncClient(timeout=30) as client:
        try:
            resp = await client.post(MISTRAL_URL, json=payload, headers=headers)
            resp.raise_for_status()
            data = resp.json()
            msg = data["choices"][0]["message"]

            # If the model wants to call a function
            if "function_call" in msg and msg["function_call"]:
                fc = msg["function_call"]
                func_name = fc["name"]
                args = json.loads(fc["arguments"]) if fc["arguments"] else {}
                result = await execute_function(func_name, args, token)
                return {"answer": json.dumps(result, indent=2), "action": func_name}
            else:
                # Fallback: treat as help_user
                return {"answer": msg.get("content", "Sorry, I couldn't process that.")}
        except Exception as e:
            return {"answer": f"Agent error: {str(e)}"}

@app.get("/health")
def health():
    return {"status": "ok"}
