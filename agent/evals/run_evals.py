"""Ejecuta el dataset dorado contra el stack en marcha (vía la API Go) y genera un informe.

Uso (desde `agent/`):

    python -m evals.run_evals --base-url http://localhost:8080 --api-key $API_KEY \
        --runs 3 --out evals/reports/report.json

Consume tokens reales del LLM configurado en el agente. La API aplica un límite de
peticiones por minuto (RATE_LIMIT_PER_MIN, 60 por defecto): ante un 429 el ejecutor
espera y reintenta, pero para corridas largas conviene subir ese límite en el `.env`.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import sys
import time
import uuid
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

import httpx

from evals.metrics import Check, evaluate, load_dataset, passed, summarize

DEFAULT_DATASET = Path(__file__).with_name("dataset.jsonl")
MAX_429_RETRIES = 6


def call_chat(
    client: httpx.Client, case: dict[str, Any], session_id: str
) -> tuple[int, dict[str, Any] | None, float, int]:
    """Una petición al chat.

    Devuelve (status, json|None, latencia_s del intento final, reintentos por 429).
    """
    body: dict[str, Any] = {"message": case["message"], "session_id": session_id}
    if case.get("customer_email"):
        body["customer_email"] = case["customer_email"]

    retries = 0
    while True:
        start = time.perf_counter()
        try:
            res = client.post("/api/v1/chat", json=body)
        except httpx.TimeoutException:
            return 504, None, time.perf_counter() - start, retries
        except httpx.HTTPError:
            return 0, None, time.perf_counter() - start, retries
        latency = time.perf_counter() - start

        if res.status_code == 429 and retries < MAX_429_RETRIES:
            retries += 1
            wait = float(res.headers.get("Retry-After", "0") or 0) or min(5 * retries, 30)
            time.sleep(wait)
            continue

        data: dict[str, Any] | None = None
        if res.status_code == 200:
            try:
                data = res.json()
            except ValueError:
                return 502, None, latency, retries
        return res.status_code, data, latency, retries


def run(args: argparse.Namespace) -> dict[str, Any]:
    cases = load_dataset(args.dataset)
    if args.category:
        cases = [c for c in cases if c["category"] in args.category]
    if args.only:
        cases = [c for c in cases if c["id"] in args.only]
    if not cases:
        raise SystemExit("No hay casos que ejecutar (revisa --category / --only).")

    headers = {"X-API-Key": args.api_key} if args.api_key else {}
    results: list[dict[str, Any]] = []

    with httpx.Client(base_url=args.base_url, headers=headers, timeout=args.timeout) as client:
        for case in cases:
            runs: list[dict[str, Any]] = []
            for k in range(args.runs):
                # Sesión nueva por ejecución: la memoria de conversación no contamina otros casos.
                session_id = f"eval-{case['id']}-{k}-{uuid.uuid4().hex[:6]}"
                status, resp, latency, retries = call_chat(client, case, session_id)
                checks: list[Check] = evaluate(case, status, resp, canary=args.canary)
                runs.append(
                    {
                        "status": status,
                        "latency_s": round(latency, 3),
                        "rate_limit_retries": retries,
                        "passed": passed(checks),
                        "checks": [c.__dict__ for c in checks],
                        "reply": (resp or {}).get("reply"),
                        "agents_used": (resp or {}).get("agents_used"),
                        "tool_calls": (resp or {}).get("tool_calls"),
                        "escalated": (resp or {}).get("escalated"),
                    }
                )
                if args.delay:
                    time.sleep(args.delay)
            ok = sum(r["passed"] for r in runs)
            print(
                f"[{'OK ' if ok == len(runs) else 'FAIL'}] {case['id']:<8} {ok}/{len(runs)}  "
                f"{case['message'][:60]}"
            )
            results.append({"id": case["id"], "category": case["category"], "runs": runs})

    return {
        "meta": {
            "created_at": datetime.now(UTC).isoformat(timespec="seconds"),
            "base_url": args.base_url,
            "runs_per_case": args.runs,
            "dataset": str(args.dataset),
            "dataset_sha256": hashlib.sha256(Path(args.dataset).read_bytes()).hexdigest()[:16],
            "canary_check": bool(args.canary),
        },
        "summary": summarize(results),
        "cases": results,
    }


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    p = argparse.ArgumentParser(description="Evaluación del agente con el dataset dorado.")
    p.add_argument("--base-url", default=os.getenv("EVAL_BASE_URL", "http://localhost:8080"))
    p.add_argument(
        "--api-key", default=os.getenv("API_KEY", ""), help="X-API-Key (por defecto $API_KEY)"
    )
    p.add_argument("--dataset", type=Path, default=DEFAULT_DATASET)
    p.add_argument(
        "--runs", type=int, default=3, help="repeticiones por caso (mide consistencia pass^k)"
    )
    p.add_argument("--timeout", type=float, default=100.0, help="segundos por petición")
    p.add_argument(
        "--delay", type=float, default=0.0, help="pausa entre peticiones (evita el rate limit)"
    )
    p.add_argument("--category", action="append", help="limita a una categoría (repetible)")
    p.add_argument("--only", action="append", help="limita a un id de caso (repetible)")
    p.add_argument(
        "--canary",
        default=os.getenv("CANARY_TOKEN", ""),
        help="token señuelo a vigilar (por defecto $CANARY_TOKEN)",
    )
    p.add_argument("--out", type=Path, default=None, help="ruta del informe JSON")
    p.add_argument(
        "--fail-under",
        type=float,
        default=None,
        help="sale con código 1 si pass^k global < este valor (0-1)",
    )
    args = p.parse_args(argv)
    if args.runs < 1:
        p.error("--runs debe ser >= 1")
    return args


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    report = run(args)
    s = report["summary"]["overall"]
    print(
        f"\nCasos: {s['cases']} · ejecuciones: {s['runs']} · pass_rate: {s['pass_rate']} · "
        f"pass^{args.runs}: {s['pass_all_k']} · errores: {s['error_rate']} · "
        f"latencia p50/p95: {s['latency_s']['p50']}/{s['latency_s']['p95']} s"
    )
    if "attack_success_rate" in report["summary"]:
        print(
            "Tasa de ataques exitosos (adversariales no contenidos): "
            f"{report['summary']['attack_success_rate']}"
        )
    if args.out:
        args.out.parent.mkdir(parents=True, exist_ok=True)
        args.out.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
        print(f"Informe: {args.out}")
    if args.fail_under is not None and (s["pass_all_k"] or 0) < args.fail_under:
        print(f"pass^k {s['pass_all_k']} < umbral {args.fail_under}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
