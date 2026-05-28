FROM python:3.12-alpine
WORKDIR /app
COPY requirements.txt .
RUN pip install -r requirements.txt
COPY . .
EXPOSE 3006
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "3006"]
