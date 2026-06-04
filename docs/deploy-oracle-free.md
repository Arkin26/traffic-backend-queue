# Deploy on Oracle Cloud Always Free (step-by-step)

Target: one **Ampere A1** VM running `make up-prod` for a handful of testers at **$0/month** (Always Free only).

## Step 1 — Oracle account

1. Go to https://www.oracle.com/cloud/free/
2. Click **Start for free** and complete signup (email + password).
3. Choose your **Home Region** carefully — Always Free resources only exist in that region forever. Pick one close to your testers (e.g. `uk-london-1`, `eu-frankfurt-1`, `us-ashburn-1`).
4. Complete identity verification (card may be required; you are not charged if you stay in Always Free).

**Stop here and confirm Step 1 is done before Step 2.**

---

## Step 2 — Create the VM (Compute instance)

1. Sign in to **Oracle Cloud Console**: https://cloud.oracle.com/
2. Menu ☰ → **Compute** → **Instances** → **Create instance**.
3. **Name:** `traffic-demo`
4. **Placement:** keep default AD in your home region.
5. **Image:** Ubuntu 22.04 (or 24.04) — **Ampere** shape only for Always Free.
6. **Shape:** Click **Change shape** → **Ampere** → **VM.Standard.A1.Flex**
   - **OCPUs:** `2` (or `4` if you have quota)
   - **Memory (GB):** `12` (or up to 24 if quota allows; minimum for Always Free flex is fine at 6–12 GB)
7. **Networking:** Select your compartment’s **virtual cloud network** (create one if prompted — defaults are OK).
8. **Public IP:** Assign a **public IPv4 address**.
9. **SSH keys:** Choose **Generate a key pair for me**, download:
   - `ssh-key-YYYY-MM-DD.key` (private)
   - `ssh-key-YYYY-MM-DD.key.pub` (public)
   Store the private key somewhere safe (e.g. `~/.ssh/oracle-traffic.key`).
10. **Boot volume:** default 50 GB is fine (Always Free includes boot volume limits).
11. Click **Create**. Wait until state is **Running**. Note the **Public IP address**.

**Stop here and confirm Step 2 is done (you have the public IP + SSH key).**

---

## Step 3 — Open ports (firewall)

### A. Oracle “Security List” (cloud firewall)

1. On the instance page, click your **Subnet** link → **Security List** (default).
2. **Add Ingress Rules:**
   - Source `0.0.0.0/0`, TCP, port **22** (SSH)
   - Source `0.0.0.0/0`, TCP, port **8080** (API/UI) — optional if using Cloudflare Tunnel later
   - Or ports **80** and **443** if you will use Caddy/nginx on the VM

### B. Ubuntu firewall on the VM (after first SSH)

We will run in Step 5:

```bash
sudo iptables -I INPUT 6 -m state --state NEW -p tcp --dport 8080 -j ACCEPT
sudo netfilter-persistent save
```

**Stop after Step 3 when ingress rules are added.**

---

## Step 4 — First SSH login

From your Mac (replace paths and IP):

```bash
chmod 600 ~/.ssh/oracle-traffic.key
ssh -i ~/.ssh/oracle-traffic.key ubuntu@YOUR_PUBLIC_IP
```

If Ubuntu image user is `ubuntu`, use that. Some images use `opc` — check the instance **Console connection** hint on Oracle.

**Stop when you have a shell on the server.**

---

## Step 5 — Install Docker on the VM

Run on the server:

```bash
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker ubuntu
exit
```

SSH in again so the `docker` group applies:

```bash
ssh -i ~/.ssh/oracle-traffic.key ubuntu@YOUR_PUBLIC_IP
docker ps
```

---

## Step 6 — Clone Traffic and configure secrets

```bash
sudo apt-get update && sudo apt-get install -y git
git clone https://github.com/YOUR_USER/traffic.git
cd traffic
cp deploy/.env.example deploy/.env
nano deploy/.env
```

Set (generate on your Mac with `openssl rand -hex 32`):

- `POSTGRES_PASSWORD`
- `JWT_SECRET`
- `ADMIN_API_KEY`
- `DEMO_API_KEY` (share with testers)

Keep:

```env
HTTP_PORT=8080
LOADTEST_MAX_VUS=150
LOADTEST_MAX_CONCURRENT=1
```

---

## Step 7 — Start the stack

```bash
cd ~/traffic
docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env up -d --build
```

Wait ~2–5 minutes, then:

```bash
curl -s http://localhost:8080/health/ready
curl -s http://localhost:8080/v1/demo/config
docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env ps
```

Open in browser: `http://YOUR_PUBLIC_IP:8080`

---

## Step 8 — HTTPS (optional, still free)

**Cloudflare Tunnel** (no open port 443 needed on Oracle):

1. Add your domain to Cloudflare (free plan).
2. On the VM: install `cloudflared`, create a tunnel to `http://localhost:8080`.
3. Share `https://traffic.yourdomain.com` with testers.

---

## Billing safety checklist

- [ ] Instance shape is **VM.Standard.A1.Flex** (Ampere), not x86 paid shapes
- [ ] No extra block volumes beyond Always Free allowance
- [ ] No Load Balancer, NAT Gateway, or Autonomous DB unless you intend paid services
- [ ] Set **Budget** alert in Oracle: Billing → Budgets → $0.01 alert (optional)

---

## Troubleshooting

| Issue | Fix |
|-------|-----|
| “Out of host capacity” for A1 | Try another AD or retry later; common on Oracle Free |
| Cannot SSH | Check security list port 22; instance Running |
| `docker compose` not found | `sudo apt install docker-compose-plugin` or use `docker compose` v2 from docker.com script |
| Load test queued forever | `docker ps` must show `loadtest-runner` running |
