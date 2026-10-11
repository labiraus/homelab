#!/usr/bin/env python3
"""Test the role's S3 configuration on disposable local data, never a supplied endpoint."""
import importlib.util
import argparse
import json
import os
from pathlib import Path
import shlex
import shutil
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
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--application-clients", action="store_true")
    parser.add_argument("--large-migration", action="store_true", help="Required release gate: real >5 GiB multipart round trip")
    args = parser.parse_args()
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

    def client(name, unsigned=False, endpoint=ENDPOINT, read_timeout=10):
        return boto3.client(
            "s3", endpoint_url=endpoint, region_name="us-east-1",
            aws_access_key_id="test-" + name, aws_secret_access_key="test-secret-" + name,
            config=Config(signature_version=UNSIGNED if unsigned else "s3v4",
                          s3={"addressing_style": "path"},
                          proxies={}, retries={"max_attempts": 0}, connect_timeout=2, read_timeout=read_timeout),
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
        binary = shutil.which("weed")
        if not binary:
            raise RuntimeError("Pinned SeaweedFS binary is required")
        command = command.replace("/usr/local/bin/weed", shlex.quote(binary), 1)
        assert "BindsTo=test.mount" in unit and "AssertPathIsMountPoint=/srv/minio" in unit
        assert "-s3.ip.bind=127.0.0.1" in command
        proxy_unit = template_env.get_template("seaweedfs-s3-proxy.service.j2").render(values)
        proxy_command = next(line.removeprefix("ExecStart=") for line in proxy_unit.splitlines()
                             if line.startswith("ExecStart="))
        proxy_binary = shutil.which("systemd-socket-proxyd") or "/lib/systemd/systemd-socket-proxyd"
        proxy_command = proxy_command.replace("/lib/systemd/systemd-socket-proxyd", shlex.quote(proxy_binary), 1)
        socket_unit = template_env.get_template("seaweedfs-s3-proxy.socket.j2").render(values)
        assert "ListenStream=127.0.0.2:8333" in socket_unit
        log = (base / "server.log").open("w+")
        server = proxy = None

        def start():
            process = subprocess.Popen(shlex.split(command), cwd=base / "config",
                                       stdout=log, stderr=subprocess.STDOUT,
                                       env={"PATH": os.environ["PATH"], "LANG": "C.UTF-8"})
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
            proxy_environment = ["-E", "LD_LIBRARY_PATH"] if os.environ.get("LD_LIBRARY_PATH") else []
            proxy = subprocess.Popen(["systemd-socket-activate", *proxy_environment, "-l", "127.0.0.2:8333",
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
            # Separate synthetic buckets exercise the archive/copy tool without
            # touching the application's fixtures or accepting a remote endpoint.
            spec = importlib.util.spec_from_file_location("migration", ROOT / "scripts/storage/migrate_versions.py")
            migration = importlib.util.module_from_spec(spec)
            spec.loader.exec_module(migration)
            for bucket in ("migration-source", "migration-target"):
                admin.create_bucket(Bucket=bucket)
                admin.put_bucket_versioning(Bucket=bucket, VersioningConfiguration={"Status": "Enabled"})
            request = dict(Bucket="migration-source", Key="folder/café.txt")
            original = admin.put_object(**request, Body=b"original", ContentType="text/plain", Metadata={"proof": "kept"}, Tagging="kind=history")
            time.sleep(1.05)
            admin.put_object(**request, Body=b"replacement", ContentType="text/plain")
            time.sleep(1.05)
            admin.delete_object(**request)
            admin.put_object(Bucket="migration-source", Key="empty.txt", Body=b"", ContentType="text/plain")
            archive = base / "archive"
            migration.export_archive(admin, "migration-source", archive)
            migration.check_archive(archive)
            mapping = migration.import_archive(admin, "migration-target", archive)
            migration.verify_copy(admin, "migration-target", mapping)
            mapped = next(r for r in mapping["entries"] if r["sourceVersion"] == original["VersionId"])
            assert admin.get_object(Bucket="migration-target", Key=request["Key"], VersionId=mapped["destinationVersion"])["Body"].read() == b"original"
            # The source remains byte-for-byte readable after copying.
            assert admin.get_object(**request, VersionId=original["VersionId"])["Body"].read() == b"original"
            try:
                migration.import_archive(admin, "migration-target", archive)
            except ValueError:
                pass
            else:
                raise AssertionError("Nonempty destination accepted")
            # Recovery after a destination write needs a fresh store and composed
            # aliases, never an endpoint-only rollback into the populated source.
            admin.put_object(Bucket="migration-target", Key="new-after-cutover.txt", Body=b"new work", ContentType="text/plain")
            admin.create_bucket(Bucket="migration-recovery")
            admin.put_bucket_versioning(Bucket="migration-recovery", VersioningConfiguration={"Status": "Enabled"})
            recovery_archive = base / "recovery-archive"
            migration.export_archive(admin, "migration-target", recovery_archive)
            recovered = migration.import_archive(admin, "migration-recovery", recovery_archive, mapping)
            migration.verify_copy(admin, "migration-recovery", recovered)
            old = next(r for r in recovered["entries"] if original["VersionId"] in r.get("aliases", []))
            assert admin.get_object(Bucket="migration-recovery", Key=request["Key"], VersionId=old["destinationVersion"])["Body"].read() == b"original"
            assert admin.get_object(Bucket="migration-recovery", Key="new-after-cutover.txt")["Body"].read() == b"new work"
            print("PASS: version archive/copy, hashes, metadata/tags, delete markers, empty content and source preservation", flush=True)
            print("PASS: recovery to a fresh store preserves original aliases and post-cutover writes", flush=True)
            spec = importlib.util.spec_from_file_location("multipart_acceptance", ROOT / "scripts/storage/multipart_acceptance.py")
            multipart = importlib.util.module_from_spec(spec)
            spec.loader.exec_module(multipart)
            multipart.run(client("admin", read_timeout=120), base, migration, large=args.large_migration)
            if args.application_clients:
                test_env = dict(os.environ, SEAWEEDFS_CLIENT_TEST="1")
                subprocess.run(["go", "test", "-count=1", "-run", "^TestSeaweedFSClients$", "-v", "."],
                               cwd=ROOT / "apps/orchestrator", env=test_env, check=True, timeout=300)
                subprocess.run(["node", "--import", "tsx", "--test", "test/storage.integration.test.ts"],
                               cwd=ROOT / "apps/processor", env=test_env, check=True, timeout=120)
            print("PASS: role templates, S3 socket proxy, idempotent bucket bootstrap, authentication, bucket isolation, metadata, "
                  "versions, copy, range reads, multipart upload, delete markers and restart persistence")
        except Exception:
            log.flush()
            details = (base / "server.log").read_text()
            print(details[:1500] + "\n...\n" + details[-6500:])
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
