"""Checks the data source outputs of dev/datasources.tf read from stdin."""
import json
import sys

out = {k: v["value"] for k, v in json.load(sys.stdin).items() if k.startswith("ds_")}
checks = {
    "ds_account_name": lambda v: v == "TF Dev",
    "ds_cluster_node_count": lambda v: v == 3,
    "ds_datacenter": lambda v: v == "AWS_US_EAST_1",
    "ds_nodes": lambda v: v >= 3,
    "ds_clusters_has_dev": lambda v: v is True,
    "ds_cloud_accounts": lambda v: v >= 1,
    "ds_peerings": lambda v: v == 0,
    "ds_default_version": lambda v: bool(v),
    "ds_has_i8g": lambda v: v is True,
    "ds_vector_types": lambda v: v >= 1,
    "ds_zones": lambda v: v >= 1,
    "ds_create_request": lambda v: v is True,
}
bad = [f"{k}={out[k]!r}" for k in out if k in checks and not checks[k](out[k])]
print("outputs:", json.dumps(out, sort_keys=True))
if bad:
    print("unexpected:", ", ".join(bad))
    sys.exit(1)
