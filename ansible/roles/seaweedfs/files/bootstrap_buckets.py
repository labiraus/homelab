"""Create missing S3 buckets and require versioning; never delete data or set expiry."""
import sys

import boto3
from botocore.config import Config


def bootstrap(client, buckets):
    existing = {entry["Name"] for entry in client.list_buckets()["Buckets"]}
    changed = False
    for bucket in buckets:
        if bucket not in existing:
            client.create_bucket(Bucket=bucket)
            changed = True
    if "documents" in buckets:
        if client.get_bucket_versioning(Bucket="documents").get("Status") != "Enabled":
            client.put_bucket_versioning(
                Bucket="documents", VersioningConfiguration={"Status": "Enabled"}
            )
            changed = True
        if client.get_bucket_versioning(Bucket="documents").get("Status") != "Enabled":
            raise RuntimeError("documents bucket versioning was not enabled")
    return changed


if __name__ == "__main__":
    client = boto3.client(
        "s3", endpoint_url=sys.argv[1], region_name="us-east-1",
        config=Config(s3={"addressing_style": "path"}, retries={"max_attempts": 10}),
    )
    print("changed=" + str(bootstrap(client, sys.argv[2:])).lower())
