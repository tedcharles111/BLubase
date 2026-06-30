import os, json, httpx, urllib.parse, asyncpg, traceback
from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse

MISTRAL_API_KEY = os.getenv("MISTRAL_API_KEY")
MISTRAL_URL = "https://api.mistral.ai/v1/chat/completions"
MODEL = "mistral-large-latest"
DATABASE_URL = os.getenv("DATABASE_URL")

app = FastAPI()

# ---------- Memory ----------
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

# ---------- Tools (kept short for now) ----------
TOOLS = [
    {"type":"function","function":{"name":"help_user","description":"Provide a helpful text response","parameters":{"type":"object","properties":{"message":{"type":"string"}},"required":["message"]}}}
]

@app.post("/assist")
async def assist(request: Request):
    try:
        token = request.headers.get("Authorization", "").removeprefix("Bearer ")
        data = await request.json()
        user_query = data.get("query", "")

        # Use a simple fallback if Mistral key is invalid or API fails
        if not MISTRAL_API_KEY or MISTRAL_API_KEY.startswith("YOUR_"):
            return JSONResponse({"answer": "AI assistant is not configured. Please set a valid MISTRAL_API_KEY.", "action": "chat"})

        messages = [
            {"role":"system","content":"You are an AI assistant for Blubase."},
            {"role":"user","content": user_query}
        ]

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
            if r.status_code != 200:
                return JSONResponse({"answer": f"Mistral API returned status {r.status_code}. Please check the API key.", "action": "chat"})
            resp_data = r.json()
            msg = resp_data["choices"][0]["message"]
            answer = msg.get("content", "I'm not sure how to help.")
            return JSONResponse({"answer": answer, "action": "chat"})
    except Exception as e:
        traceback.print_exc()
        return JSONResponse({"answer": f"AI assistant encountered an error: {str(e)}", "action": "chat"})

@app.get("/history")
async def history(request: Request):
    token = request.headers.get("Authorization", "").removeprefix("Bearer ")
    user_id = token.split(".")[0] if token else "anonymous"
    rows = await get_memory(user_id, 50)
    return rows

@app.get("/health")
def health():
    return {"status": "ok"}
