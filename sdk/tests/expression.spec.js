/**
 * Unit tests for expression evaluation logic (JSON Logic targeting rules).
 * These tests verify the client-side expression evaluation without needing a server.
 */

import { FlagshipClient } from '../dist/flagshipClient.js';

// Mock snapshot data with expression-based flags
const mockSnapshot = {
  etag: 'test-etag',
  flags: {
    // Simple equality check
    'premium-only': {
      key: 'premium-only',
      enabled: true,
      rollout: 100,
      expression: JSON.stringify({ '==': [{ var: 'plan' }, 'premium'] }),
      env: 'test',
      updatedAt: new Date().toISOString(),
    },
    // Array membership (in operator)
    'us-ca-only': {
      key: 'us-ca-only',
      enabled: true,
      rollout: 100,
      expression: JSON.stringify({ in: [{ var: 'country' }, ['US', 'CA']] }),
      env: 'test',
      updatedAt: new Date().toISOString(),
    },
    // Numeric comparison
    'adult-only': {
      key: 'adult-only',
      enabled: true,
      rollout: 100,
      expression: JSON.stringify({ '>=': [{ var: 'age' }, 18] }),
      env: 'test',
      updatedAt: new Date().toISOString(),
    },
    // Complex AND logic
    'premium-us': {
      key: 'premium-us',
      enabled: true,
      rollout: 100,
      expression: JSON.stringify({
        and: [
          { '==': [{ var: 'plan' }, 'premium'] },
          { '==': [{ var: 'country' }, 'US'] },
        ],
      }),
      env: 'test',
      updatedAt: new Date().toISOString(),
    },
    // OR logic
    'premium-or-beta': {
      key: 'premium-or-beta',
      enabled: true,
      rollout: 100,
      expression: JSON.stringify({
        or: [
          { '==': [{ var: 'plan' }, 'premium'] },
          { '==': [{ var: 'isBeta' }, true] },
        ],
      }),
      env: 'test',
      updatedAt: new Date().toISOString(),
    },
    // NOT logic
    'not-free': {
      key: 'not-free',
      enabled: true,
      rollout: 100,
      expression: JSON.stringify({ '!': { '==': [{ var: 'plan' }, 'free'] } }),
      env: 'test',
      updatedAt: new Date().toISOString(),
    },
    // Complex expression: (premium AND US/CA) OR beta
    'complex-targeting': {
      key: 'complex-targeting',
      enabled: true,
      rollout: 100,
      expression: JSON.stringify({
        or: [
          {
            and: [
              { '==': [{ var: 'plan' }, 'premium'] },
              { in: [{ var: 'country' }, ['US', 'CA']] },
            ],
          },
          { '==': [{ var: 'isBeta' }, true] },
        ],
      }),
      env: 'test',
      updatedAt: new Date().toISOString(),
    },
    // No expression - should work normally
    'no-expression': {
      key: 'no-expression',
      enabled: true,
      rollout: 100,
      env: 'test',
      updatedAt: new Date().toISOString(),
    },
    // Flag disabled with expression - should still be disabled
    'disabled-with-expression': {
      key: 'disabled-with-expression',
      enabled: false,
      rollout: 100,
      expression: JSON.stringify({ '==': [{ var: 'plan' }, 'premium'] }),
      env: 'test',
      updatedAt: new Date().toISOString(),
    },
    // User ID targeting
    'user-id-check': {
      key: 'user-id-check',
      enabled: true,
      rollout: 100,
      expression: JSON.stringify({ '==': [{ var: 'id' }, 'special-user'] }),
      env: 'test',
      updatedAt: new Date().toISOString(),
    },
  },
  updatedAt: new Date().toISOString(),
  rolloutSalt: 'test-salt-12345',
};

// Create a mock fetch that returns the snapshot
function createMockFetch() {
  return async (url, options) => {
    return {
      ok: true,
      status: 200,
      json: async () => mockSnapshot,
    };
  };
}

// Mock EventSource that does nothing
class MockEventSource {
  constructor(url) {
    this.url = url;
  }
  addEventListener() {}
  close() {}
}

