import os
from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse
import uvicorn

app = FastAPI()

@app.post("/assist")
async def assist(request: Request):
    data = await request.json()
    prompt = data.get("prompt", "")
    return JSONResponse({"response": f"You asked: {prompt}"})

if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=3006)
