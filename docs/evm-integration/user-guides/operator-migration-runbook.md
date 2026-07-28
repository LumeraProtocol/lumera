# EVM Migration Operator Runbook

**Audience:** validators, supernodes, relayers, and custodial account operators
**Scope:** executable safety gates around the existing migration helpers; this does not replace account-specific guides.

Migration is irreversible after the transaction is included. Record commands and public addresses, but never record mnemonics, private keys, keyring passphrases, bearer tokens, or raw environment dumps.

## 1. Discover the process before changing it

Set an evidence directory on an encrypted operator-controlled volume:

```bash
umask 077
EVIDENCE_DIR="$HOME/evmigration-evidence/$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$EVIDENCE_DIR"
```

### systemd

```bash
sudo systemctl show lumerad \
  -p User -p Group -p FragmentPath -p ExecStart -p WorkingDirectory \
  | tee "$EVIDENCE_DIR/systemd-lumerad.txt"
sudo systemctl cat lumerad | tee "$EVIDENCE_DIR/systemd-lumerad-unit.txt"
```

From `User`, `ExecStart`, and the unit environment, identify the service user, exact executable, `--home`/base directory, config directory, and keyring backend/location. Run all helper/keyring commands as that service user. Do not assume the interactive user's `$HOME` or keyring.

### Docker

```bash
docker inspect <container> \
  --format '{{json .Config.User}} {{json .Path}} {{json .Args}} {{json .Mounts}} {{json .Config.Image}}' \
  | tee "$EVIDENCE_DIR/docker-lumerad.txt"
docker image inspect <image> --format '{{json .RepoDigests}}' \
  | tee "$EVIDENCE_DIR/docker-image-digests.txt"
```

Identify the container user, command/arguments, mounted home/config/keyring paths, and immutable image digest. Run the helper in an image/container with those same mounts and identity.

### Kubernetes

```bash
kubectl -n <namespace> get statefulset/<name> -o yaml \
  | tee "$EVIDENCE_DIR/kubernetes-workload.yaml"
kubectl -n <namespace> get pod <pod> -o jsonpath='{range .spec.containers[*]}{.name}{" user="}{.securityContext.runAsUser}{" command="}{.command}{" args="}{.args}{" mounts="}{.volumeMounts}{"\n"}{end}' \
  | tee "$EVIDENCE_DIR/kubernetes-runtime.txt"
```

Identify `runAsUser`, command/args, PVC/config/secret mounts, base directory, and keyring location. The captured YAML is sensitive operational metadata even though it must not contain Secret values; restrict the evidence directory.

## 2. Pin binary, home, and keyring provenance

Use the exact release executables and explicit flags. The helpers resolve `--binary` once to an absolute canonical path, then print that path, the actual `version --long` version, SHA-256, and source before doing key work. They print keyring backend and keyring location separately, with each source. Independently verify all three executable records in the manifest: `artifacts.chain_executable`, `artifacts.supernode_executable`, and `artifacts.sncli_executable`, including each exact `release_path`, version, tag, commit, SHA-256, and source. For container procedures, also pull and inspect the exact `artifacts.container_image.artifact@artifacts.container_image.digest` from `artifacts.container_image.source`; a mutable tag alone is not approved.

```bash
LUMERAD=/absolute/path/to/approved/lumerad
"$LUMERAD" version --long | tee "$EVIDENCE_DIR/lumerad-version.txt"
sha256sum "$LUMERAD" | tee "$EVIDENCE_DIR/lumerad.sha256"

sudo -u <service-user> ./scripts/migrate-validator.sh \
  <legacy-key-name> <destination-key-name> \
  --binary "$LUMERAD" \
  --home /absolute/lumera/home \
  --keyring-backend <test|file|os> \
  --keyring-dir /absolute/keyring/location \
  --chain-id <chain-id> \
  --node <trusted-rpc> \
  --i-have-stopped-the-node \
  --dry-run
```

For a non-validator account, use `migrate-account.sh` and omit `--i-have-stopped-the-node`. Do not continue if the displayed binary, version, checksum, service user, home, backend, or location differs from the approved compatibility manifest.

