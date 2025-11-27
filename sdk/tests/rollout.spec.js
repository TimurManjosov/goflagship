/**
 * Unit tests for rollout evaluation logic.
 * These tests verify the client-side rollout and variant bucketing without needing a server.
 */

import { FlagshipClient } from '../dist/flagshipClient.js';

// Mock snapshot data for testing
const mockSnapshot = {
  etag: 'test-etag',
  flags: {
    'feature-100': {
      key: 'feature-100',
      enabled: true,
      rollout: 100,
      env: 'test',
      updatedAt: new Date().toISOString(),
    },
    'feature-0': {
      key: 'feature-0',
      enabled: true,
      rollout: 0,
      env: 'test',
      updatedAt: new Date().toISOString(),
    },
    'feature-50': {
      key: 'feature-50',
      enabled: true,
      rollout: 50,
      env: 'test',
      updatedAt: new Date().toISOString(),
    },
    'feature-disabled': {
      key: 'feature-disabled',
      enabled: false,
      rollout: 100,
      env: 'test',
      updatedAt: new Date().toISOString(),
    },
    'ab-test': {
      key: 'ab-test',
      enabled: true,
      rollout: 100,
      variants: [
        { name: 'control', weight: 50, config: { layout: 'old' } },
        { name: 'treatment', weight: 50, config: { layout: 'new' } },
      ],
      env: 'test',
      updatedAt: new Date().toISOString(),
    },
    'multi-variant': {
      key: 'multi-variant',
      enabled: true,
      rollout: 100,
      variants: [
        { name: 'A', weight: 33, config: { version: 'A' } },
        { name: 'B', weight: 33, config: { version: 'B' } },
        { name: 'C', weight: 34, config: { version: 'C' } },
      ],
      env: 'test',
      updatedAt: new Date().toISOString(),
    },
    // Expression-based targeting flags
    'premium-only': {
      key: 'premium-only',
      enabled: true,
      rollout: 100,
      expression: '{"==": [{"var": "plan"}, "premium"]}',
      env: 'test',
      updatedAt: new Date().toISOString(),
    },
    'us-users': {
      key: 'us-users',
      enabled: true,
      rollout: 100,
      expression: '{"in": [{"var": "country"}, ["US", "CA"]]}',
      env: 'test',
      updatedAt: new Date().toISOString(),
    },
    'complex-targeting': {
      key: 'complex-targeting',
      enabled: true,
      rollout: 100,
      expression: '{"or": [{"and": [{"==": [{"var": "plan"}, "premium"]}, {"in": [{"var": "country"}, ["US", "CA"]]}]}, {"==": [{"var": "betaTester"}, true]}]}',
      env: 'test',
      updatedAt: new Date().toISOString(),
    },
    'expression-with-rollout': {
      key: 'expression-with-rollout',
      enabled: true,
      rollout: 50,
      expression: '{"==": [{"var": "plan"}, "premium"]}',
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
  console.log('🧪 Running rollout evaluation tests...\n');
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

  // Initialize client with mock
  const client = new FlagshipClient({
    baseUrl: 'http://localhost:8080',
    user: { id: 'test-user-123' },
    fetchImpl: createMockFetch(),
    eventSourceImpl: MockEventSource,
  });

  await client.init();

  // Test 1: 100% rollout should always be enabled
  test('100% rollout returns true for any user', () => {
    assertTrue(client.isEnabled('feature-100'));
  });

  // Test 2: 0% rollout should always be disabled
  test('0% rollout returns false for any user', () => {
    assertFalse(client.isEnabled('feature-0'));
  });

  // Test 3: Disabled flag should return false even with 100% rollout
  test('Disabled flag returns false even with 100% rollout', () => {
    assertFalse(client.isEnabled('feature-disabled'));
  });

  // Test 4: Non-existent flag returns false
  test('Non-existent flag returns false', () => {
    assertFalse(client.isEnabled('non-existent'));
  });

  // Test 5: isEnabled is deterministic (same user always gets same result)
  test('isEnabled is deterministic', () => {
    const result1 = client.isEnabled('feature-50');
    const result2 = client.isEnabled('feature-50');
    assertEqual(result1, result2);
  });

  // Test 6: No user context should return false for partial rollouts
  test('No user context returns false for partial rollout', () => {
    const noUserClient = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      // No user!
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    
    // Manually set the cache
    noUserClient['cache'] = mockSnapshot;
    
    assertFalse(noUserClient.isEnabled('feature-50'));
    // But 100% rollout should still work
    assertTrue(noUserClient.isEnabled('feature-100'));
  });

  // Test 7: Variants - getVariant returns a variant name
  test('getVariant returns a variant name', () => {
    const variant = client.getVariant('ab-test');
    assertTrue(variant === 'control' || variant === 'treatment', 
      `Expected control or treatment, got ${variant}`);
  });

  // Test 8: Variants - getVariant is deterministic
  test('getVariant is deterministic', () => {
    const variant1 = client.getVariant('ab-test');
    const variant2 = client.getVariant('ab-test');
    assertEqual(variant1, variant2);
  });

  // Test 9: Variants - getVariantConfig returns the correct config
  test('getVariantConfig returns correct config', () => {
    const variant = client.getVariant('ab-test');
    const config = client.getVariantConfig('ab-test');
    
    if (variant === 'control') {
      assertEqual(config?.layout, 'old');
    } else {
      assertEqual(config?.layout, 'new');
    }
  });

  // Test 10: Variants - undefined for flags without variants
  test('getVariant returns undefined for flag without variants', () => {
    const variant = client.getVariant('feature-100');
    assertEqual(variant, undefined);
  });

  // Test 11: Variants - undefined for non-existent flag
  test('getVariant returns undefined for non-existent flag', () => {
    const variant = client.getVariant('non-existent');
    assertEqual(variant, undefined);
  });

  // Test 12: setUser changes user context
  test('setUser changes user context and affects rollout', () => {
    const result1 = client.isEnabled('feature-50');
    client.setUser({ id: 'different-user-456' });
    // The result may or may not change depending on bucketing
    // But getUser should return the new user
    assertEqual(client.getUser()?.id, 'different-user-456');
    
    // Reset user for other tests
    client.setUser({ id: 'test-user-123' });
  });

  // Test 13: Multi-variant returns one of the variants
  test('Multi-variant returns one of the expected variants', () => {
    const variant = client.getVariant('multi-variant');
    assertTrue(variant === 'A' || variant === 'B' || variant === 'C',
      `Expected A, B, or C, got ${variant}`);
  });

  // Test 14: Distribution test - run many users and check distribution
  test('Rollout distribution is roughly correct for 50%', () => {
    let enabledCount = 0;
    const totalUsers = 1000;
    
    for (let i = 0; i < totalUsers; i++) {
      const testClient = new FlagshipClient({
        baseUrl: 'http://localhost:8080',
        user: { id: `user-${i}` },
        fetchImpl: createMockFetch(),
        eventSourceImpl: MockEventSource,
      });
      testClient['cache'] = mockSnapshot;
      
      if (testClient.isEnabled('feature-50')) {
        enabledCount++;
      }
    }
    
    const percentage = (enabledCount / totalUsers) * 100;
    // Allow 10% variance (40-60%)
    assertTrue(percentage >= 40 && percentage <= 60,
      `Expected ~50% enabled, got ${percentage.toFixed(1)}%`);
  });

  // ---- Expression Evaluation Tests ----

  // Test 15: Simple expression - premium only
  test('Expression: premium-only returns true for premium users', () => {
    const premiumClient = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      user: { id: 'user-1', attributes: { plan: 'premium' } },
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    premiumClient['cache'] = mockSnapshot;
    assertTrue(premiumClient.isEnabled('premium-only'));
  });

  test('Expression: premium-only returns false for free users', () => {
    const freeClient = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      user: { id: 'user-2', attributes: { plan: 'free' } },
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    freeClient['cache'] = mockSnapshot;
    assertFalse(freeClient.isEnabled('premium-only'));
  });

  // Test 16: IN array expression
  test('Expression: us-users returns true for US users', () => {
    const usClient = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      user: { id: 'user-3', attributes: { country: 'US' } },
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    usClient['cache'] = mockSnapshot;
    assertTrue(usClient.isEnabled('us-users'));
  });

  test('Expression: us-users returns true for CA users', () => {
    const caClient = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      user: { id: 'user-4', attributes: { country: 'CA' } },
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    caClient['cache'] = mockSnapshot;
    assertTrue(caClient.isEnabled('us-users'));
  });

  test('Expression: us-users returns false for UK users', () => {
    const ukClient = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      user: { id: 'user-5', attributes: { country: 'UK' } },
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    ukClient['cache'] = mockSnapshot;
    assertFalse(ukClient.isEnabled('us-users'));
  });

  // Test 17: Complex expression (premium in US/CA OR beta tester)
  test('Expression: complex-targeting returns true for premium US user', () => {
    const premiumUsClient = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      user: { id: 'user-6', attributes: { plan: 'premium', country: 'US', betaTester: false } },
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    premiumUsClient['cache'] = mockSnapshot;
    assertTrue(premiumUsClient.isEnabled('complex-targeting'));
  });

  test('Expression: complex-targeting returns false for premium UK user (not beta)', () => {
    const premiumUkClient = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      user: { id: 'user-7', attributes: { plan: 'premium', country: 'UK', betaTester: false } },
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    premiumUkClient['cache'] = mockSnapshot;
    assertFalse(premiumUkClient.isEnabled('complex-targeting'));
  });

  test('Expression: complex-targeting returns true for free UK beta tester', () => {
    const betaTesterClient = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      user: { id: 'user-8', attributes: { plan: 'free', country: 'UK', betaTester: true } },
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    betaTesterClient['cache'] = mockSnapshot;
    assertTrue(betaTesterClient.isEnabled('complex-targeting'));
  });

  // Test 18: Expression with rollout - only premium AND in rollout
  test('Expression with rollout: non-premium users are excluded regardless of rollout', () => {
    const freeClient = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      user: { id: 'user-9', attributes: { plan: 'free' } },
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    freeClient['cache'] = mockSnapshot;
    // Should be false because expression fails
    assertFalse(freeClient.isEnabled('expression-with-rollout'));
  });

  // Test 19: No user attributes - should fail expression
  test('Expression: missing attributes returns false', () => {
    const noAttrsClient = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      user: { id: 'user-10' }, // No attributes
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    noAttrsClient['cache'] = mockSnapshot;
    assertFalse(noAttrsClient.isEnabled('premium-only'));
  });

  // Test 20: User ID is passed to expression context
  test('Expression: user id is available in context', () => {
    // Create a flag with expression checking user id
    const idCheckSnapshot = {
      ...mockSnapshot,
      flags: {
        ...mockSnapshot.flags,
        'id-check': {
          key: 'id-check',
          enabled: true,
          rollout: 100,
          expression: '{"==": [{"var": "id"}, "special-user"]}',
          env: 'test',
          updatedAt: new Date().toISOString(),
        },
      },
    };
    
    const specialClient = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      user: { id: 'special-user' },
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    specialClient['cache'] = idCheckSnapshot;
    assertTrue(specialClient.isEnabled('id-check'));
    
    const normalClient = new FlagshipClient({
      baseUrl: 'http://localhost:8080',
      user: { id: 'normal-user' },
      fetchImpl: createMockFetch(),
      eventSourceImpl: MockEventSource,
    });
    normalClient['cache'] = idCheckSnapshot;
    assertFalse(normalClient.isEnabled('id-check'));
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
