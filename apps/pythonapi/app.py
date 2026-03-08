import logging
import os
import random
import time

import requests
from flask import Flask, jsonify

from UserResponse import UserResponse

DEFAULT_PORT = 8080
DEFAULT_USER_API_URL = "http://userapi/user"
DEFAULT_REQUEST_TIMEOUT_SECONDS = 5
BENCHMARK_MIN_MS = 100
BENCHMARK_MAX_MS = 1000


def configure_logging() -> None:
    logging.basicConfig(level=os.getenv("LOG_LEVEL", "INFO"))


def create_app() -> Flask:
    app = Flask(__name__)
    app.config["USER_API_URL"] = os.getenv("USER_API_URL", DEFAULT_USER_API_URL)
    app.config["USER_API_TIMEOUT_SECONDS"] = float(
        os.getenv("USER_API_TIMEOUT_SECONDS", DEFAULT_REQUEST_TIMEOUT_SECONDS)
    )

    session = requests.Session()

    @app.post("/hello")
    def hello_python():
        app.logger.info("hello_python called")

        response = session.post(
            app.config["USER_API_URL"],
            json={"userid": 1},
            timeout=app.config["USER_API_TIMEOUT_SECONDS"],
        )
        response.raise_for_status()

        loaded_json = response.json()
        user = UserResponse(**loaded_json)

        return jsonify({"data": f"Hello {user.username} (called via Python)!"})

    @app.get("/python/benchmarking")
    def benchmarking():
        random_ms = random.randint(BENCHMARK_MIN_MS, BENCHMARK_MAX_MS)
        time.sleep(random_ms / 1000.0)
        return jsonify({"data": "benchmarking test!"})

    @app.get("/liveness")
    def healthx():
        return "", 200

    @app.get("/readiness")
    def healthz():
        return "", 200

    return app


configure_logging()
app = create_app()


if __name__ == "__main__":
    port = int(os.getenv("PORT", DEFAULT_PORT))
    app.run(host="0.0.0.0", port=port)