## 3. Back up configuration (mode 0600)

Back up configuration, not mnemonics or private keys, before stopping:

```bash
umask 077
install -m 0600 /absolute/lumera/home/config/config.toml \
  "$EVIDENCE_DIR/config.toml.before"
install -m 0600 /absolute/lumera/home/config/app.toml \
  "$EVIDENCE_DIR/app.toml.before"
stat -c '%a %U:%G %n' "$EVIDENCE_DIR"/*.before
sha256sum "$EVIDENCE_DIR"/*.before > "$EVIDENCE_DIR/config-before.sha256"
```

For a supernode or Hermes relayer, use the discovered config path in place of these examples. Review copies before sharing: endpoints and topology may be sensitive. Never copy keyring contents into this evidence directory.

## 4. Pre-stage and prove the destination before downtime

**Required PR-2 compatibility dependency:** use only the release's approved destination pre-stage operation that implements the PR-2 no-echo contract. That operation must read the mnemonic from a hidden TTY or protected input file descriptor, never from argv, never echo it, never enable shell tracing, and print only non-secret key metadata. This PR-3 runbook does not claim that an unfinished PR-2 command exists in the current binary; if the release compatibility manifest does not name and hash an implementation of this contract, stop.

After PR-2 pre-staging, verify the destination locally using the same binary/home/backend/location:

```bash
DEST_JSON=$(
  sudo -u <service-user> "$LUMERAD" keys show <destination-key-name> \
    --output json \
    --home /absolute/lumera/home \
    --keyring-backend <backend> \
    --keyring-dir /absolute/keyring/location
)
printf '%s\n' "$DEST_JSON" | jq '{name,address,type:(.type // .pubkey."@type" // .pubkey.type_url)}'
DEST_ADDR=$(printf '%s\n' "$DEST_JSON" | jq -er '.address')
```

The destination must be coin type 60 / `eth_secp256k1`, controlled and recoverable by the operator, and fresh on-chain. Run the helper dry-run and retain its public-address output. It checks key types, migration indexes, destination account freshness, and the migration estimate. A destination mismatch or unknown key type is a hard stop.

## 5. Stop and prove stopped

### systemd

```bash
sudo systemctl stop lumerad
sudo systemctl is-active --quiet lumerad && { echo 'lumerad still active' >&2; exit 1; } || true
sudo systemctl show lumerad -p ActiveState -p SubState -p MainPID
pgrep -a -u <service-user> -f '(^|/)lumerad( |$)' && { echo 'lumerad process remains' >&2; exit 1; } || true
```

### Docker

```bash
docker stop --time 60 <container>
test "$(docker inspect -f '{{.State.Running}}' <container>)" = false
docker inspect -f '{{.State.Status}} {{.State.ExitCode}} {{.State.FinishedAt}}' <container>
```

Disable or account for an external restart policy before manual replacement; do not start a second process with the same consensus key.

### Kubernetes

```bash
kubectl -n <namespace> scale statefulset/<name> --replicas=0
kubectl -n <namespace> wait --for=delete pod/<pod> --timeout=120s
! kubectl -n <namespace> get pod -l app=<label> --no-headers 2>/dev/null | grep -q .
```

Record the original replica count. If an operator manages replicas, pause reconciliation first. Do not continue while any pod with the validator consensus key is running.

## 6. Re-run dry-run, verify destination, broadcast once

Repeat the same dry-run after stopping. Compare its displayed absolute binary path/version/SHA-256 and separate keyring backend/location provenance to the approved manifest. Verify the printed legacy and destination public addresses and destination key type one final time.

The irreversible boundary is the first live helper invocation. Choose the branch that matches the discovered supervisor and run it exactly once. The paths, image digest, service identity, mounts, binary/helper hashes, keyring, and RPC must match the approved manifest and the successful post-stop dry-run.

### systemd / secured operator host

```bash
sudo -u <service-user> ./scripts/migrate-validator.sh \
  <legacy-key-name> <destination-key-name> \
  --binary "$LUMERAD" \
  --home /absolute/lumera/home \
  --keyring-backend <backend> \
  --keyring-dir /absolute/keyring/location \
  --chain-id <chain-id> \
  --node <trusted-rpc> \
  --i-have-stopped-the-node
```

