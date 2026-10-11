#!/usr/bin/env python3
"""Test the role's S3 configuration on disposable local data, never a supplied endpoint."""
import importlib.util
import json
import os
from pathlib import Path
import shlex
import subprocess
import tempfile
import time

import boto3
from botocore import UNSIGNED
from botocore.config import Config
from botocore.exceptions import ClientError, ConnectionClosedError, EndpointConnectionError
import jinja2
import yaml

ROOT = Path(__file__).resolve().parents[1]
ROLE = ROOT / "ansible/roles/seaweedfs"
ENDPOINT = "http://127.0.0.2:8333"


def denied(operation, **kwargs):
    try:
        operation(**kwargs)
    except ClientError as exc:
        assert exc.response["ResponseMetadata"]["HTTPStatusCode"] == 403, exc
    else:
        raise AssertionError("Expected access denial")


def main():
    values = yaml.safe_load((ROLE / "defaults/main.yml").read_text())
    fake_env = {}
    for identity in values["seaweedfs_identities"]:
        fake_env[identity["access_key_env"]] = "test-" + identity["name"]
        fake_env[identity["secret_key_env"]] = "test-secret-" + identity["name"]
    template_env = jinja2.Environment(loader=jinja2.FileSystemLoader(ROLE / "templates"))
    template_env.filters["to_json"] = json.dumps
    template_env.globals["lookup"] = lambda plugin, name: fake_env[name]
    spec = importlib.util.spec_from_file_location("bootstrap", ROLE / "files/bootstrap_buckets.py")
    bootstrap = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(bootstrap)

    def client(name, unsigned=False, endpoint=ENDPOINT):
        return boto3.client(
            "s3", endpoint_url=endpoint, region_name="us-east-1",
            aws_access_key_id="test-" + name, aws_secret_access_key="test-secret-" + name,
            config=Config(signature_version=UNSIGNED if unsigned else "s3v4",
                          s3={"addressing_style": "path"},
                          retries={"max_attempts": 0}, connect_timeout=2, read_timeout=10),
        )

    with tempfile.TemporaryDirectory(prefix="homelab-seaweedfs-") as temporary:
        base = Path(temporary)
        for directory in ("volumes", "metadata/master", "metadata/filer", "config"):
            (base / directory).mkdir(parents=True)
        values.update(seaweedfs_volume_dir=str(base / "volumes"),
                      seaweedfs_metadata_dir=str(base / "metadata"),
                      seaweedfs_s3_bind="127.0.0.2", seaweedfs_s3_port=8333,
                      seaweedfs_mount_unit={"stdout": "test.mount"})
        for name in ("s3.json", "filer.toml"):
            rendered = template_env.get_template(name + ".j2").render(values)
            (base / "config" / name).write_text(rendered)
        identities = json.loads((base / "config/s3.json").read_text())["identities"]
        assert len(identities) == 5 and all(i["name"] != "anonymous" for i in identities)
        secrets = list(yaml.safe_load_all(template_env.get_template("k8s-secrets.yaml.j2").render(values)))
        assert [s["metadata"]["name"] for s in secrets] == [
            "documents-s3-credentials", "cnpg-seaweedfs-s3", "velero-s3-credentials"]
        assert "\naws_secret_access_key=test-secret-velero\n" in secrets[2]["stringData"]["cloud"]
        unit = template_env.get_template("seaweedfs.service.j2").render(values)
        command = next(line.removeprefix("ExecStart=") for line in unit.splitlines()
                       if line.startswith("ExecStart="))
        command = command.replace("/etc/seaweedfs/s3.json", str(base / "config/s3.json"))
        assert "BindsTo=test.mount" in unit and "AssertPathIsMountPoint=/srv/minio" in unit
        assert "-s3.ip.bind=127.0.0.1" in command
        proxy_unit = template_env.get_template("seaweedfs-s3-proxy.service.j2").render(values)
        proxy_command = next(line.removeprefix("ExecStart=") for line in proxy_unit.splitlines()
                             if line.startswith("ExecStart="))
        socket_unit = template_env.get_template("seaweedfs-s3-proxy.socket.j2").render(values)
        assert "ListenStream=127.0.0.2:8333" in socket_unit
        log = (base / "server.log").open("w+")
        server = proxy = None

        def start():
            process = subprocess.Popen(shlex.split(command), cwd=base / "config",
                                       stdout=log, stderr=subprocess.STDOUT)
            try:
                for _ in range(60):
                    if process.poll() is not None:
                        raise RuntimeError("SeaweedFS exited during startup")
                    try:
                        client("admin", endpoint="http://127.0.0.1:8333").list_buckets()
                        return process
                    except (ClientError, EndpointConnectionError):
                        time.sleep(1)
                raise RuntimeError("SeaweedFS S3 readiness timed out")
            except Exception:
                process.terminate()
                process.wait(timeout=30)
                raise

        try:
            server = start()
            proxy = subprocess.Popen(["systemd-socket-activate", "-l", "127.0.0.2:8333",
                                      *shlex.split(proxy_command)], stdout=log, stderr=subprocess.STDOUT)
            for attempt in range(10):
                try:
                    client("admin").list_buckets()
                    break
                except (EndpointConnectionError, ConnectionClosedError):
                    if attempt == 9:
                        raise
                    time.sleep(1)
            admin, documents = client("admin"), client("documents")
            assert bootstrap.bootstrap(admin, values["seaweedfs_buckets"])
            assert not bootstrap.bootstrap(admin, values["seaweedfs_buckets"])
            print("PASS: bucket bootstrap and idempotency", flush=True)
            denied(client("anonymous", unsigned=True).list_objects_v2, Bucket="documents")
            denied(documents.list_objects_v2, Bucket="cnpg-backups")
            for name, bucket in (("cnpg", "cnpg-backups"), ("velero", "velero"),
                                 ("site1", "static-sites-site1")):
                scoped = client(name)
                scoped.put_object(Bucket=bucket, Key="probe", Body=b"backup-probe")
                assert scoped.get_object(Bucket=bucket, Key="probe")["Body"].read() == b"backup-probe"
                scoped.list_objects_v2(Bucket=bucket)
                denied(scoped.get_object, Bucket="documents", Key="probe")
            print("PASS: anonymous denial and workload bucket isolation", flush=True)
            key = {"Bucket": "documents", "Key": "history.txt"}
            first = documents.put_object(**key, Body=b"first", ContentType="text/plain", Metadata={"probe": "yes"})
            second = documents.put_object(**key, Body=b"second", ContentType="text/plain")
            assert first.get("VersionId") and second.get("VersionId") != first["VersionId"]
            old = documents.get_object(**key, VersionId=first["VersionId"])
            assert old["Body"].read() == b"first" and old["Metadata"]["probe"] == "yes"
            assert old["ContentType"] == "text/plain"
            assert documents.get_object(**key, Range="bytes=0-2")["Body"].read() == b"sec"
            assert len(documents.list_object_versions(Bucket="documents")["Versions"]) == 2
            documents.copy_object(Bucket="documents", Key="copy.txt", CopySource={**key, "VersionId": first["VersionId"]})
            assert documents.get_object(Bucket="documents", Key="copy.txt")["Body"].read() == b"first"
            upload = documents.create_multipart_upload(Bucket="documents", Key="multipart.bin")
            parts = []
            for number, content in enumerate((b"a" * (5 * 1024 * 1024), b"tail"), start=1):
                part = documents.upload_part(Bucket="documents", Key="multipart.bin", UploadId=upload["UploadId"],
                                             PartNumber=number, Body=content)
                parts.append({"PartNumber": number, "ETag": part["ETag"]})
            documents.complete_multipart_upload(Bucket="documents", Key="multipart.bin", UploadId=upload["UploadId"],
                                                MultipartUpload={"Parts": parts})
            assert documents.head_object(Bucket="documents", Key="multipart.bin")["ContentLength"] == 5 * 1024 * 1024 + 4
            deleted = documents.delete_object(**key)
            assert deleted.get("DeleteMarker")
            assert documents.get_object(**key, VersionId=first["VersionId"])["Body"].read() == b"first"
            print("PASS: object operations and historical versions; restarting server", flush=True)
            server.terminate()
            server.wait(timeout=30)
            server = start()
            # A listing can succeed before restarted volumes have registered with the master.
            for attempt in range(60):
                try:
                    assert documents.get_object(**key, VersionId=first["VersionId"])["Body"].read() == b"first"
                    break
                except ClientError as exc:
                    if exc.response["ResponseMetadata"]["HTTPStatusCode"] < 500 or attempt == 59:
                        raise
                    time.sleep(1)
                except (EndpointConnectionError, ConnectionClosedError):
                    if attempt == 59:
                        raise
                    time.sleep(1)
            assert admin.get_bucket_versioning(Bucket="documents")["Status"] == "Enabled"
            print("PASS: role templates, S3 socket proxy, idempotent bucket bootstrap, authentication, bucket isolation, metadata, "
                  "versions, copy, range reads, multipart upload, delete markers and restart persistence")
        except Exception:
            log.flush()
            print((base / "server.log").read_text()[-8000:])
            raise
        finally:
            if proxy is not None and proxy.poll() is None:
                proxy.terminate()
                proxy.wait(timeout=30)
            if server is not None and server.poll() is None:
                server.terminate()
                server.wait(timeout=30)
            log.close()


if __name__ == "__main__":
    os.environ["AWS_EC2_METADATA_DISABLED"] = "true"
    main()
