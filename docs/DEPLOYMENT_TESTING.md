# Deployment Testing

This directory contains tools for testing the ORLY deployment script to ensure it works correctly across different environments.

## Test Scripts

### Local Testing (Recommended)

```bash
./scripts/test-deploy-local.sh
```

This script tests the deployment functionality locally without requiring Docker. It validates:

- ✅ Script help functionality
- ✅ Required files and permissions
- ✅ Go download URL accessibility
- ✅ Environment file generation
- ✅ Systemd service file creation
- ✅ Go module configuration
- ✅ Build capability (if Go is available)

### Docker Testing

```bash
./scripts/test-deploy-docker.sh
```

This script creates a clean Ubuntu 22.04 container and tests the full deployment process. Requires Docker to be installed and accessible.

If you get permission errors, try:
```bash
sudo ./scripts/test-deploy-docker.sh
```

Or add your user to the docker group:
```bash
sudo usermod -aG docker $USER
newgrp docker
```

## Docker Files

### `scripts/Dockerfile.deploy-test`

A comprehensive Docker image that:
- Starts with Ubuntu 22.04
- Creates a non-root test user
- Copies the project files
- Runs extensive deployment validation tests
- Generates a detailed test report

### `.dockerignore`

Optimizes Docker builds by excluding unnecessary files like:
- Build artifacts
- IDE files
- Git history
- Node modules (rebuilt during test)
- Documentation files

## Test Coverage

The tests validate all aspects of the deployment script:

1. **Environment Setup**
   - Go installation detection
   - Directory creation
   - Environment file generation
   - Shell configuration

2. **Dependency Management**
   - Go download URL validation
   - Build dependency scripts
   - Web UI build process

3. **System Integration**
   - Systemd service creation
   - Capability setting for port 443
   - Binary installation
   - Security hardening

4. **Error Handling**
   - Invalid directory detection
   - Missing file validation
   - Permission checks
   - Network accessibility

## Usage Examples

### Quick Validation
```bash
# Test locally (fastest)
./scripts/test-deploy-local.sh

# View the generated report
cat deployment-test-report.txt
```

### Full Environment Testing
```bash
# Test in clean Docker environment
./scripts/test-deploy-docker.sh

# Test with different architectures
docker build --platform linux/arm64 -f scripts/Dockerfile.deploy-test -t orly-deploy-test-arm64 .
docker run --rm orly-deploy-test-arm64
```

### CI/CD Integration
```bash
# In your CI pipeline
./scripts/test-deploy-local.sh || exit 1
echo "Deployment script validation passed"
```

## Troubleshooting

### Docker Permission Issues
```bash
# Add user to docker group
sudo usermod -aG docker $USER
newgrp docker

# Or run with sudo
sudo ./scripts/test-deploy-docker.sh
```

### Missing Dependencies
```bash
# Install curl for URL testing
sudo apt install curl

# Install Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
```

### Build Test Failures
The build test may be skipped if:
- Go is not installed
- Build dependencies are missing
- Network is unavailable

This is normal for testing environments and doesn't affect deployment validation.

## Test Reports

Both test scripts generate detailed reports:

- **Local**: `deployment-test-report.txt`
- **Docker**: Displayed in container output

Reports include:
- System information
- Test results summary
- Validation status for each component
- Deployment readiness confirmation

## Integration with Deployment

These tests are designed to validate the deployment script before actual deployment:

```bash
# 1. Test the deployment script
./scripts/test-deploy-local.sh

# 2. If tests pass, deploy to production
./scripts/deploy.sh

# 3. Configure and start the service
export ORLY_TLS_DOMAINS=relay.example.com
sudo systemctl start orly
```

The tests ensure that the deployment script will work correctly in production environments.