### Docker one-shot container

Do not apply the systemd command to a container deployment. Keep the original workload container stopped and run a one-shot container from the manifest's exact `artifacts.container_image.artifact@artifacts.container_image.digest`, as the same numeric UID/GID, with the stopped container's volumes. `--volumes-from` preserves every original mount destination; it does not remap mounts. Therefore set `CONTAINER_HOME` and `CONTAINER_KEYRING_DIR` to the exact original **in-container destination paths** recorded by `docker inspect`, not invented `/mounted/...` paths. The image must contain `artifacts.bound_files["scripts/migrate-validator.sh"]` and `artifacts.chain_executable` at their manifest-pinned `release_path` values; verify both hashes before use.

```bash
# The workload container remains stopped for both invocations.
test "$(docker inspect -f '{{.State.Running}}' <stopped-container>)" = false
APPROVED_IMAGE='<artifacts.container_image.artifact>@<artifacts.container_image.digest>'
SERVICE_UID_GID='<uid>:<gid>'
MIGRATION_HELPER='<manifest-pinned-helper-release_path-in-image>'
LUMERAD_IN_IMAGE='<artifacts.chain_executable.release_path-in-image>'
CONTAINER_HOME='<original-home-mount-destination-from-docker-inspect>'
CONTAINER_KEYRING_DIR='<original-keyring-mount-destination-from-docker-inspect>'

run_migration() {
  docker run --rm -it --name evmigration-once \
    --user "$SERVICE_UID_GID" --network host \
    --volumes-from <stopped-container>:rw \
    --entrypoint "$MIGRATION_HELPER" \
    "$APPROVED_IMAGE" \
    <legacy-key-name> <destination-key-name> \
    --binary "$LUMERAD_IN_IMAGE" \
    --home "$CONTAINER_HOME" \
    --keyring-backend <backend> \
    --keyring-dir "$CONTAINER_KEYRING_DIR" \
    --chain-id <chain-id> --node <trusted-rpc> \
    --i-have-stopped-the-node "$@"
}

# These invoke the exact same reviewed command; only --dry-run differs.
run_migration --dry-run
# After approving the dry-run transcript, invoke the live command ONCE.
run_migration
```

`--network host` is an explicit example for a trusted operator-controlled host. If policy forbids it, attach the one-shot container to an approved network that can reach `<trusted-rpc>`; do not target the stopped container or an untrusted public endpoint.

### Kubernetes one-shot pod

Keep the StatefulSet at zero replicas. Apply a dedicated Pod using the workload's exact `serviceAccountName`, `runAsUser`/`runAsGroup`, PVC, config/keyring mount paths, manifest-pinned image digest, helper, binary, and trusted RPC. Do not mount a validator PVC read-write into any other pod concurrently.

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: evmigration-once
  namespace: <namespace>
spec:
  restartPolicy: Never
  serviceAccountName: <same-service-account-as-stopped-workload>
  securityContext:
    runAsUser: <same-runAsUser>
    runAsGroup: <same-runAsGroup>
    fsGroup: <same-fsGroup>
  containers:
    - name: migrate
      image: <registry>/<image>@sha256:<manifest-image-digest>
      imagePullPolicy: IfNotPresent
      command: ["/release/scripts/migrate-validator.sh"]
      args:
        - <legacy-key-name>
        - <destination-key-name>
        - --binary
        - /release/bin/lumerad
        - --home
        - /mounted/lumera/home
        - --keyring-backend
        - <backend>
        - --keyring-dir
        - /mounted/keyring/location
        - --chain-id
        - <chain-id>
        - --node
        - <trusted-rpc>
        - --i-have-stopped-the-node
        - --dry-run
      stdin: true
      tty: true
      volumeMounts:
        - {name: workload-data, mountPath: /mounted/lumera/home}
        - {name: workload-keyring, mountPath: /mounted/keyring/location}
  volumes:
    - name: workload-data
      persistentVolumeClaim: {claimName: <same-workload-data-pvc>}
    - name: workload-keyring
      persistentVolumeClaim: {claimName: <same-keyring-pvc>}