async function runTests() {
  console.log('🧪 Running expression evaluation tests...\n');
  let passed = 0;
  let failed = 0;

  function test(name, fn) {
    try {
      fn();
      console.log(`✅ ${name}`);
      passed++;
    } catch (err) {
      console.error(`❌ ${name}`);
      console.error(`   ${err.message}`);
      failed++;
    }
  }

  function assertEqual(actual, expected, msg = '') {
    if (actual !== expected) {
      throw new Error(`Expected ${expected}, got ${actual}. ${msg}`);
    }
  }

  function assertTrue(value, msg = '') {
    if (!value) {
      throw new Error(`Expected true, got ${value}. ${msg}`);
    }
  }

  function assertFalse(value, msg = '') {
    if (value) {
      throw new Error(`Expected false, got ${value}. ${msg}`);
    }
  }

  // Test 1: Premium user should see premium-only flag
  test('Premium user matches premium-only expression', () => {
    const client = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      user: { id: 'user-1', attributes: { plan: 'premium' } },
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    client['cache'] = mockSnapshot;

    assertTrue(client.isEnabled('premium-only'));
  });

  // Test 2: Free user should NOT see premium-only flag
  test('Free user does not match premium-only expression', () => {
    const client = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      user: { id: 'user-1', attributes: { plan: 'free' } },
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    client['cache'] = mockSnapshot;

    assertFalse(client.isEnabled('premium-only'));
  });

  // Test 3: US user should see us-ca-only flag
  test('US user matches country in [US, CA]', () => {
    const client = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      user: { id: 'user-1', attributes: { country: 'US' } },
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    client['cache'] = mockSnapshot;

    assertTrue(client.isEnabled('us-ca-only'));
  });

  // Test 4: CA user should see us-ca-only flag
  test('CA user matches country in [US, CA]', () => {
    const client = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      user: { id: 'user-1', attributes: { country: 'CA' } },
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    client['cache'] = mockSnapshot;

    assertTrue(client.isEnabled('us-ca-only'));
  });

  // Test 5: UK user should NOT see us-ca-only flag
  test('UK user does not match country in [US, CA]', () => {
    const client = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      user: { id: 'user-1', attributes: { country: 'UK' } },
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    client['cache'] = mockSnapshot;

    assertFalse(client.isEnabled('us-ca-only'));
  });

  // Test 6: Adult (age >= 18) matches adult-only
  test('Age 21 matches age >= 18', () => {
    const client = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      user: { id: 'user-1', attributes: { age: 21 } },
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    client['cache'] = mockSnapshot;

    assertTrue(client.isEnabled('adult-only'));
  });

  // Test 7: Minor (age < 18) does NOT match adult-only
  test('Age 16 does not match age >= 18', () => {
    const client = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      user: { id: 'user-1', attributes: { age: 16 } },
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    client['cache'] = mockSnapshot;

    assertFalse(client.isEnabled('adult-only'));
  });

  // Test 8: AND - premium US user matches
  test('Premium US user matches AND condition', () => {
    const client = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      user: { id: 'user-1', attributes: { plan: 'premium', country: 'US' } },
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    client['cache'] = mockSnapshot;

    assertTrue(client.isEnabled('premium-us'));
  });

  // Test 9: AND - premium non-US user does NOT match
  test('Premium UK user does not match AND condition', () => {
    const client = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      user: { id: 'user-1', attributes: { plan: 'premium', country: 'UK' } },
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    client['cache'] = mockSnapshot;

    assertFalse(client.isEnabled('premium-us'));
  });

  // Test 10: OR - beta user (not premium) matches
  test('Beta user matches OR condition', () => {
    const client = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      user: { id: 'user-1', attributes: { plan: 'free', isBeta: true } },
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    client['cache'] = mockSnapshot;

    assertTrue(client.isEnabled('premium-or-beta'));
  });

  // Test 11: OR - premium user (not beta) matches
  test('Premium user (not beta) matches OR condition', () => {
    const client = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      user: { id: 'user-1', attributes: { plan: 'premium', isBeta: false } },
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    client['cache'] = mockSnapshot;

    assertTrue(client.isEnabled('premium-or-beta'));
  });

  // Test 12: OR - neither premium nor beta does NOT match
  test('Free non-beta user does not match OR condition', () => {
    const client = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      user: { id: 'user-1', attributes: { plan: 'free', isBeta: false } },
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    client['cache'] = mockSnapshot;

    assertFalse(client.isEnabled('premium-or-beta'));
  });

  // Test 13: NOT - premium user is not free
  test('Premium user matches NOT free', () => {
    const client = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      user: { id: 'user-1', attributes: { plan: 'premium' } },
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    client['cache'] = mockSnapshot;

    assertTrue(client.isEnabled('not-free'));
  });

  // Test 14: NOT - free user is free
  test('Free user does not match NOT free', () => {
    const client = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      user: { id: 'user-1', attributes: { plan: 'free' } },
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    client['cache'] = mockSnapshot;

    assertFalse(client.isEnabled('not-free'));
  });

  // Test 15: Complex - premium US user matches
  test('Premium US user matches complex targeting', () => {
    const client = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      user: { id: 'user-1', attributes: { plan: 'premium', country: 'US', isBeta: false } },
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    client['cache'] = mockSnapshot;

    assertTrue(client.isEnabled('complex-targeting'));
  });

  // Test 16: Complex - beta user matches
  test('Beta user matches complex targeting', () => {
    const client = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      user: { id: 'user-1', attributes: { plan: 'free', country: 'UK', isBeta: true } },
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    client['cache'] = mockSnapshot;

    assertTrue(client.isEnabled('complex-targeting'));
  });

  // Test 17: Complex - neither premium-US nor beta does not match
  test('Free UK non-beta user does not match complex targeting', () => {
    const client = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      user: { id: 'user-1', attributes: { plan: 'free', country: 'UK', isBeta: false } },
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    client['cache'] = mockSnapshot;

    assertFalse(client.isEnabled('complex-targeting'));
  });

  // Test 18: No expression - should work normally
  test('Flag without expression works normally', () => {
    const client = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      user: { id: 'user-1', attributes: {} },
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    client['cache'] = mockSnapshot;

    assertTrue(client.isEnabled('no-expression'));
  });

  // Test 19: Disabled flag with expression is still disabled
  test('Disabled flag with matching expression is still disabled', () => {
    const client = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      user: { id: 'user-1', attributes: { plan: 'premium' } },
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    client['cache'] = mockSnapshot;

    assertFalse(client.isEnabled('disabled-with-expression'));
  });

  // Test 20: Missing attribute returns false
  test('Missing attribute causes expression to not match', () => {
    const client = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      user: { id: 'user-1', attributes: {} }, // No plan attribute
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    client['cache'] = mockSnapshot;

    assertFalse(client.isEnabled('premium-only'));
  });

  // Test 21: User ID is available in expression context
  test('User ID is available in expression context', () => {
    const client = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      user: { id: 'special-user', attributes: {} },
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    client['cache'] = mockSnapshot;

    assertTrue(client.isEnabled('user-id-check'));
  });

  // Test 22: Wrong user ID does not match
  test('Wrong user ID does not match expression', () => {
    const client = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      user: { id: 'other-user', attributes: {} },
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    client['cache'] = mockSnapshot;

    assertFalse(client.isEnabled('user-id-check'));
  });

  // Test 23: setUser changes expression evaluation
  test('setUser changes expression evaluation', () => {
    const client = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      user: { id: 'user-1', attributes: { plan: 'free' } },
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    client['cache'] = mockSnapshot;

    // Should not match initially
    assertFalse(client.isEnabled('premium-only'));

    // Change to premium user
    client.setUser({ id: 'user-1', attributes: { plan: 'premium' } });

    // Should now match
    assertTrue(client.isEnabled('premium-only'));
  });

  console.log(`\n📊 Results: ${passed} passed, ${failed} failed`);

  if (failed > 0) {
    process.exitCode = 1;
  }
}

runTests().catch((err) => {
  console.error('Test runner failed:', err);
  process.exitCode = 1;
});
