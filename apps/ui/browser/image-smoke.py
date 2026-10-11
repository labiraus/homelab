"""Check the complete UI image over its own loopback, with networking disabled."""
import re
import subprocess
import sys
import time


def command(*args):
    # A create-time pull reports progress on stderr; stdout is only the ID.
    return subprocess.check_output(args)


container = command("docker", "create", "--network", "none", sys.argv[1]).decode().strip()
try:
    command("docker", "start", container)
    command("docker", "exec", container, "nginx", "-t")
    for attempt in range(30):
        try:
            html = command("docker", "exec", container, "wget", "-qO-", "http://127.0.0.1/").decode()
            break
        except subprocess.CalledProcessError:
            if attempt == 29:
                raise
            time.sleep(0.2)
    assets = re.findall(r'(?:src|href)="(/assets/[^"\s]+\.(?:js|css))"', html)
    assert any(asset.endswith(".js") for asset in assets), "built module script missing"
    assert any(asset.endswith(".css") for asset in assets), "built stylesheet missing"
    for asset in assets:
        data = command("docker", "exec", container, "wget", "-qO-", "http://127.0.0.1" + asset)
        assert data, f"empty asset: {asset}"
    print(f"Verified Nginx, index page and {len(assets)} compiled assets with container networking disabled.")
finally:
    subprocess.run(["docker", "rm", "-f", container], check=True, stdout=subprocess.DEVNULL)
