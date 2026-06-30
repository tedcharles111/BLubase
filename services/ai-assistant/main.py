import os, json, httpx, urllib.parse
from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse

MISTRAL_API_KEY = os.getenv("MISTRAL_API_KEY")
MISTRAL_URL = "https://api.mistral.ai/v1/chat/completions"
MODEL = "mistral-large-latest"
API_BASE = "http://127.0.0.1"

app = FastAPI()

# ---------- Tools the agent can use ----------
TOOLS = [
    {"type":"function","function":{"name":"list_projects","description":"List all projects owned by the current user","parameters":{"type":"object","properties":{}}}},
    {"type":"function","function":{"name":"create_project","description":"Create a new project for the current user","parameters":{"type":"object","properties":{"name":{"type":"string","description":"Project name"}},"required":["name"]}}},
    {"type":"function","function":{"name":"run_sql","description":"Execute a SQL query on the user's database","parameters":{"type":"object","properties":{"query":{"type":"string","description":"SQL query to execute"}},"required":["query"]}}},
    {"type":"function","function":{"name":"get_url_config","description":"Get the URL configuration (site URL, redirect URLs, JWT expiry)","parameters":{"type":"object","properties":{}}}},
    {"type":"function","function":{"name":"update_url_config","description":"Update the URL configuration","parameters":{"type":"object","properties":{"site_url":{"type":"string"},"jwt_expiry_hours":{"type":"integer"},"redirect_urls":{"type":"array","items":{"type":"string"}}}}}},
    {"type":"function","function":{"name":"list_oauth_providers","description":"List all configured OAuth providers","parameters":{"type":"object","properties":{}}}},
    {"type":"function","function":{"name":"add_oauth_provider","description":"Add or update an OAuth provider","parameters":{"type":"object","properties":{"provider":{"type":"string"},"client_id":{"type":"string"},"client_secret":{"type":"string"},"enabled":{"type":"boolean"}},"required":["provider","client_id","client_secret"]}}},
    {"type":"function","function":{"name":"list_email_templates","description":"List all email templates","parameters":{"type":"object","properties":{}}}},
    {"type":"function","function":{"name":"create_email_template","description":"Create or update an email template","parameters":{"type":"object","properties":{"name":{"type":"string"},"subject":{"type":"string"},"body":{"type":"string"}},"required":["name","subject","body"]}}},
    {"type":"function","function":{"name":"get_smtp_config","description":"Get SMTP configuration","parameters":{"type":"object","properties":{}}}},
    {"type":"function","function":{"name":"update_smtp_config","description":"Update SMTP configuration","parameters":{"type":"object","additionalProperties":{"type":"string"}}}},
    {"type":"function","function":{"name":"forgot_password","description":"Request a password reset OTP","parameters":{"type":"object","properties":{"email":{"type":"string"}},"required":["email"]}}},
    {"type":"function","function":{"name":"reset_password","description":"Reset password using OTP","parameters":{"type":"object","properties":{"email":{"type":"string"},"otp":{"type":"string"},"new_password":{"type":"string"}},"required":["email","otp","new_password"]}}},
    {"type":"function","function":{"name":"help_user","description":"Provide a helpful text response (only when no other tool is appropriate)","parameters":{"type":"object","properties":{"message":{"type":"string"}},"required":["message"]}}}
]

# ---------- Internal endpoints ----------
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
        else:
            return {"error": f"unsupported method {method}"}
        try:
            return r.json() if r.status_code == 200 else {"error": r.text}
        except:
            return {"error": r.text}

@app.post("/assist")
async def assist(request: Request):
    try:
        token = request.headers.get("Authorization", "").removeprefix("Bearer ")
        body = await request.json()
        user_query = body.get("query", "")

        if not MISTRAL_API_KEY:
            return JSONResponse({"answer": "AI assistant is not configured.", "action": "chat"})

        messages = [
            {"role": "system", "content": "You are an agentic AI for Blubase. Use the provided tools to directly perform actions for the user. When asked to do something, decide which tool to call. Never tell the user to do it themselves – always use a tool to do it for them. If you just want to chat, use the help_user tool."},
            {"role": "user", "content": user_query}
        ]

        payload = {
            "model": MODEL,
            "messages": messages,
            "tools": TOOLS,
            "tool_choice": "auto",
            "temperature": 0.2
        }

        async with httpx.AsyncClient(timeout=30) as client:
            r = await client.post(MISTRAL_URL, json=payload,
                                 headers={"Authorization": f"Bearer {MISTRAL_API_KEY}", "Content-Type": "application/json"})
            r.raise_for_status()
            data = r.json()
            msg = data["choices"][0]["message"]

            # If the model wants to use a tool
            if "tool_calls" in msg and msg["tool_calls"]:
                tc = msg["tool_calls"][0]
                fname = tc["function"]["name"]
                fargs = json.loads(tc["function"]["arguments"]) if tc["function"]["arguments"] else {}
                result = await execute_tool(fname, fargs, token)
                return JSONResponse({
                    "answer": json.dumps(result, indent=2),
                    "action": fname
                })
            else:
                # Fallback: the model responded with plain text
                return JSONResponse({
                    "answer": msg.get("content", "I'm not sure how to help with that."),
                    "action": "chat"
                })

    except Exception as e:
        return JSONResponse({"answer": f"Agent error: {str(e)}", "action": "chat"})

@app.get("/history")
async def history(request: Request):
    return []

@app.get("/health")
def health():
    return {"status": "ok"}
