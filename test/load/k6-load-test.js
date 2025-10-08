import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// Custom metrics
const messageCreationRate = new Rate('message_creation_success_rate');
const messageCreationDuration = new Trend('message_creation_duration');
const messageRetrievalRate = new Rate('message_retrieval_success_rate');
const messageRetrievalDuration = new Trend('message_retrieval_duration');
const errorCounter = new Counter('errors');

// Test configuration
export const options = {
  stages: [
    { duration: '2m', target: 10 },   // Ramp up to 10 users over 2 minutes
    { duration: '5m', target: 10 },   // Stay at 10 users for 5 minutes
    { duration: '2m', target: 20 },   // Ramp up to 20 users over 2 minutes
    { duration: '5m', target: 20 },   // Stay at 20 users for 5 minutes
    { duration: '2m', target: 0 },    // Ramp down to 0 users over 2 minutes
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'], // 95% of requests should be below 500ms
    http_req_failed: ['rate<0.1'],    // Error rate should be below 10%
    message_creation_success_rate: ['rate>0.9'], // 90% success rate for message creation
    message_retrieval_success_rate: ['rate>0.95'], // 95% success rate for message retrieval
  },
};

// Base URL - can be overridden with environment variable
const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

// Test data generators
function generateRandomMessage() {
  const recipients = [
    'user1@example.com',
    'user2@example.com', 
    'user3@example.com',
    'user4@example.com',
    'user5@example.com'
  ];
  
  const messages = [
    'Hello, this is a test message!',
    'Important notification for you.',
    'Your order has been processed.',
    'Welcome to our service!',
    'Thank you for your subscription.'
  ];

  return {
    recipient: recipients[Math.floor(Math.random() * recipients.length)],
    message: messages[Math.floor(Math.random() * messages.length)],
    webhook_url: `${BASE_URL}/webhook/test-${Math.floor(Math.random() * 1000)}`
  };
}

// Main test function
export default function () {
  // Test 1: Create a new message
  const messageData = generateRandomMessage();
  
  const createResponse = http.post(`${BASE_URL}/api/v1/messages`, JSON.stringify(messageData), {
    headers: {
      'Content-Type': 'application/json',
    },
  });

  const createSuccess = check(createResponse, {
    'message creation status is 201': (r) => r.status === 201,
    'message creation response has id': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.id !== undefined;
      } catch (e) {
        return false;
      }
    },
  });

  messageCreationRate.add(createSuccess);
  messageCreationDuration.add(createResponse.timings.duration);

  if (!createSuccess) {
    errorCounter.add(1);
    console.error(`Message creation failed: ${createResponse.status} - ${createResponse.body}`);
    return;
  }

  // Extract message ID from response
  let messageId;
  try {
    const createBody = JSON.parse(createResponse.body);
    messageId = createBody.id;
  } catch (e) {
    errorCounter.add(1);
    console.error('Failed to parse create response body');
    return;
  }

  // Test 2: Retrieve the created message
  sleep(0.1); // Small delay to simulate real usage
  
  const getResponse = http.get(`${BASE_URL}/api/v1/messages/${messageId}`);
  
  const retrievalSuccess = check(getResponse, {
    'message retrieval status is 200': (r) => r.status === 200,
    'message retrieval response has correct id': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.id === messageId;
      } catch (e) {
        return false;
      }
    },
    'message retrieval response has recipient': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.recipient === messageData.recipient;
      } catch (e) {
        return false;
      }
    },
  });

  messageRetrievalRate.add(retrievalSuccess);
  messageRetrievalDuration.add(getResponse.timings.duration);

  if (!retrievalSuccess) {
    errorCounter.add(1);
    console.error(`Message retrieval failed: ${getResponse.status} - ${getResponse.body}`);
  }

  // Test 3: Get sent messages (list endpoint)
  sleep(0.1);
  
  const listResponse = http.get(`${BASE_URL}/api/v1/messages/sent?limit=10`);
  
  check(listResponse, {
    'sent messages list status is 200': (r) => r.status === 200,
    'sent messages list response is array': (r) => {
      try {
        const body = JSON.parse(r.body);
        return Array.isArray(body);
      } catch (e) {
        return false;
      }
    },
  });

  // Test 4: Health check
  const healthResponse = http.get(`${BASE_URL}/health`);
  
  check(healthResponse, {
    'health check status is 200': (r) => r.status === 200,
  });

  // Test 5: Metrics endpoint
  const metricsResponse = http.get(`${BASE_URL}/metrics`);
  
  check(metricsResponse, {
    'metrics endpoint status is 200': (r) => r.status === 200,
    'metrics response contains prometheus metrics': (r) => r.body.includes('insider_messaging'),
  });

  // Random sleep between 0.5 and 2 seconds to simulate real user behavior
  sleep(Math.random() * 1.5 + 0.5);
}

// Setup function - runs once before the test
export function setup() {
  console.log('Starting load test against:', BASE_URL);
  
  // Test if the service is available
  const healthCheck = http.get(`${BASE_URL}/health`);
  if (healthCheck.status !== 200) {
    throw new Error(`Service not available at ${BASE_URL}. Health check failed with status: ${healthCheck.status}`);
  }
  
  console.log('Service is available. Starting load test...');
  return { baseUrl: BASE_URL };
}

// Teardown function - runs once after the test
export function teardown(data) {
  console.log('Load test completed for:', data.baseUrl);
}