```

```bash
kubectl -n <namespace> apply -f evmigration-once.yaml
kubectl -n <namespace> attach -it pod/evmigration-once
kubectl -n <namespace> wait --for=jsonpath='{.status.phase}'=Succeeded pod/evmigration-once --timeout=10m
kubectl -n <namespace> logs pod/evmigration-once > "$EVIDENCE_DIR/kubernetes-dry-run.txt"
kubectl -n <namespace> delete pod/evmigration-once --wait=true
```

Approve the dry-run transcript, remove only the final `--dry-run` item from the reviewed YAML, and repeat `apply`/`attach`/`wait` exactly once for the live Pod. Save its logs, then delete it before restoring the StatefulSet. If the keyring is not on a dedicated PVC, reproduce the workload's exact Secret/CSI mount instead; never copy mnemonic or private-key material into a ConfigMap or command argument.

For non-validator accounts, substitute the manifest-pinned `migrate-account.sh` and omit `--i-have-stopped-the-node`. Use a manifest-pinned batch/multisig entry point only when the matching method guide explicitly requires it; never improvise an unbound entry point.

Do not put a mnemonic in argv. Do not use `--yes` until the reviewed dry-run transcript and destination proof are approved. Capture the tx hash/public addresses, but redact terminal output before sharing.

## 7. Retry boundary: query before any retry

Before broadcast, failures are reversible: fix binary/config/keyring/RPC inputs and repeat dry-run. After a tx hash is returned, or after an ambiguous timeout/connection loss during broadcast, **do not rebroadcast**. Query public state first:

```bash
"$LUMERAD" query tx <tx-hash> --node <trusted-rpc> --output json
"$LUMERAD" query evmigration migration-record <legacy-address> \
  --node <trusted-rpc> --output json
```

A migration record is authoritative and irreversible; finalize forward. If neither query proves inclusion, preserve the full redacted error and escalate before another broadcast. A deterministic pre-CheckTx rejection with no tx hash and no migration record may be corrected and retried.

## 8. Finalize, restart, and verify

Apply the account-specific local finalization described in the validator, supernode, or relayer guide. Then restart exactly one supervised instance.

```bash
# systemd
sudo systemctl start lumerad
sudo systemctl is-active --quiet lumerad
sudo journalctl -u lumerad --since '10 minutes ago' --no-pager | tail -n 200

# Docker alternative
docker start <container>
docker inspect -f '{{.State.Running}} {{.State.Status}}' <container>
docker logs --since 10m <container> 2>&1 | tail -n 200

# Kubernetes alternative
kubectl -n <namespace> scale statefulset/<name> --replicas=<original-count>
kubectl -n <namespace> rollout status statefulset/<name> --timeout=5m
kubectl -n <namespace> logs statefulset/<name> --since=10m --all-containers=true | tail -n 200
```

Use public addresses for chain checks; no keyring unlock is required:

```bash
"$LUMERAD" query evmigration migration-record <legacy-address> --node <trusted-rpc> --output json
"$LUMERAD" query bank balances <destination-address> --node <trusted-rpc> --output json
"$LUMERAD" status --node <trusted-rpc> | jq '.sync_info | {catching_up,latest_block_height}'
```

For supernodes, use the release-matched authenticated `sncli` client. The SuperNode registers `grpc.health.v1.Health` on its authenticated transport; a generic plaintext `grpcurl` probe does not establish that transport and is not an acceptance check. `sncli` requires a TOML config even when the remote address and endpoint are overridden on the command line. Prepare it under the discovered secured operator/supernode base directory (mode `0600`) with:

- `[lumera] grpc_addr` set to a trusted Lumera chain gRPC endpoint and the exact `chain_id`;
- `[keyring] backend`, `dir`, and `key_name` set to the same post-migration keyring identity; `local_address` must equal that key's migrated address;
- no plaintext passphrase; if needed, use a mode-`0600` `passphrase_file` supplied by the platform secret mechanism;
- `[supernode] address` and `grpc_endpoint` set to the migrated SuperNode identity and its authenticated gRPC endpoint.

This check has the same exact PR-2 compatibility dependency as destination prestaging: the manifest-pinned no-echo implementation must have placed the destination key in this keyring, and the SuperNode config must already reference that migrated identity. Otherwise stop; do not work around the gate by using unauthenticated `grpcurl` or HTTP.

```bash
umask 077
SNCLI=/absolute/path/to/manifest-pinned/sncli
SNCLI_CONFIG=/absolute/supernode/basedir/sncli-config.toml
chmod 0600 "$SNCLI_CONFIG"
# If passphrase_file is configured, its existing platform-secret mount must also be mode 0600:
chmod 0600 /absolute/secret/mount/sncli-keyring-passphrase
sha256sum "$SNCLI" | tee "$EVIDENCE_DIR/sncli.sha256"

