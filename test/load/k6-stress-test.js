import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// Custom metrics for stress testing
const messageCreationRate = new Rate('stress_message_creation_success_rate');
const messageCreationDuration = new Trend('stress_message_creation_duration');
const systemOverloadRate = new Rate('system_overload_rate');
const errorCounter = new Counter('stress_errors');

// Stress test configuration - aggressive load
export const options = {
  stages: [
    { duration: '1m', target: 50 },   // Quickly ramp up to 50 users
    { duration: '2m', target: 100 },  // Ramp up to 100 users
    { duration: '3m', target: 200 },  // Stress test with 200 users
    { duration: '2m', target: 300 },  // Peak stress with 300 users
    { duration: '1m', target: 0 },    // Quick ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<2000'], // Allow higher latency during stress
    http_req_failed: ['rate<0.3'],     // Allow higher error rate during stress
    stress_message_creation_success_rate: ['rate>0.7'], // 70% success rate acceptable under stress
    system_overload_rate: ['rate<0.5'], // System should not be overloaded more than 50% of the time
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

// Generate high-volume test data
function generateBulkMessages(count = 5) {
  const messages = [];
  for (let i = 0; i < count; i++) {
    messages.push({
      recipient: `stress-user-${Math.floor(Math.random() * 10000)}@example.com`,
      message: `Stress test message ${i} - ${Date.now()}`,
      webhook_url: `${BASE_URL}/webhook/stress-${Math.floor(Math.random() * 10000)}`
    });
  }
  return messages;
}

export default function () {
  // Stress Test 1: Rapid message creation
  const messages = generateBulkMessages(3);
  
  messages.forEach((messageData, index) => {
    const createResponse = http.post(`${BASE_URL}/api/v1/messages`, JSON.stringify(messageData), {
      headers: {
        'Content-Type': 'application/json',
      },
      timeout: '10s', // Longer timeout for stress conditions
    });

    const createSuccess = check(createResponse, {
      'stress message creation completed': (r) => r.status === 201 || r.status === 429 || r.status === 503,
      'stress message creation not server error': (r) => r.status < 500 || r.status === 503,
    });

    // Track system overload (429 Too Many Requests or 503 Service Unavailable)
    const systemOverloaded = createResponse.status === 429 || createResponse.status === 503;
    systemOverloadRate.add(systemOverloaded);

    const actualSuccess = createResponse.status === 201;
    messageCreationRate.add(actualSuccess);
    messageCreationDuration.add(createResponse.timings.duration);

    if (!createSuccess) {
      errorCounter.add(1);
      console.error(`Stress test message ${index} failed: ${createResponse.status} - ${createResponse.body}`);
    }

    // Very short sleep to maintain high pressure
    sleep(0.01);
  });

  // Stress Test 2: Concurrent message retrieval
  const messageIds = [1, 2, 3, 4, 5]; // Assume some messages exist
  
  messageIds.forEach(id => {
    const getResponse = http.get(`${BASE_URL}/api/v1/messages/${id}`, {
      timeout: '5s',
    });
    
    check(getResponse, {
      'stress message retrieval handled': (r) => r.status < 500 || r.status === 503,
    });
  });

  // Stress Test 3: Bulk operations
  const listResponse = http.get(`${BASE_URL}/api/v1/messages/sent?limit=100`, {
    timeout: '10s',
  });
  
  check(listResponse, {
    'stress bulk retrieval handled': (r) => r.status < 500 || r.status === 503,
  });

  // Stress Test 4: Health and metrics under load
  const healthResponse = http.get(`${BASE_URL}/health`, { timeout: '3s' });
  const metricsResponse = http.get(`${BASE_URL}/metrics`, { timeout: '5s' });
  
  check(healthResponse, {
    'health endpoint survives stress': (r) => r.status === 200 || r.status === 503,
  });
  
  check(metricsResponse, {
    'metrics endpoint survives stress': (r) => r.status === 200 || r.status === 503,
  });

  // Minimal sleep to maintain maximum pressure
  sleep(0.05);
}

export function setup() {
  console.log('Starting STRESS test against:', BASE_URL);
  console.log('WARNING: This test will generate high load and may impact system performance');
  
  const healthCheck = http.get(`${BASE_URL}/health`);
  if (healthCheck.status !== 200) {
    throw new Error(`Service not available at ${BASE_URL}. Health check failed with status: ${healthCheck.status}`);
  }
  
  console.log('Service is available. Starting stress test...');
  return { baseUrl: BASE_URL };
}

export function teardown(data) {
  console.log('Stress test completed for:', data.baseUrl);
  console.log('Check system metrics and logs for performance impact analysis');
}