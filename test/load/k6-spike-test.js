import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// Custom metrics for spike testing
const spikeRecoveryRate = new Rate('spike_recovery_success_rate');
const spikeResponseTime = new Trend('spike_response_time');
const spikeErrorRate = new Rate('spike_error_rate');
const recoveryTime = new Trend('recovery_time_seconds');

// Spike test configuration - sudden traffic spikes
export const options = {
  stages: [
    { duration: '2m', target: 5 },    // Normal load baseline
    { duration: '10s', target: 200 }, // Sudden spike!
    { duration: '1m', target: 200 },  // Maintain spike
    { duration: '10s', target: 5 },   // Quick drop
    { duration: '2m', target: 5 },    // Recovery period
    { duration: '10s', target: 300 }, // Even bigger spike!
    { duration: '30s', target: 300 }, // Maintain bigger spike
    { duration: '10s', target: 0 },   // Complete drop
  ],
  thresholds: {
    http_req_duration: ['p(90)<3000'], // 90% under 3s during spikes
    spike_recovery_success_rate: ['rate>0.8'], // 80% success during recovery
    spike_error_rate: ['rate<0.4'], // Less than 40% errors during spikes
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

// Track spike phases
let currentPhase = 'baseline';
let spikeStartTime = null;

function detectPhase() {
  const currentVUs = __VU;
  const totalVUs = __ENV.K6_VUS || 1;
  
  if (totalVUs > 150 && currentPhase !== 'spike') {
    currentPhase = 'spike';
    spikeStartTime = Date.now();
    console.log(`Spike detected! VUs: ${totalVUs}`);
  } else if (totalVUs <= 10 && currentPhase === 'spike') {
    currentPhase = 'recovery';
    if (spikeStartTime) {
      const recoveryDuration = (Date.now() - spikeStartTime) / 1000;
      recoveryTime.add(recoveryDuration);
      console.log(`Recovery phase started. Spike duration: ${recoveryDuration}s`);
    }
  } else if (totalVUs <= 10 && currentPhase === 'recovery') {
    currentPhase = 'baseline';
  }
  
  return currentPhase;
}

export default function () {
  const phase = detectPhase();
  
  // Adjust behavior based on current phase
  let timeout = '5s';
  let expectedSuccessRate = 0.9;
  
  if (phase === 'spike') {
    timeout = '15s'; // Longer timeout during spikes
    expectedSuccessRate = 0.6; // Lower expectations during spikes
  } else if (phase === 'recovery') {
    timeout = '10s';
    expectedSuccessRate = 0.8;
  }

  // Test 1: Message creation during spike
  const messageData = {
    recipient: `spike-user-${__VU}-${Date.now()}@example.com`,
    message: `Spike test message from VU ${__VU} during ${phase} phase`,
    webhook_url: `${BASE_URL}/webhook/spike-${__VU}-${Date.now()}`
  };

  const startTime = Date.now();
  const createResponse = http.post(`${BASE_URL}/api/v1/messages`, JSON.stringify(messageData), {
    headers: {
      'Content-Type': 'application/json',
    },
    timeout: timeout,
  });
  const endTime = Date.now();

  spikeResponseTime.add(endTime - startTime);

  const isError = createResponse.status >= 400;
  spikeErrorRate.add(isError);

  const spikeSuccess = check(createResponse, {
    [`${phase} - message creation handled gracefully`]: (r) => {
      // During spikes, we accept rate limiting and service unavailable
      return r.status === 201 || r.status === 429 || r.status === 503;
    },
    [`${phase} - no server crashes`]: (r) => r.status !== 0 && r.status < 500 || r.status === 503,
  });

  if (phase === 'recovery') {
    spikeRecoveryRate.add(createResponse.status === 201);
  }

  // Test 2: System health during spike
  const healthResponse = http.get(`${BASE_URL}/health`, {
    timeout: '3s',
  });

  check(healthResponse, {
    [`${phase} - health endpoint responsive`]: (r) => r.status === 200 || r.status === 503,
  });

  // Test 3: Metrics collection during spike
  if (Math.random() < 0.1) { // Only 10% of VUs check metrics to reduce load
    const metricsResponse = http.get(`${BASE_URL}/metrics`, {
      timeout: '5s',
    });

    check(metricsResponse, {
      [`${phase} - metrics collection working`]: (r) => r.status === 200 || r.status === 503,
    });
  }

  // Test 4: Message retrieval during spike
  if (createResponse.status === 201) {
    try {
      const createBody = JSON.parse(createResponse.body);
      const messageId = createBody.id;

      const getResponse = http.get(`${BASE_URL}/api/v1/messages/${messageId}`, {
        timeout: timeout,
      });

      check(getResponse, {
        [`${phase} - message retrieval handled`]: (r) => {
          return r.status === 200 || r.status === 429 || r.status === 503 || r.status === 404;
        },
      });
    } catch (e) {
      // Ignore parsing errors during spikes
    }
  }

  // Adaptive sleep based on phase
  if (phase === 'spike') {
    sleep(0.01); // Minimal sleep during spike to maintain pressure
  } else if (phase === 'recovery') {
    sleep(0.1); // Short sleep during recovery
  } else {
    sleep(0.5); // Normal sleep during baseline
  }
}

export function setup() {
  console.log('Starting SPIKE test against:', BASE_URL);
  console.log('This test simulates sudden traffic spikes to test system resilience');
  
  const healthCheck = http.get(`${BASE_URL}/health`);
  if (healthCheck.status !== 200) {
    throw new Error(`Service not available at ${BASE_URL}. Health check failed with status: ${healthCheck.status}`);
  }
  
  console.log('Service is available. Starting spike test...');
  return { baseUrl: BASE_URL, startTime: Date.now() };
}

export function teardown(data) {
  const totalDuration = (Date.now() - data.startTime) / 1000;
  console.log(`Spike test completed for: ${data.baseUrl}`);
  console.log(`Total test duration: ${totalDuration}s`);
  console.log('Analyze system behavior during traffic spikes and recovery periods');
}