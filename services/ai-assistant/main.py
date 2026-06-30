import os, json, httpx
from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse

MISTRAL_API_KEY = os.getenv("MISTRAL_API_KEY")
MISTRAL_URL = "https://api.mistral.ai/v1/chat/completions"
MODEL = "mistral-large-latest"

app = FastAPI()

@app.post("/assist")
async def assist(request: Request):
    try:
        body = await request.json()
        user_query = body.get("query", "")

        if not MISTRAL_API_KEY or MISTRAL_API_KEY.startswith("YOUR_"):
            return JSONResponse({"answer": "AI assistant is not configured. Please set a valid MISTRAL_API_KEY.", "action": "chat"})

        if not user_query:
            return JSONResponse({"answer": "Please ask a question.", "action": "chat"})

        payload = {
            "model": MODEL,
            "messages": [
                {"role": "system", "content": "You are a helpful AI assistant for Blubase, an open‑source backend platform."},
                {"role": "user", "content": user_query}
            ],
            "temperature": 0.7
        }

        async with httpx.AsyncClient(timeout=30) as client:
            r = await client.post(
                MISTRAL_URL,
                json=payload,
                headers={
                    "Authorization": f"Bearer {MISTRAL_API_KEY}",
                    "Content-Type": "application/json"
                }
            )

            if r.status_code != 200:
                return JSONResponse({"answer": f"Mistral API returned status {r.status_code}. Please check your API key.", "action": "chat"})

            data = r.json()
            answer = data["choices"][0]["message"]["content"]
            return JSONResponse({"answer": answer, "action": "chat"})

    except Exception as e:
        return JSONResponse({"answer": f"AI assistant error: {str(e)}", "action": "chat"})

@app.get("/health")
def health():
    return {"status": "ok"}
