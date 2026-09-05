# pdfingest setup — running Docling yourself

The skill needs a Docling somewhere. Everything below is reproducible with no
access to any particular deployment — pick the rung that fits. All rungs talk
to the same client (`plugins/assay/scripts/pdfingest.sh`); only
`ASSAY_DOCLING_URL` (or the local-CLI fallback) differs.

## 1. Local install (zero infra — the default)

```bash
# Python 3.10+ (3.9 was dropped in docling v2.70)
python3 --version
pip install docling            # or: pipx install docling / uv tool install docling

# First conversion downloads the models (~2 GB, once) into the local cache;
# after that everything runs offline.
docling paper.pdf --to md      # writes paper.md next to a local out dir
docling --version
```

With no `ASSAY_DOCLING_URL` endpoint reachable, `pdfingest.sh` falls back to
this CLI automatically — nothing further to configure.

For reproducibility, pin what you install (`pip install docling==<version>`)
and note the version alongside your extraction, like any other tool.

## 2. Docker (one container, sync HTTP API)

```bash
docker run -d --name docling-serve -p 5001:5001 \
  quay.io/docling-project/docling-serve-cpu:<pinned-tag>   # ~4.4 GB image
# verify — the FastAPI schema page should return 200:
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:5001/docs

export ASSAY_DOCLING_URL=http://localhost:5001
pdfingest.sh --health
pdfingest.sh paper.pdf > paper.md
```

Pin an explicit version tag (never `:latest`) — e.g. `v1.18.0` at the time of
writing; check the [releases](https://github.com/docling-project/docling-serve/releases)
for the current one. The `-cpu` image is CPU-only torch; CUDA variants exist
(`-cu128`/`-cu130`) if you have a GPU and care about throughput — occasional
document work does not need one. Models ship inside the image; the container
needs no network access after the pull.

## 3. Plain Kubernetes (always-on, no autoscaler)

One replica is ample for personal or small-team use. Save as
`docling-serve.yaml` and `kubectl apply -f docling-serve.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: docling-serve
spec:
  replicas: 1
  selector:
    matchLabels: { app: docling-serve }
  template:
    metadata:
      labels: { app: docling-serve }
    spec:
      containers:
        - name: docling-serve
          image: quay.io/docling-project/docling-serve-cpu:<pinned-tag>
          ports: [{ containerPort: 5001 }]
          env:
            - name: DOCLING_SERVE_ENABLE_UI
              value: "0"            # API only; the UI is an unauthenticated upload surface
          resources:
            requests: { cpu: 500m, memory: 2Gi }
            limits:   { cpu: "2",  memory: 6Gi }
---
apiVersion: v1
kind: Service
metadata:
  name: docling-serve
spec:
  selector: { app: docling-serve }
  ports: [{ port: 5001, targetPort: 5001 }]
```

Reach it from your machine with
`kubectl port-forward svc/docling-serve 5001:5001`, then set
`ASSAY_DOCLING_URL=http://localhost:5001`. Because it is an unauthenticated
file-upload endpoint, keep it ClusterIP — do not expose it on a public
ingress — and fence its egress to nothing (the image is self-contained).

## 4. Scale-to-zero (optional — only if you already run KEDA)

If your cluster runs [KEDA](https://keda.dev) + the
[KEDA HTTP add-on](https://github.com/kedacore/http-add-on), the same
Deployment from rung 3 can idle at zero replicas and wake per request via an
`HTTPScaledObject` (`min: 0`, a small `max`, a few-minute cooldown). Clients
then point `ASSAY_DOCLING_URL` at the add-on's interceptor proxy rather than
the docling Service, and a cold start after idle legitimately takes tens of
seconds to minutes — use a generous timeout, don't retry-storm. This rung is
strictly optional; the skill works identically without it.

## Confidentiality, once for all rungs

The document body crosses the endpoint's wire WHOLE — point
`ASSAY_DOCLING_URL` only at an endpoint you or your operator control, never a
third-party API for confidential material. All the rungs above keep everything
on your machines.
