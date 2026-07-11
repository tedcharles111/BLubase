import os, sys, traceback, json, urllib.parse

def log_error(msg):
    print(msg, file=sys.stderr)
    with open("/app/ai-assistant/last_error.log", "a") as f:
        f.write(msg + "\n")

try:
    from fastapi import FastAPI, Request
    from fastapi.responses import JSONResponse
    import httpx
except Exception as e:
    log_error(f"ImportError: {e}\n{traceback.format_exc()}")
    sys.exit(2)

MISTRAL_API_KEY = os.getenv("MISTRAL_API_KEY")
if not MISTRAL_API_KEY:
    log_error("FATAL: MISTRAL_API_KEY environment variable is not set.")
    sys.exit(2)

MISTRAL_URL = "https://api.mistral.ai/v1/chat/completions"
MODEL = "mistral-large-latest"

app = FastAPI()

@app.post("/assist")
async def assist(request: Request):
    data = await request.json()
    prompt = data.get("prompt", "")
    try:
        async with httpx.AsyncClient() as client:
            resp = await client.post(
                MISTRAL_URL,
                headers={
                    "Authorization": f"Bearer {MISTRAL_API_KEY}",
                    "Content-Type": "application/json"
                },
                json={
                    "model": MODEL,
                    "messages": [{"role": "user", "content": prompt}]
                },
                timeout=30
            )
            result = resp.json()
            reply = result["choices"][0]["message"]["content"]
            return JSONResponse({"response": reply})
    except Exception as e:
        log_error(f"Mistral API error: {e}")
        return JSONResponse({"error": str(e)}, status_code=500)

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=3006)

# Diagnostic endpoint
@app.get("/error-log")
async def get_error_log():
    try:
        with open("/app/ai-assistant/last_error.log", "r") as f:
            content = f.read()
        return JSONResponse({"log": content})
    except FileNotFoundError:
        return JSONResponse({"log": "No errors yet."})
