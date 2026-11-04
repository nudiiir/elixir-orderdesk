# The live demo on Kubernetes

The bot linked from the main README runs from `orderdesk-demo.yaml`: Odoo 18, its Postgres, and Order Desk, in
one namespace with nothing exposed. The bot polls Telegram, and the panel answers only requests from inside
its own pod (look at it with `kubectl port-forward`).

```bash
# 1. the Linux binary, on the node (the pod runs it from hostPath /opt/orderdesk-demo)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o orderdesk ./cmd/orderdesk
sudo install -m 755 orderdesk /opt/orderdesk-demo/orderdesk

# 2. the keys, then everything else
kubectl create namespace orderdesk-demo
kubectl -n orderdesk-demo create secret generic orderdesk-keys --from-env-file=keys.env   # META_MODEL_API_KEY, TELEGRAM_BOT_TOKEN
kubectl apply -f deploy/k8s/orderdesk-demo.yaml

# 3. Odoo's database and catalogue (erps/seed), through a port-forward
kubectl -n orderdesk-demo port-forward svc/odoo 8069:8069 &
python erps/seed/odoo_setup.py && python erps/seed/odoo_seed.py erps/seed/seed.json

# 4. connect the desk to Odoo
kubectl -n orderdesk-demo port-forward svc/orderdesk 8610:8610 &
curl -X POST localhost:8610/api/erp/connect -H "Content-Type: application/json" \
  -d '{"kind":"odoo","url":"http://odoo:8069","database":"orderdesk","username":"admin","password":"admin"}'
```

A public demo code (settings `demo_code` and `demo_customer`) links any chat to one demo shop; the hosted demo
caps Meta model spend with `ai_budget_day` and leaves the rule engineer off (`engineer_auto` = 0).

## Opening it to the judges

`public.yaml` gives the panel and Odoo services for your reverse proxy (behind Cloudflare) and adds a nightly cleanup of demo orders older
than three days. Before applying it:

- start Order Desk with `-public-readonly`: visitors may read the panel, every change stays inside the pod;
- start Odoo with `--no-database-list --db-filter=^orderdesk$` (no database manager);
- run `odoo_judge.py <odoo url> admin <new admin password> <judge password>`: a `judge` user that can read
  sales, stock and products and change nothing (the script tries four writes and shows they are refused),
  and a strong admin password; then reconnect the desk with it (`/api/erp/connect`);
- cap the public desk: settings `ai_budget_day`, `rate_per_10min`, `rate_per_day`, `orders_per_day`.
