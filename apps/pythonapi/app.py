import json
import os
import sys
import logging
import random
import time

import requests
from flask import Flask
from UserResponse import UserResponse

# Set up logging
logging.basicConfig(level=logging.INFO, stream=sys.stdout)

# Set up flask
app = Flask(__name__)

@app.route("/hello", methods=["POST"])
def hello_python():
    logging.info("hello_python() called")

    api_url = "http://userapi/user"
    request = {"userid": 1}

    response = requests.post(api_url, json=request)
    if not response.ok:
        logging.warning(f"A non-success response code: {response.status_code} was returned. ")

    loaded_json = json.loads(response.text)
    user = UserResponse(**loaded_json)
    to_return = {"data": "Hello " + user.username + " (called via Python 🐍)!"}

    return to_return

@app.route("/python/benchmarking", methods=["GET"])
def benchmarking():
    min_ms = 100 # 0.1 seconds
    max_ms = 1000 # 1 second
    random_ms = random.randint(min_ms, max_ms)
    time.sleep(random_ms / 1000.0)
    to_return = {"data": "benchmarking test!"}

    return to_return


@app.route("/liveness")
def healthx():
    return "", 200


@app.route("/readiness")
def healthz():
    return "", 200


if __name__ == "__main__":
    port = int(os.environ.get("PORT", 8080))
    print(f"Port: {port}")
    app.run(host="0.0.0.0", port=port)
