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

  console.log(`\n📊 Results: ${passed} passed, ${failed} failed`);
  
  if (failed > 0) {
    process.exitCode = 1;
  }
}

runTests().catch((err) => {
  console.error('Test runner failed:', err);
  process.exitCode = 1;
});
