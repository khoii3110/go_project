# Go Microservices Starter (Beginner Friendly)

This project contains 3 Go microservices and shared infrastructure via Docker Compose:

- auth-service
- team-service
- asset-service

Infrastructure managed by Docker Compose:

- PostgreSQL
- Redis

## 1) Install Go

Pick your OS and run the commands exactly as shown.

### Windows (PowerShell)

1. Install Go:

```powershell
winget install -e --id GoLang.Go
```

2. Restart terminal, then verify:

```powershell
go version
```

### macOS (Terminal)

1. Install Homebrew (if you do not have it):

```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
```

2. Install Go:

```bash
brew update
brew install go
```

3. Verify:

```bash
go version
```

### Linux (Ubuntu/Debian example)

1. Download Go archive (replace version if newer exists):

```bash
cd /tmp
curl -LO https://go.dev/dl/go1.22.5.linux-amd64.tar.gz
```

2. Remove any old Go install and install new one:

```bash
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.22.5.linux-amd64.tar.gz
```

3. Add Go to PATH:

```bash
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

4. Verify:

```bash
go version
```

## 2) Install Docker (for Postgres and Redis)

Install Docker Desktop (Windows/macOS) or Docker Engine + Docker Compose plugin (Linux), then verify:

```bash
docker --version
docker compose version
```

## 3) Run this project

From the project root (the folder containing docker-compose.yml):

1. Build and start everything:

```bash
docker compose up --build -d
```

2. Check running containers:

```bash
docker compose ps
```

3. Check logs:

```bash
docker compose logs -f
```

4. Test each service in a new terminal:

```bash
curl http://localhost:8081/healthz
curl http://localhost:8082/healthz
curl http://localhost:8083/healthz
```

5. Stop everything:

```bash
docker compose down
```

6. Stop and remove volumes too (deletes DB data):

```bash
docker compose down -v
```

## Project Structure

```text
.
|- auth-service/
|  |- cmd/server/main.go
|  |- go.mod
|  `- Dockerfile
|- team-service/
|  |- cmd/server/main.go
|  |- go.mod
|  `- Dockerfile
|- asset-service/
|  |- cmd/server/main.go
|  |- go.mod
|  `- Dockerfile
|- docker/postgres/init.sql
`- docker-compose.yml
```