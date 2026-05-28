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

@app.post("/assist")
async def assist(q: Query):
    if not MISTRAL_API_KEY:
        return {"answer": "Mistral API key not set."}
    headers = {
        "Authorization": f"Bearer {MISTRAL_API_KEY}",
        "Content-Type": "application/json"
    }
    system_prompt = "You are a database and backend expert. Help the user with their queries."
    if q.context:
        system_prompt += f" Context: {q.context}"
    payload = {
        "model": MODEL,
        "messages": [
            {"role": "system", "content": system_prompt},
            {"role": "user", "content": q.query}
        ]
    }
    async with httpx.AsyncClient() as client:
        try:
            resp = await client.post(MISTRAL_URL, json=payload, headers=headers, timeout=30.0)
            resp.raise_for_status()
            data = resp.json()
            answer = data["choices"][0]["message"]["content"]
            return {"answer": answer}
        except Exception as e:
            return {"answer": f"Error calling Mistral: {str(e)}"}
