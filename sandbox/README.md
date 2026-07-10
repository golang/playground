# Go Playground Sandbox Backend

This directory contains the source code and deployment configuration for the Go Playground sandbox backend.

The backend is responsible for executing untrusted Go code submitted by users. It uses **gVisor (`runsc`)** inside a Docker container on Container-Optimized OS (COS) VMs to ensure secure isolation.

---

## Architecture Overview

The backend runs as a Managed Instance Group (MIG) of GCE VMs.
*   **Host OS**: Google Container-Optimized OS (COS).
*   **Service**: A systemd service (`playsandbox.service`) started on boot via `cloud-init`.
*   **Runtime**: The systemd service downloads gVisor (`runsc`), configures Docker to use it as a runtime, and starts the `playground-sandbox` container in privileged mode.
*   **Execution**: The sandbox container listens for compile/run requests from the frontend, spawns worker containers using the `runsc` runtime to execute the user code, and returns the output.

---

## Local Development

You can build and run the sandbox backend locally for testing.

### 1. Build Docker Images
To build the sandbox server and gVisor runner images locally:
```bash
make docker TAG=local-test
make dockergvisor TAG=local-test
```

### 2. Run Locally
To run the sandbox server locally, mapped to port `8080`:
```bash
make runlocal TAG=local-test
```
This starts the sandbox in dev mode. You can test it by sending a run request:
```bash
curl -v --data-binary @path/to/hello.go http://localhost:8080/run
```

---

## Release Tagging Convention

When releasing new versions to production, follow this naming convention for both Docker tags and GCE resources to ensure traceability:

*   **Format**: `cos-YYYYMMDD-HASH`
    *   `YYYYMMDD`: The current UTC date.
    *   `HASH`: The 7-character short Git commit hash of the version being deployed (e.g., `git rev-parse --short HEAD`).
*   **Example**: `cos-20260710-a67ec5c`

---

## Production Deployment (GCE)

Deployments are performed manually via the `gcloud` command.
Always use a canary rollout when deploying changes to production.

### Prerequisites
1.  Ensure your git working directory is clean.
2.  Get the short commit hash of `HEAD`:
    ```bash
    git rev-parse --short HEAD
    ```
    (We will use `HASH` as a placeholder below).

### Step 1: Build and Push Production Images
Build and push the new images to the Google Container Registry (GCR) using the tagging convention:
```bash
make push TAG=cos-YYYYMMDD-HASH
```

### Step 2: Generate the Cloud-Init Configuration
Generate the expanded `cloud-init.yaml` file, which embeds the new tag so the VMs pull the correct image on boot:
```bash
make cloud-init.yaml.expanded TAG=cos-YYYYMMDD-HASH
```

### Step 3: Create a GCE Instance Template
Create a new instance template containing the updated metadata config. Replace `<GCP_PROJECT>` with your target GCP project:

```bash
gcloud compute instance-templates create play-sandbox-cos-YYYYMMDD-HASH \
    --project=<GCP_PROJECT> \
    --machine-type=e2-standard-8 \
    --network=golang \
    --no-address \
    --image-project=cos-cloud \
    --image-family=cos-stable \
    --metadata-from-file=user-data=cloud-init.yaml.expanded \
    --scopes=https://www.googleapis.com/auth/devstorage.read_only,https://www.googleapis.com/auth/logging.write,https://www.googleapis.com/auth/monitoring.write
```

### Step 4: Canary Rollout (Recommended)
Deploy the new template to **1 instance** first to verify health under production load.

Replace `<GCP_PROJECT>`, `<REGION>`, and `<MIG_NAME>` with your GCE configuration. `<LEGACY_TEMPLATE>` is the template currently running in production.

```bash
gcloud compute instance-groups managed rolling-action start-update <MIG_NAME> \
    --project=<GCP_PROJECT> \
    --region=<REGION> \
    --version=template=<LEGACY_TEMPLATE> \
    --canary-version=template=play-sandbox-cos-YYYYMMDD-HASH,target-size=1 \
    --max-surge=3 \
    --max-unavailable=0
```
*Note: For regional MIGs, `max-surge` must be at least equal to the number of zones (usually 3).*

### Step 5: Verify Canary Health
1.  Check the status of the instances in the MIG:
    ```bash
    gcloud compute instance-groups managed list-instances <MIG_NAME> \
        --region=<REGION> \
        --project=<GCP_PROJECT>
    ```
    Verify that the new instance is `RUNNING` and has `ACTION=NONE`.
2.  Check the serial console logs of the new instance to verify successful boot and service startup:
    ```bash
    gcloud compute instances get-serial-port-output <NEW_INSTANCE_NAME> \
        --zone=<ZONE> \
        --project=<GCP_PROJECT>
    ```
    Look for `[  OK  ] Started playsandbox.service.` and ensure no repeated container restart loops are logged.
3.  Monitor production metrics/logs for any spike in timeouts or errors.

### Step 6: Promote to 100%
Once the canary is verified healthy (typically after 1-2 hours of monitoring), promote the new template to 100% of the MIG:

```bash
gcloud compute instance-groups managed rolling-action start-update <MIG_NAME> \
    --project=<GCP_PROJECT> \
    --region=<REGION> \
    --version=template=play-sandbox-cos-YYYYMMDD-HASH \
    --max-surge=3 \
    --max-unavailable=0
```

---

## Rollback Procedure

If critical issues are detected after a rollout, you can immediately revert the MIG to the previous stable template.

Run the rolling update command pointing to the last known-good template:

```bash
gcloud compute instance-groups managed rolling-action start-update <MIG_NAME> \
    --project=<GCP_PROJECT> \
    --region=<REGION> \
    --version=template=<LAST_KNOWN_GOOD_TEMPLATE> \
    --max-surge=3 \
    --max-unavailable=0
```
Verify the rollback progress using `list-instances` as described in the verification step.
