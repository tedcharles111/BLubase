import os
from fastapi import FastAPI
from pydantic import BaseModel
import httpx

MISTRAL_API_KEY = os.getenv("MISTRAL_API_KEY")
MISTRAL_URL = "https://api.mistral.ai/v1/chat/completions"
MODEL = "mistral-large-latest"

app = FastAPI()

class Query(BaseModel):
    query: str
    context: str = ""

@app.get("/health")
def health():
    return {"status": "ok"}

@app.post("/assist")
async def assist(q: Query):
    if not MISTRAL_API_KEY:
        return {"answer": "Mistral API key not set."}
    headers = {
        "Authorization": f"Bearer {MISTRAL_API_KEY}",
        "Content-Type": "application/json"
    }
    payload = {
        "model": MODEL,
        "messages": [
            {"role": "system", "content": "You are a database and backend expert."},
            {"role": "user", "content": q.query}
        ]
    }
    async with httpx.AsyncClient(timeout=20.0) as client:
        try:
            resp = await client.post(MISTRAL_URL, json=payload, headers=headers)
            resp.raise_for_status()
            data = resp.json()
            answer = data["choices"][0]["message"]["content"]
            return {"answer": answer}
        except Exception as e:
            return {"answer": f"Error: {str(e)}"}
