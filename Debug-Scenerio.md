# Kubernetes Debugging Question Bank
*Scenario-first prep, built around your `task-api` / `worker` / `sidecar-app` / `db-migration` repo*

How to use this: don't jump to the answer. Read the scenario, say out loud (or write) what you'd check first, second, third — then compare against the diagnostic path below. Interviewers are grading your *order of investigation* as much as your final answer.

---

## Section 1 — Pod Lifecycle Failures

### 1.1 `task-api` pod stuck in `CrashLoopBackOff`
Your `task-api-deployment` pod keeps restarting. `kubectl get pods` shows `CrashLoopBackOff`, restart count climbing.

**Diagnostic path:**
1. `kubectl describe pod <pod>` — check `Last State`, `Reason`, `Exit Code`.
   - Exit code `0` = clean exit, container just isn't a long-running process (unlikely here — Gin should block on `.Run()`).
   - Exit code `1` = app-level panic/error.
   - Exit code `137` = OOMKilled (SIGKILL).
   - Exit code `143` = SIGTERM, graceful-ish shutdown.
2. `kubectl logs <pod> --previous` — logs from the crashed instance, not the current restart attempt.
3. In your case specifically: `task-api`'s `main()` calls `connectToDb(dsn)` and does `log.Println(err); return` on failure — it doesn't `os.Exit(1)`, so ask yourself: does the container actually exit, or does `main()` return normally and the process ends with code 0, causing Kubernetes to still restart it because it's not a "Completed" job but a Deployment expecting a running process?
4. Check the `DSN` secret is actually populated (`kubectl get secret db-secret -o yaml` and decode) — a bad DSN is the #1 suspect here.
5. Check whether `db-migration` init container even completed — if `task-api` starts before the `tasks` table exists, does it fail? (It won't fail on missing table until a query runs — so first request would 500, not crash. Good follow-up: "would this actually crash the pod, or just serve errors?")

**What they're really testing:** do you distinguish between "container process died" vs "app returned unhealthy responses" — these need completely different fixes.

---

### 1.2 `task-worker` pod stuck in `Init:0/1` or `Init:CrashLoopBackOff`
Your worker pod (which has an init container in the API deployment pattern — imagine you added `db-migration` as an init container to `worker` too) never reaches `Running`.

**Diagnostic path:**
1. `kubectl describe pod <pod>` → `Init Containers` section shows per-container status.
2. `kubectl logs <pod> -c db-migration` — init container logs are fetched by name, not by default.
3. Your `db-migration/main.go` retries 20 times with 2s sleep = ~40s max before `log.Fatalf`. If Postgres isn't reachable in that window, the init container exits non-zero and Kubernetes retries the **whole pod's init phase** with backoff.
4. Root causes to check in order: (a) is `postgres-deployment` even Running? (b) is `postgres-service` selector (`app: postgres-pod`) matching the pod's labels? (c) is the DSN's port (`5433`) actually the **Service** port, not the container port (`5432`)? — this is a real subtlety in your own manifests worth being able to explain cold.

**Follow-up they'll ask:** "Why can an init container 'succeed' 20 retries later but the pod still never becomes Ready?" → Because init containers run to completion sequentially before app containers start; if it eventually exits 0, the pod proceeds. The real trap is a **permanent** misconfig (wrong port, wrong secret key) where retries never help — you should know when to stop debugging "is it slow" and start debugging "is it wrong."

---

### 1.3 `ImagePullBackOff` on any of your four images
`kubectl get pods` shows `ErrImagePull` → `ImagePullBackOff`.

**Diagnostic path:**
1. `kubectl describe pod <pod>` → look at the `Events` section for the exact pull error string (auth failure vs. image not found vs. registry unreachable).
2. In your manifests, every image uses `imagePullPolicy: Never` — meaning Kubernetes will **never** attempt to pull from a registry; it only looks at the local node's image cache (e.g., the Minikube VM's Docker daemon).
3. Classic trap: you `docker build` on your host machine, but Minikube runs its own Docker daemon inside a VM — the image exists on your host but not where the kubelet is looking. Fix: `eval $(minikube docker-env)` before building, or `minikube image load <image>`.
4. If using Kind instead: `kind load docker-image <image>:<tag> --name <cluster>`.

**What they're testing:** do you understand that `imagePullPolicy: Never` is a deliberate local-dev choice, and what breaks if someone forgets it exists when moving to a real cluster/registry?

---

