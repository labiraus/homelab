"""Known mutation order against the harness's disposable SeaweedFS only."""
import time


def run(client, base, migration):
    for bucket in ("order-source", "order-target", "order-null-source", "order-null-target", "order-null-tied"):
        client.create_bucket(Bucket=bucket)
        client.put_bucket_versioning(Bucket=bucket, VersioningConfiguration={"Status": "Enabled"})
    key, writes = "folder/café + %2F.bin", []
    # Several mutations normally share a whole-second LastModified. The test
    # asserts this rather than mistaking distinct timestamps for tie coverage.
    for number in range(9):
        request = dict(Bucket="order-source", Key=key)
        deleted = number % 2 == 1
        response = client.delete_object(**request) if deleted else client.put_object(**request, Body=f"body-{number}".encode())
        writes.append((response["VersionId"], deleted))
    client.put_object(Bucket="order-source", Key="z-deleted", Body=b"last")
    client.delete_object(Bucket="order-source", Key="z-deleted")
    archive = base / "order-archive"
    manifest = migration.export_archive(client, "order-source", archive, page_sizes=(2, 3))
    values = [r for r in manifest["entries"] if r["key"] == key]
    assert [(r["sourceVersion"], r["deleteMarker"]) for r in values] == writes
    assert any(a["modified"] == b["modified"] and a["deleteMarker"] != b["deleteMarker"]
               for a, b in zip(values, values[1:])), "Fixture did not exercise a mixed-kind timestamp tie"
    assert all(len(s["pages"]) > 1 for s in manifest["orderEvidence"]["scans"])
    mapping = migration.import_archive(client, "order-target", archive)
    assert mapping["schemaVersion"] == 1
    # Re-export rapid destination writes to check every intermediate position,
    # not only latest state. Import no longer spaces writes artificially.
    reexport = migration.export_archive(client, "order-target", base / "order-reexport", page_sizes=(3, 2))
    migration.check_archive(base / "order-reexport")
    assert [(r["key"], r["sourceVersion"]) for r in reexport["entries"]] == [
        (r["key"], r["destinationVersion"]) for r in mapping["entries"]]
    for number, (version, deleted) in enumerate(writes):
        if not deleted:
            body = client.get_object(Bucket="order-source", Key=key, VersionId=version)["Body"]
            try:
                assert body.read() == f"body-{number}".encode()
            finally:
                body.close()
    print("PASS: real mixed-kind timestamp ties across two pagination sizes, every copied historical position and source preservation", flush=True)

    request = dict(Bucket="order-null-source", Key=key)
    old = client.put_object(**request, Body=b"old")
    time.sleep(1.05)
    client.put_bucket_versioning(Bucket=request["Bucket"], VersioningConfiguration={"Status": "Suspended"})
    client.put_object(**request, Body=b"latest-null")
    null_archive = base / "order-null-archive"
    manifest = migration.export_archive(client, request["Bucket"], null_archive, page_sizes=(1, 2))
    assert [r["sourceVersion"] for r in manifest["entries"]] == [old["VersionId"], "null"]
    assert manifest["entries"][-1]["latest"]
    migration.import_archive(client, "order-null-target", null_archive)
    print("PASS: latest null version positioned by distinct timestamp, paginated and copied with native destination IDs", flush=True)

    # Find a real tied null/non-null pair without relying on fixture timing at
    # one particular second boundary. All writes remain in this test bucket.
    tied = False
    for attempt in range(6):
        request = dict(Bucket="order-null-tied", Key=f"tie-{attempt}")
        client.put_bucket_versioning(Bucket=request["Bucket"], VersioningConfiguration={"Status": "Enabled"})
        client.put_object(**request, Body=b"version")
        client.put_bucket_versioning(Bucket=request["Bucket"], VersioningConfiguration={"Status": "Suspended"})
        client.put_object(**request, Body=b"null")
        values = [r for r in migration.inventory(client, request["Bucket"], validate_order=False) if r["key"] == request["Key"]]
        if len(values) == 2 and values[0]["modified"] == values[1]["modified"]:
            tied = True
            break
    assert tied, "Fixture could not establish a real null-version timestamp tie"
    refused = base / "order-null-refused"
    try:
        migration.export_archive(client, "order-null-tied", refused, page_sizes=(1, 2))
    except ValueError as error:
        assert "Null version shares" in str(error), str(error)
    else:
        raise AssertionError("Unprovable null tie produced an archive")
    assert not (refused / "manifest.json").exists()
    print("PASS: real tied null history refused without a completed archive or source mutation", flush=True)
