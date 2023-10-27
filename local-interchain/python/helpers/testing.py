import time

import httpx


def poll_for_start(API_URL: str, waitSeconds=120):
    for i in range(waitSeconds + 1):
        try:
            httpx.get(API_URL)
            return
        except Exception:
            if i % 5 == 0:
                print(f"waiting for server to start (iter:{i}) ({API_URL})")

            time.sleep(1)

    raise Exception("Local-IC REST API Server did not start")
