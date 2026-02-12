# Docker Testing Guide

This guide explains how to test the dotfile agent in a Docker container.

## Prerequisites

- Docker installed (version 20.10+)
- Docker Compose installed (version 1.29+)
- GitHub personal access token
- A Git repository with your dotfiles

## Quick Start

### 1. Set Up Environment Variables

Copy the example environment file and configure it:

```bash
cp .env.example .env
```

Edit `.env` and set your values:

```bash
# Required
GITHUB_TOKEN=ghp_your_token_here
GIT_URL=https://github.com/yourusername/dotfiles.git

# Optional
WEBHOOK_URL=
DOTFILE_MACHINE_ID=test-machine-001
DOTFILE_BROKER_URL=
```

### 2. Build and Run

```bash
# Build the image
docker-compose build

# Start the container
docker-compose up -d

# View logs
docker-compose logs -f
```

### 3. Test the Agent

```bash
# Check if the agent is running
curl http://localhost:3000/sync

# Trigger a manual sync
curl -X POST http://localhost:3000/sync

# Watch sync progress with SSE
curl -N http://localhost:3000/sync
```

## Docker Commands

### Build

```bash
# Build the image
docker-compose build

# Build without cache
docker-compose build --no-cache

# Build specific service
docker build -t dotfile-agent:latest .
```

### Run

```bash
# Start in detached mode
docker-compose up -d

# Start with logs
docker-compose up

# Start and rebuild
docker-compose up -d --build
```

### Manage

```bash
# Stop the container
docker-compose stop

# Stop and remove
docker-compose down

# Stop and remove with volumes
docker-compose down -v

# Restart
docker-compose restart

# View logs
docker-compose logs -f

# View last 100 lines
docker-compose logs --tail=100
```

### Inspect

```bash
# Execute commands in container
docker-compose exec dotfile-agent sh

# Check container status
docker-compose ps

# View resource usage
docker stats dotfile-agent

# Inspect volumes
docker volume ls
docker volume inspect dotfile-agent_dotfiles-data
```

## Testing Scenarios

### Scenario 1: Basic Sync Test

1. Start the container:
   ```bash
   docker-compose up -d
   ```

2. Check initial status:
   ```bash
   curl http://localhost:3000/sync | jq
   ```

3. Trigger sync:
   ```bash
   curl -X POST http://localhost:3000/sync
   ```

4. Verify files were synced:
   ```bash
   docker-compose exec dotfile-agent ls -la /home/dotfile/dotfiles
   ```

### Scenario 2: Configuration Testing

1. Create a test config file:
   ```bash
   cat > test-config.yaml << 'EOF'
   dotfiles:
     - software: bash
       install:
         linux: apt install -y bash
       files:
         - path: .bashrc
           target: home
   EOF
   ```

2. Mount it in docker-compose.yml:
   ```yaml
   volumes:
     - ./test-config.yaml:/home/dotfile/dotfiles/dotfile-config.yaml:ro
   ```

3. Restart and test:
   ```bash
   docker-compose restart
   docker-compose logs -f
   ```

### Scenario 3: Webhook Testing

1. Set webhook URL in `.env`:
   ```bash
   WEBHOOK_URL=https://api.github.com/repos/user/repo/events
   ```

2. Restart container:
   ```bash
   docker-compose restart
   ```

3. Push to your repository and watch logs:
   ```bash
   docker-compose logs -f
   ```

### Scenario 4: Multi-Container Testing

Create a test setup with broker:

```yaml
# docker-compose.test.yml
version: '3.8'

services:
  broker:
    image: nginx:alpine
    ports:
      - "8080:80"
    networks:
      - dotfile-network

  dotfile-agent-1:
    build: .
    environment:
      - GITHUB_TOKEN=${GITHUB_TOKEN}
      - DOTFILE_MACHINE_ID=machine-1
      - DOTFILE_BROKER_URL=http://broker
    networks:
      - dotfile-network

  dotfile-agent-2:
    build: .
    environment:
      - GITHUB_TOKEN=${GITHUB_TOKEN}
      - DOTFILE_MACHINE_ID=machine-2
      - DOTFILE_BROKER_URL=http://broker
    networks:
      - dotfile-network

networks:
  dotfile-network:
    driver: bridge
```

Run with:
```bash
docker-compose -f docker-compose.test.yml up
```

## Debugging

### View Container Logs

```bash
# All logs
docker-compose logs

# Follow logs
docker-compose logs -f

# Last 50 lines
docker-compose logs --tail=50

# Specific time range
docker-compose logs --since 10m
```