### 1.4 `OOMKilled`
A pod's `Last State` shows `Reason: OOMKilled`, exit code 137.

**Diagnostic path:**
1. `kubectl describe pod` → confirm `OOMKilled` and check if a memory `limit` was set (if none set, this is a node-level OOM, much worse — the kubelet may evict other pods too).
2. `kubectl top pod` (needs metrics-server) to see actual usage trend before the kill.
3. None of your current manifests set `resources.limits` — this is worth flagging as a known gap (it's already in your README roadmap). Be ready to explain: without limits, a pod can consume unbounded memory on the node and get killed abruptly with no graceful shutdown, versus with limits it gets OOMKilled predictably at a known threshold — which is actually *easier* to debug and capacity-plan for.
4. Fix pattern: set `requests` (scheduling guarantee) and `limits` (hard ceiling) — and explain why setting only `limits` without `requests` is a common misconfiguration (Kubernetes defaults `requests` = `limits` if you only set the latter — costs you scheduling flexibility).

---

## Section 2 — Networking & Service Discovery

### 2.1 `worker` can't reach `task-api-service`
Worker logs show `NETWORK_ERROR` audit entries — connection refused or timeout to `http://task-api-service:80`.

**Diagnostic path:**
1. `kubectl get endpoints task-api-service` — if empty, the Service's `selector` isn't matching any pod's labels. This is the single most common "Service doesn't work" root cause.
2. In your manifests: `task-api-service` selects `app: task-api-pod`; `task-api-deployment`'s pod template must have that exact label. A typo here (`task-api-pod` vs `task-api`) silently breaks everything with zero error messages — Service just has no backends.
3. `kubectl exec` into the worker pod and `curl http://task-api-service:80/task/get` directly to isolate DNS+routing from app logic.
4. Check `kubectl exec <pod> -- nslookup task-api-service` — confirms CoreDNS is resolving the Service name at all (rules out cluster DNS being broken).
5. Port mismatch check: Service `port: 80` → `targetPort: 8080`. If someone changes the container's listen port without updating `targetPort`, you get "connection refused" even though DNS resolves fine.

**Follow-up:** "The Service resolves and has endpoints, but requests still time out — now what?" → Check NetworkPolicies (none in your repo currently, but they'd ask), check if the target container's port is actually bound to `0.0.0.0` and not `127.0.0.1` inside the container.

---

### 2.2 Postgres port confusion (specific to your repo)
Your `postgres-service` listens on `5433`, forwarding to `targetPort: 5432` (Postgres's real port). Your `db-secret`'s DSN hardcodes `postgres-service:5433`.

**Question to answer cold:** why does this work, and what would break if someone "fixed" the DSN to use `5432` instead?

**Answer path:** Services proxy `port` → `targetPort`; clients always connect to the **Service's `port`** (5433), never the container's actual port directly, unless they bypass the Service and hit the Pod IP. Changing the DSN to `5432` would fail because nothing is listening on `5433→5432`'s Service abstraction at `:5432` — the Service simply doesn't expose that port. This is a good one to be able to draw on a whiteboard.

---

### 2.3 Intermittent 503s / "passing health checks but 3% of requests fail"
(This exact framing shows up in real interviews — see DataCamp's framing.)

**Diagnostic path:**
1. Check if it's load-balancing-shaped: with multiple replicas, is one specific pod unhealthy in a way that readiness probes don't catch (e.g., DB connection pool exhausted on one replica only)?
2. `kubectl get endpoints` while reproducing — does the failing request correlate with one particular pod IP?
3. Check readiness vs liveness probe design: a probe that only checks "process is up" (e.g., TCP check) won't catch "app is up but DB connection pool is saturated." You'd want a `/healthz` that actually pings the DB dependency.
4. Your current manifests have **no probes at all** — a good self-critique point: "with no readiness probe, Kubernetes sends traffic to a pod the instant its container starts, even before `gin.Run()` has actually bound the port or `connectToDb` succeeded downstream." Explain how you'd add one.

---

## Section 3 — Multi-Container Pods (your sidecar pattern)

### 3.1 Sidecar isn't seeing new log lines
`worker` is clearly running (audit entries appearing via `kubectl exec worker -- cat logs/audit.log`), but `kubectl logs <pod> -c sidecar` shows nothing new.

**Diagnostic path:**
1. First confirm both containers are actually mounting the **same** volume: `kubectl describe pod` → check `Volumes` and `Mounts` sections for both containers reference the same `shared-logs` `emptyDir`, same `mountPath: /app/logs`.
2. Check `sidecar`'s `getLogFilePath()` — it defaults to `logs/audit.log` (relative path) if `LOG_DIR` isn't set, while `worker` explicitly writes to `/app/logs/audit.log` (via `LOG_DIR` env or default `./logs` relative to its own workdir `/app`). If the two containers have different working directories or one doesn't set `LOG_DIR` consistently, they resolve to different absolute paths even though the manifest *looks* like it wires them together.
3. Confirm the sidecar's polling loop path: it does a "wait for file to exist" loop first — if the worker hasn't written yet, or writes to a different path, the sidecar waits forever silently with no error, which is a debugging trap (no crash, no log, just silence).
4. Check `bufio.Reader` + `io.EOF` handling — this is a "tail -f" pattern via polling; if the worker is buffering writes (it isn't here — it calls `file.Sync()` after every write) or truncating the file (log rotation), the sidecar's reader position could go stale.

**What they're testing:** can you reason about shared state between containers in a pod, given they share network/volumes but *not* filesystem-by-default and *not* environment unless independently configured?

---

### 3.2 `emptyDir` — what happens to logs when the pod is rescheduled?
Straight conceptual question they'll pair with the above scenario.

**Answer:** `emptyDir` volumes are tied to the **pod's lifecycle on a given node**, not the container's. If a container in the pod crashes and restarts, the volume persists. If the *entire pod* is deleted/rescheduled (node failure, eviction, rolling update), the `emptyDir` — and your audit log — is gone. Good follow-up to raise yourself: "this is why for anything you actually need to keep, you'd want a `PersistentVolumeClaim` or ship logs to something external (stdout + a log aggregator, which is actually what the sidecar is already doing right — it's decoupling 'durable log storage' from the app)."

---

### 3.3 Sidecar container never terminates when the pod is deleted, holding up `kubectl delete`
**Diagnostic path:**
1. `kubectl delete pod` sends SIGTERM to all containers, waits `terminationGracePeriodSeconds` (default 30s), then SIGKILLs.
2. Your `sidecar-app`'s main loop selects on `ctx.Done()` (tied to SIGTERM/SIGINT via `signal.NotifyContext`) and does a final flush read before returning — so it *should* exit cleanly. Good self-check: trace through the code and confirm there's no path where it blocks indefinitely (e.g., if `reader.ReadString('\n')` blocks on a named pipe instead of a regular file — not an issue here since it's a real file, but a good "what if" to reason about).
3. If it *did* hang, you'd see the pod stuck in `Terminating` for the full grace period before force-kill — a classic sign to look at `preStop` hooks or whether the app handles SIGTERM at all.

---

## Section 4 — Init Containers & Job-like Behavior

### 4.1 `db-migration` init container succeeds locally but times out in CI/cluster
**Diagnostic path:**
1. Is Postgres actually a separate pod that needs time to become Ready before `task-api`'s pod is even scheduled? Init containers don't wait for *other pods'* readiness automatically — they only guarantee ordering *within their own pod*. Your `db-migration` handles this itself via its own retry loop, which is the correct pattern — but be ready to explain **why** you can't rely on Kubernetes scheduling order between separate Deployments (Postgres pod and task-api pod scheduling is not coordinated by default).
2. Check resource contention — if the cluster/node is under memory/CPU pressure, Postgres itself might be slow to accept connections within your 40s retry window.

**Follow-up they love:** "Why not just use a Kubernetes `Job` instead of an init container for migrations?" → Good answer: init containers guarantee "runs before this specific pod's app container, every time this pod is created" — including on every rolling restart, which is actually what you want for idempotent `CREATE TABLE IF NOT EXISTS` migrations. A `Job` is a good fit for a one-time, cluster-wide migration you run once per deploy, not per-pod-creation; using a Job would need it wired into your CI/CD or Helm hook (e.g. `helm.sh/hook: pre-install`) rather than the Pod spec itself.

---

## Section 5 — Config & Secrets

### 5.1 App reads stale config after you update a ConfigMap
You `kubectl apply -f k8s/postgres-cred.yml` with a new `DB_HOST`, but the running `task-api` pod still connects to the old host.

**Diagnostic path:**
1. Env vars sourced from a ConfigMap/Secret via `valueFrom` are only read **once, at container start**. Updating the ConfigMap does **not** restart or refresh already-running containers.
2. Fix options: manually `kubectl rollout restart deployment task-api-deployment`, or mount the ConfigMap as a **volume** instead of env vars — volume-mounted ConfigMaps *do* get live-updated on the filesystem (with a sync delay), though your app still needs to re-read the file to notice.
3. Good to mention: this is why some setups add a checksum annotation of the ConfigMap into the pod template (common Helm pattern) to force a rollout whenever config changes.

---

### 5.2 Secret value is wrong / pod can't authenticate to Postgres
`task-api` logs show `password authentication failed`.

**Diagnostic path:**
1. `kubectl get secret db-secret -o jsonpath='{.data.DB_PASSWORD}' | base64 -d` — confirm the actual decoded value, don't trust what you *think* you applied.
2. Check whether Postgres's `POSTGRES_PASSWORD` env (set from the same secret) matches what `task-api`'s DSN uses — in your manifests they reference the *same* `db-secret` keys, so a mismatch would mean the Secret itself was edited without restarting the **Postgres** pod (Postgres only sets its password at first init of the data directory — changing the Secret after that does nothing unless you wipe/reinit the DB).
3. This is a great one to explain confidently: Postgres's user/password is baked into its data directory at first boot; a Kubernetes Secret change afterward is cosmetic unless you handle it at the DB level too (e.g., `ALTER USER`).

---

## Section 6 — Rollouts & Scaling

### 6.1 Rolling update to `task-api` causes a burst of 502s
**Diagnostic path:**
1. No `readinessProbe` (true in your current manifests) means new pods are added to the Service's endpoints the instant the container starts, before Gin has actually bound port 8080 or the app has warmed up — resulting in the LB routing requests to a pod that isn't ready yet.
2. No `terminationGracePeriodSeconds` tuning + no `preStop` hook means old pods might get SIGTERM and be removed from iptables/Service endpoints *after* they've already stopped accepting new connections but before in-flight requests finish — or the reverse race, where traffic still arrives briefly after the container starts shutting down.
3. Correct fix to describe: add a `readinessProbe` hitting a real endpoint, and a `preStop` hook with a short sleep to let the endpoint-removal propagate before the container actually stops.

---

### 6.2 Scaling `worker` to 2+ replicas — does anything break?
Conceptual, tests whether you understand your own design's assumptions.

**Answer to reason through:** Each `worker` replica polls the *same* `task-api-service` independently on its own ticker. With no locking/claiming mechanism, two workers could both see the same `Pending` task and both "process" it — you have no `SELECT ... FOR UPDATE`, no status transition to `Processing`, no distributed lock. This is a genuinely good gap to identify yourself in an interview: "the current single-replica design implicitly relies on there being exactly one worker; scaling it requires either DB-level row locking, a message queue (this is a natural fit for RabbitMQ — which and you already use at IndiaMART), or an idempotency/claim pattern." Interviewers love when you spot this before they ask.

---

## Section 7 — Rapid-fire diagnostic command recall

Be able to produce these without hesitating:

| Situation | Command |
|---|---|
| Why is this pod not starting? | `kubectl describe pod <pod>` |
| What did the crashed container print? | `kubectl logs <pod> --previous` |
| Logs from one container in a multi-container pod | `kubectl logs <pod> -c <container>` |
| Init container logs | `kubectl logs <pod> -c <init-container-name>` |
| Is the Service routing anywhere? | `kubectl get endpoints <service>` |
| Live shell to debug from inside | `kubectl exec -it <pod> -c <container> -- sh` |
| Current CPU/memory usage | `kubectl top pod` (needs metrics-server) |
| Why did scheduling fail? | `kubectl describe pod` → `Events`, or `kubectl get events --sort-by=.lastTimestamp` |
| Force a restart to pick up new ConfigMap | `kubectl rollout restart deployment <name>` |
| Roll back a bad deploy | `kubectl rollout undo deployment <name>` |
| Check what image/tag is actually running | `kubectl get pod <pod> -o jsonpath='{.spec.containers[*].image}'` |

---

## How to actually rehearse this

Pick one scenario a day. Set a 3-minute timer. Talk through your diagnostic order out loud before checking the path above — the framework is the skill, not the memorized answer. When you're comfortable, try breaking your own cluster on purpose (delete a Secret key, typo a selector, remove a `targetPort`) and time how fast you find it using only the commands in Section 7.

Want to go deeper on any one section — e.g. actually inject these failures into your Minikube cluster and walk through fixing them live?
