# Load Testing with k6

This directory contains k6 load testing scripts for the Insider Messaging system. These tests help validate system performance, identify bottlenecks, and ensure the application can handle expected traffic loads.

## Prerequisites

1. Install k6: https://k6.io/docs/getting-started/installation/
2. Ensure the messaging service is running locally or specify a different BASE_URL
3. Make sure the database and Redis are properly configured and running

## Test Scripts

### 1. Load Test (`k6-load-test.js`)

**Purpose**: Simulates realistic user load patterns to test normal operation performance.

**Test Scenarios**:
- Message creation and retrieval
- Sent messages listing
- Health checks
- Metrics endpoint validation

**Load Pattern**:
- Ramp up to 10 users over 2 minutes
- Maintain 10 users for 5 minutes
- Ramp up to 20 users over 2 minutes
- Maintain 20 users for 5 minutes
- Ramp down to 0 users over 2 minutes

**Usage**:
```bash
# Run with default settings (localhost:8080)
k6 run test/load/k6-load-test.js

# Run against different environment
BASE_URL=http://staging.example.com:8080 k6 run test/load/k6-load-test.js

# Run with custom options
k6 run --vus 30 --duration 10m test/load/k6-load-test.js
```

### 2. Stress Test (`k6-stress-test.js`)

**Purpose**: Pushes the system beyond normal capacity to identify breaking points and failure modes.

**Test Scenarios**:
- Rapid message creation with minimal delays
- Concurrent message retrieval
- Bulk operations under high load
- System endpoint availability under stress

**Load Pattern**:
- Ramp up to 50 users in 1 minute
- Increase to 100 users over 2 minutes
- Stress test with 200 users for 3 minutes
- Peak stress with 300 users for 2 minutes
- Quick ramp down in 1 minute

**Usage**:
```bash
# Run stress test
k6 run test/load/k6-stress-test.js

# Run with different target
BASE_URL=http://production.example.com k6 run test/load/k6-stress-test.js
```

### 3. Spike Test (`k6-spike-test.js`)

**Purpose**: Tests system resilience during sudden traffic spikes and recovery behavior.

**Test Scenarios**:
- Sudden traffic spikes from 5 to 200+ users
- System behavior during spike maintenance
- Recovery performance after spike ends
- Multiple spike patterns

**Load Pattern**:
- Baseline: 5 users for 2 minutes
- Spike 1: Jump to 200 users for 1 minute
- Recovery: Drop to 5 users for 2 minutes
- Spike 2: Jump to 300 users for 30 seconds
- Complete drop to 0 users

**Usage**:
```bash
# Run spike test
k6 run test/load/k6-spike-test.js

# Monitor system during spikes
BASE_URL=http://localhost:8080 k6 run --out json=spike-results.json test/load/k6-spike-test.js
```

## Metrics and Thresholds

### Load Test Thresholds
- 95% of requests < 500ms response time
- Error rate < 10%
- Message creation success rate > 90%
- Message retrieval success rate > 95%

### Stress Test Thresholds
- 95% of requests < 2000ms (higher tolerance)
- Error rate < 30% (acceptable under stress)
- Message creation success rate > 70%
- System overload rate < 50%

### Spike Test Thresholds
- 90% of requests < 3000ms during spikes
- Spike recovery success rate > 80%
- Spike error rate < 40%

## Custom Metrics

All tests track custom metrics:
- `message_creation_success_rate`: Success rate for message creation
- `message_creation_duration`: Time taken to create messages
- `message_retrieval_success_rate`: Success rate for message retrieval
- `system_overload_rate`: Rate of 429/503 responses (stress test)
- `spike_recovery_success_rate`: Success rate during recovery (spike test)

## Environment Variables

- `BASE_URL`: Target service URL (default: http://localhost:8080)
- `K6_VUS`: Override virtual users count
- `K6_DURATION`: Override test duration

## Running Tests

### Local Development
```bash
# Start the service first
make run

# In another terminal, run tests
k6 run test/load/k6-load-test.js
```

### CI/CD Integration
```bash
# Run all load tests in sequence
k6 run test/load/k6-load-test.js
k6 run test/load/k6-stress-test.js
k6 run test/load/k6-spike-test.js

# Generate reports
k6 run --out json=load-test-results.json test/load/k6-load-test.js
```

### Docker Integration
```bash
# Run tests against dockerized service
docker-compose up -d
BASE_URL=http://localhost:8080 k6 run test/load/k6-load-test.js
docker-compose down
```

## Interpreting Results

### Success Indicators
- All thresholds pass (green checkmarks)
- Response times within acceptable ranges
- Low error rates
- Stable performance across test duration

### Warning Signs
- Increasing response times over test duration
- High error rates (>10% for load, >30% for stress)
- Memory leaks (check system metrics)
- Database connection pool exhaustion

### Failure Indicators
- Threshold violations
- Service crashes or becomes unresponsive
- Cascading failures across system components
- Data corruption or loss

## Monitoring During Tests

While running load tests, monitor:
1. Application logs for errors
2. Database performance and connections
3. Redis memory usage and connections
4. System resources (CPU, memory, disk I/O)
5. Network bandwidth and latency
6. Prometheus metrics at `/metrics` endpoint

## Best Practices

1. **Baseline First**: Always run load tests before stress/spike tests
2. **Monitor Resources**: Watch system resources during tests
3. **Clean Environment**: Use fresh database/cache state for consistent results
4. **Gradual Scaling**: Don't jump directly to maximum load
5. **Document Results**: Keep records of test results for comparison
6. **Test Regularly**: Include load tests in CI/CD pipeline

## Troubleshooting

### Common Issues
- **Connection Refused**: Service not running or wrong BASE_URL
- **High Error Rates**: Database/Redis connection limits reached
- **Timeouts**: Increase timeout values in test scripts
- **Memory Issues**: Check for memory leaks in application

### Performance Tuning
- Adjust database connection pool sizes
- Tune Redis memory settings
- Configure proper logging levels
- Optimize database queries
- Implement caching strategies