### Access Container Shell

```bash
# Interactive shell
docker-compose exec dotfile-agent sh

# Run commands
docker-compose exec dotfile-agent ls -la /home/dotfile
docker-compose exec dotfile-agent cat /home/dotfile/.config/dotfile-agent/dotfile-agent.doclite
```

### Check Health

```bash
# Container health status
docker inspect --format='{{.State.Health.Status}}' dotfile-agent

# Health check logs
docker inspect --format='{{range .State.Health.Log}}{{.Output}}{{end}}' dotfile-agent
```

### Network Debugging

```bash
# Test connectivity from container
docker-compose exec dotfile-agent wget -O- http://api.github.com

# Check DNS resolution
docker-compose exec dotfile-agent nslookup github.com

# View network details
docker network inspect dotfile-agent_dotfile-network
```

## Troubleshooting

### Issue: Container exits immediately

**Solution**: Check logs for errors
```bash
docker-compose logs
```

Common causes:
- Missing `GITHUB_TOKEN`
- Invalid `GIT_URL`
- Port 3000 already in use

### Issue: Cannot connect to GitHub

**Solution**: Verify token and network
```bash
# Test from container
docker-compose exec dotfile-agent wget -O- https://api.github.com

# Check token
echo $GITHUB_TOKEN
```

### Issue: Files not syncing

**Solution**: Check repository and config
```bash
# Verify repository was cloned
docker-compose exec dotfile-agent ls -la /home/dotfile/dotfiles

# Check config file exists
docker-compose exec dotfile-agent cat /home/dotfile/dotfiles/dotfile-config.yaml

# Trigger manual sync with logs
docker-compose logs -f &
curl -X POST http://localhost:3000/sync
```

### Issue: Permission denied errors

**Solution**: Check volume permissions
```bash
# Inspect volume
docker volume inspect dotfile-agent_dotfiles-data

# Fix permissions (if needed)
docker-compose exec -u root dotfile-agent chown -R dotfile:dotfile /home/dotfile
```

## Performance Testing

### Load Testing

```bash
# Install Apache Bench
apt-get install apache2-utils

# Test sync endpoint
ab -n 100 -c 10 http://localhost:3000/sync

# Stress test
ab -n 1000 -c 50 -t 60 http://localhost:3000/sync
```

### Resource Monitoring

```bash
# Real-time stats
docker stats dotfile-agent

# Memory usage
docker stats --no-stream --format "table {{.Container}}\t{{.MemUsage}}" dotfile-agent

# CPU usage
docker stats --no-stream --format "table {{.Container}}\t{{.CPUPerc}}" dotfile-agent
```

## Cleanup

```bash
# Stop and remove containers
docker-compose down

# Remove volumes
docker-compose down -v

# Remove images
docker rmi dotfile-agent:latest

# Remove all unused resources
docker system prune -a --volumes
```

## Production Considerations

### Security

1. Use secrets management:
   ```bash
   docker secret create github_token token.txt
   ```

2. Run as non-root (already configured)

3. Use read-only root filesystem:
   ```yaml
   security_opt:
     - no-new-privileges:true
   read_only: true
   tmpfs:
     - /tmp
   ```

### Monitoring

1. Add logging driver:
   ```yaml
   logging:
     driver: "json-file"
     options:
       max-size: "10m"
       max-file: "3"
   ```

2. Add health checks (already configured)

3. Use container orchestration (Kubernetes, Docker Swarm)

### Scaling

For multiple machines:

```bash
# Scale to 3 instances
docker-compose up -d --scale dotfile-agent=3

# Use different machine IDs
docker-compose up -d \
  -e DOTFILE_MACHINE_ID=machine-1 \
  -e DOTFILE_MACHINE_ID=machine-2 \
  -e DOTFILE_MACHINE_ID=machine-3
```

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Docker Build and Test

on: [push]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      
      - name: Build image
        run: docker build -t dotfile-agent:test .
      
      - name: Run tests
        run: |
          docker run -d -p 3000:3000 \
            -e GITHUB_TOKEN=${{ secrets.GITHUB_TOKEN }} \
            -e GIT_URL=${{ secrets.GIT_URL }} \
            dotfile-agent:test
          
          sleep 5
          curl -f http://localhost:3000/sync || exit 1
```

## Additional Resources

- [Docker Documentation](https://docs.docker.com/)
- [Docker Compose Documentation](https://docs.docker.com/compose/)
- [GitHub Personal Access Tokens](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token)
