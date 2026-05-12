import os
import time

import psycopg2
from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse, Response
from prometheus_client import Counter, Histogram, REGISTRY, generate_latest

app = FastAPI()

DATABASE_URL = os.environ.get("DATABASE_URL")


def _get_db_conn():
    """Return a new connection to the database."""
    return psycopg2.connect(DATABASE_URL)


@app.on_event("startup")
def _init_db():
    if DATABASE_URL is None:
        return
    conn = _get_db_conn()
    try:
        with conn.cursor() as cur:
            cur.execute(
                "CREATE TABLE IF NOT EXISTS visits "
                "(id SERIAL PRIMARY KEY, timestamp TIMESTAMPTZ DEFAULT NOW())"
            )
        conn.commit()
    finally:
        conn.close()

REQUEST_COUNT = Counter(
    "http_requests_total",
    "Total HTTP requests",
    ["method", "path", "status"],
)

REQUEST_DURATION = Histogram(
    "http_request_duration_seconds",
    "HTTP request duration in seconds",
    ["method", "path"],
)


@app.middleware("http")
async def metrics_middleware(request: Request, call_next):
    start = time.perf_counter()
    response = await call_next(request)
    duration = time.perf_counter() - start

    path = request.url.path
    method = request.method
    status = str(response.status_code)

    REQUEST_COUNT.labels(method=method, path=path, status=status).inc()
    REQUEST_DURATION.labels(method=method, path=path).observe(duration)

    return response


@app.get("/")
async def root():
    return {"app": "sample-app", "team": "team-alpha", "version": "1.0.0"}


@app.get("/health")
async def health():
    return {"status": "ok"}


@app.get("/db-health")
async def db_health():
    if DATABASE_URL is None:
        return {"status": "no database configured"}
    conn = _get_db_conn()
    try:
        with conn.cursor() as cur:
            cur.execute("INSERT INTO visits DEFAULT VALUES")
            cur.execute("SELECT COUNT(*) FROM visits")
            count = cur.fetchone()[0]
        conn.commit()
        return {"status": "ok", "visits": count}
    finally:
        conn.close()


@app.get("/metrics")
async def metrics():
    data = generate_latest(REGISTRY)
    return Response(content=data, media_type="text/plain; charset=utf-8")