sudo -u <supernode-service-user> "$SNCLI" \
  --config "$SNCLI_CONFIG" \
  --address <migrated-supernode-lumera-address> \
  --grpc_endpoint <supernode-grpc-host:port> \
  health-check
```

Require exit code 0 and the exact authenticated output `✅ Health status: SERVING`. Then inspect service logs for key/backend/decode/permission failures and verify the account-specific chain record. Restore any temporary RPC timeout only after verification.

## 9. Portal no-code gate (live browser evidence required)

Portal source inspection is not acceptance evidence. Before release approval, attach a sanitized live-browser evidence bundle from the exact Portal build and target chain:

- Portal commit/build identifier and chain profile are visible.
- Review screen visibly shows legacy address, destination Lumera address, Ethereum hex address, and account type before signing.
- Destination freshness succeeds; an intentionally occupied destination is blocked before signing/broadcast.
- Validator flow visibly requires maintenance planned, node stopped, and commands copied.
- Sign/confirm screen repeats From/To and the irreversible warning before the enabled migrate action.
- Network trace or explorer evidence proves only one broadcast occurred.
- Success screen shows tx hash and the on-chain migration record; refresh preserves the migrated state.
- Failure evidence covers rejected signing, destination preflight failure, and ambiguous broadcast without leaking secrets.
- Screenshots, HAR, and console logs are redacted: no mnemonic, private key, passphrase, auth header, cookie, bearer token, or full environment dump.

The current Portal source derives and displays the destination and runs destination preflight before proof signing and again before broadcast. Therefore no fourth-PR product blocker was proven by source inspection, but the live-browser gate above remains mandatory.

## 10. Compatibility manifest and owned-testnet provenance

Use:

- [`../operator-artifacts/compatibility-manifest.schema.json`](../operator-artifacts/compatibility-manifest.schema.json)
- [`../operator-artifacts/compatibility-manifest.template.json`](../operator-artifacts/compatibility-manifest.template.json)
- [`../operator-artifacts/owned-testnet-baseline.template.json`](../operator-artifacts/owned-testnet-baseline.template.json)

The current release workflow publishes a SHA-256 `release_checksum` beside the tarball, and current release commits may be GitHub-signed, but neither authenticates separately downloaded release artifacts. The template therefore fails closed with `approval.status = "blocked"`.

The final approved manifest is an immutable RFC 8785 JCS file. Its detached Sigstore bundle is published as `compatibility-manifest.sigstore.json`; the bundle is **not embedded back into the signed manifest**, so the process is non-circular. Before canonicalization, the release owner must supply the trusted certificate identity and OIDC issuer, record them in `approval.detached_signature`, set the final approval fields, and revalidate the completed draft. Canonicalize and sign those immutable bytes, then verify without modifying either file. Never edit approval or signature metadata after canonicalization/signing. Keyless `cosign sign-blob` is required; do not introduce a long-lived private signing secret.

The schema rejects `approved` while PR-2 prestaging is blocked, any validation is not `pass`, any required binary/helper/guide identity is a placeholder or zero hash, release-owner approval is false, or trusted signer identity/issuer is missing/placeholder. Schema acceptance does not prove a signature cryptographically: approval is effective only after the exact detached verification command succeeds over the published canonical manifest bytes.
