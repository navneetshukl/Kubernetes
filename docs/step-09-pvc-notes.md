# Step 9 — PersistentVolume & PersistentVolumeClaim (PVC)

## Goal
Understand persistent storage in Kubernetes by giving PostgreSQL a PVC that survives pod deletion.

---

## Architecture Overview
```
Pod (PostgreSQL) → PVC (postgres-pvc) → PV (postgres-pv) → hostPath: /data/postgres
```

---

## Files Created/Modified

### 1. `k8s/postgres-pvc.yaml` — PV + PVC with Selector Binding
```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: postgres-pv
  labels:
    type: local
    app: postgres
spec:
  capacity:
    storage: 1Gi
  accessModes:
    - ReadWriteOnce
  persistentVolumeReclaimPolicy: Retain
  storageClassName: standard
  hostPath:
    path: /data/postgres
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: postgres-pvc
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: standard
  selector:
    matchLabels:
      type: local
      app: postgres
```

### 2. `k8s/postgres-deployment.yaml` — Updated to Use PVC
```yaml
volumes:
- name: postgres-data
  persistentVolumeClaim:
    claimName: postgres-pvc
```
---

## Key Concepts

### PersistentVolume (PV)
- Cluster-level resource (not namespaced)
- Represents actual storage (disk, NFS, cloud volume, hostPath)
- Created by admin or dynamically provisioned

### PersistentVolumeClaim (PVC)
- Namespaced resource (request for storage by a pod)
- Pod mounts PVC, not PV directly
- Can use `selector` to bind to specific PV

### Binding Process (Static Provisioning)
1. PVC created
2. K8s searches for UNBOUND PV matching:
   - capacity ≥ request (1Gi ≥ 1Gi)
   - accessModes compatible (RWO)
   - storageClassName match (standard)
   - selector.matchLabels match PV labels
3. PV found → BIND (1:1 exclusive)
4. PVC.status=Bound, PV.status=Bound

### Selector-Based Binding
| Component | Field | Purpose |
|-----------|-------|---------|
| PV | metadata.labels | Identifies PV for selection |
| PVC | spec.selector.matchLabels | Filters which PV to bind |

> **Why selectors?** Prevents wrong PV binding, enables env-specific PVs

### Reclaim Policies
- **Retain**: PV kept, data preserved (our choice)
- **Delete**: PV + storage deleted
- **Recycle**: Deprecated
---

## Access Modes
- **RWO** (ReadWriteOnce): Single node read/write — PostgreSQL needs this
- **ROX** (ReadOnlyMany): Multi-node read-only
- **RWX** (ReadWriteMany): Multi-node read/write

---

## StorageClass
`storageClassName: standard`
- Dynamic: PVC omits selector → StorageClass auto-provisions
- Static: PV pre-created + selector (our case)

---

## hostPath PV
```yaml
hostPath:
  path: /data/postgres
```
- Local node filesystem only
- Dev/testing only — not production for multi-node
- Pod reschedule to different node = data unavailable

---

## Verification Commands
```bash
kubectl apply -f k8s/postgres-pvc.yaml
kubectl apply -f k8s/postgres-deployment.yaml
kubectl get pvc postgres-pvc
kubectl get pv postgres-pv
kubectl describe pvc postgres-pvc
kubectl describe pv postgres-pv
```

### Expected Output
```
$ kubectl get pvc postgres-pvc
NAME         STATUS   VOLUME       CAPACITY   ACCESS MODES   STORAGECLASS   AGE
postgres-pvc Bound    postgres-pv  1Gi        RWO            standard       30s

$ kubectl get pv postgres-pv
NAME         CAPACITY   ACCESS MODES   RECLAIM POLICY   STATUS   CLAIM                  STORAGECLASS   AGE
postgres-pv  1Gi        RWO            Retain           Bound    default/postgres-pvc   standard       30s
```

---

## Data Persistence Test
```bash
# 1. Connect to PostgreSQL
kubectl exec -it deploy/postgres-deployment -- psql -U postgres -d notifications

# 2. Insert test data
INSERT INTO notifications (title, message) VALUES ('Notification 1', 'Test message 1');
INSERT INTO notifications (title, message) VALUES ('Notification 2', 'Test message 2');

# 3. Verify data
SELECT * FROM notifications;

# 4. Delete PostgreSQL pod
kubectl delete pod -l app=postgres

# 5. Verify data persists
kubectl exec -it deploy/postgres-deployment -- psql -U postgres -d notifications -c "SELECT * FROM notifications;"
```

---

## Important Points Summary
| # | Concept | Key Takeaway |
|---|---------|--------------|
| 1 | PV vs PVC | PV = storage resource, PVC = request |
| 2 | Binding | 1:1 exclusive, matches capacity, accessModes, storageClass, labels |
| 3 | Selector | PVC.selector.matchLabels → PV.metadata.labels |
| 4 | Static vs Dynamic | Static: PV pre-created. Dynamic: StorageClass auto-provisions |
| 5 | emptyDir vs PVC | emptyDir = ephemeral, PVC = persistent |
| 6 | ReclaimPolicy | Retain = keep data, Delete = auto-cleanup |
| 7 | hostPath | Local node storage, not production-ready |
| 8 | Pod spec | Pod references PVC via claimName |

---

## Common Pitfalls
| Pitfall | Solution |
|---------|----------|
| PVC stuck Pending | Check PV labels, capacity, storageClass match |
| Pod fails to mount | Verify PVC Bound, hostPath dir exists on node |
| Data lost after delete | Use PVC not emptyDir, ReclaimPolicy=Retain |
| Multiple PVCs bind same PV | Unique labels + selectors per PV/PVC |
| Pod reschedules, data gone | hostPath is node-local — use network storage |

---

## Next Steps
- Step 10: Dynamic provisioning with StorageClass
- Step 11: StatefulSet for PostgreSQL
- Step 12: Backup/restore with Velero or pg_dump

---

## References
- [Kubernetes Persistent Volumes](https://kubernetes.io/docs/concepts/storage/persistent-volumes/)
- [PV/PVC Binding](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#binding)
- [StorageClasses](https://kubernetes.io/docs/concepts/storage/storage-classes/)
- [hostPath](https://kubernetes.io/docs/concepts/storage/volumes/#hostpath)