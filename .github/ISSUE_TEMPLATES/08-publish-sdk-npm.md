# Publish TypeScript SDK to npm

## Problem / Motivation

Currently, the SDK is available only as source code in the repository. To use it, developers must:

1. Clone the repository or copy files manually
2. Build the SDK themselves
3. Maintain their own local version

This creates friction for adoption and makes it hard to:
- Version the SDK properly
- Distribute updates easily
- Track SDK usage across projects
- Provide proper package management integration
- Support different module systems (ESM/CJS/UMD)

## Proposed Solution

Publish the SDK to npm as a versioned package:

1. **Package Name**: `@go-flagship/sdk` or `go-flagship-sdk`
2. **Build Outputs**: ESM (modern), CJS (Node.js), UMD (browser global)
3. **TypeScript Support**: Ship with `.d.ts` type definitions
4. **Versioning**: Semantic versioning (semver)
5. **Documentation**: README on npm with examples

## Concrete Tasks

### Phase 1: Package Configuration
- [ ] Update `sdk/package.json`:
  ```json
  {
    "name": "@go-flagship/sdk",
    "version": "1.0.0",
    "description": "Official TypeScript SDK for go-flagship feature flags",
    "main": "./dist/index.cjs",
    "module": "./dist/index.mjs",
    "types": "./dist/index.d.ts",
    "exports": {
      ".": {
        "import": "./dist/index.mjs",
        "require": "./dist/index.cjs",
        "types": "./dist/index.d.ts"
      }
    },
    "files": [
      "dist",
      "README.md",
      "LICENSE"
    ],
    "keywords": [
      "feature-flags",
      "feature-toggles",
      "configuration",
      "a-b-testing",
      "typescript",
      "sse",
      "real-time"
    ],
    "author": "Timur Manjosov",
    "license": "MIT",
    "repository": {
      "type": "git",
      "url": "https://github.com/TimurManjosov/go-flagship",
      "directory": "sdk"
    },
    "bugs": {
      "url": "https://github.com/TimurManjosov/go-flagship/issues"
    },
    "homepage": "https://github.com/TimurManjosov/go-flagship#readme"
  }
  ```
- [ ] Set `private: false` (remove private flag)
- [ ] Add proper `engines` field (Node.js version requirements)

### Phase 2: Build Setup
- [ ] Choose build tool:
  - **Option A**: `tsup` (modern, fast, zero-config)
  - **Option B**: `rollup` + plugins (more control)
  - **Option C**: `esbuild` (fastest)
  - **Recommendation**: tsup for simplicity
- [ ] Install build dependencies:
  ```bash
  npm install --save-dev tsup
  ```
- [ ] Configure build script in `package.json`:
  ```json
  {
    "scripts": {
      "build": "tsup src/index.ts --format cjs,esm --dts --clean",
      "prepublishOnly": "npm run build",
      "test": "node ./tests/live-update.spec.js"
    }
  }
  ```
- [ ] Create `src/index.ts` as entry point:
  ```typescript
  export { FlagshipClient } from './flagshipClient';
  export type { FlagshipOptions, UserContext, FlagConfig } from './types';
  ```
- [ ] Move `flagshipClient.ts` to `src/` directory
- [ ] Create TypeScript interfaces in `src/types.ts`

### Phase 3: Build Outputs
- [ ] Generate ESM build: `dist/index.mjs`
  - For modern bundlers (Webpack, Vite, etc.)
  - Tree-shakeable
- [ ] Generate CJS build: `dist/index.cjs`
  - For Node.js and older tools
  - `require()` compatible
- [ ] Generate type definitions: `dist/index.d.ts`
  - TypeScript autocomplete
  - IntelliSense support
- [ ] Verify builds work:
  - Test ESM: `import { FlagshipClient } from '@go-flagship/sdk'`
  - Test CJS: `const { FlagshipClient } = require('@go-flagship/sdk')`
  - Test types: No TypeScript errors

### Phase 4: Package Documentation
- [ ] Create `sdk/README.md` for npm:
  ```markdown
  # @go-flagship/sdk
  
  Official TypeScript SDK for go-flagship feature flags.
  
  ## Installation
  
  ```bash
  npm install @go-flagship/sdk
  ```
  
  ## Quick Start
  
  ```typescript
  import { FlagshipClient } from '@go-flagship/sdk';
  
  const client = new FlagshipClient({
    baseUrl: 'http://localhost:8080',
    user: { id: 'user-123' }
  });
  
  await client.init();
  
  if (client.isEnabled('new_feature')) {
    // Feature is enabled
  }
  ```
  
  ## API Reference
  
  ### `new FlagshipClient(options)`
  ### `client.init()`
  ### `client.isEnabled(key)`
  ### `client.getConfig(key)`
  ### `client.on(event, handler)`
  ### `client.close()`
  
  ## License
  
  MIT
  ```
- [ ] Add JSDoc comments to all public methods
- [ ] Add usage examples for common scenarios:
  - Basic flag checking
  - SSE real-time updates
  - User context
  - Error handling

### Phase 5: Versioning & Changelog
- [ ] Create `sdk/CHANGELOG.md` following Keep a Changelog format
- [ ] Document version 1.0.0 features:
  - Flag snapshot fetching
  - Real-time SSE updates
  - ETag caching
  - Event system (ready, update, error)
  - Type-safe API
- [ ] Add version badge to README
- [ ] Document semver policy:
  - Major: Breaking changes
  - Minor: New features (backward compatible)
  - Patch: Bug fixes

### Phase 6: Publishing
- [ ] Create npm account (if not exists): https://www.npmjs.com/signup
- [ ] Login locally: `npm login`
- [ ] Test publish (dry run):
  ```bash
  cd sdk
  npm pack  # Creates tarball
  tar -tzf go-flagship-sdk-1.0.0.tgz  # Inspect contents
  ```
- [ ] Publish to npm:
  ```bash
  npm publish --access public
  ```
- [ ] Verify package on npm: https://www.npmjs.com/package/@go-flagship/sdk
- [ ] Test installation in fresh project:
  ```bash
  mkdir test-install && cd test-install
  npm init -y
  npm install @go-flagship/sdk
  node -e "const {FlagshipClient} = require('@go-flagship/sdk'); console.log('OK')"
  ```

### Phase 7: CI/CD Automation
- [ ] Add GitHub Actions workflow `.github/workflows/publish-sdk.yml`:
  ```yaml
  name: Publish SDK to npm
  
  on:
    push:
      tags:
        - 'sdk-v*'  # Trigger on tags like sdk-v1.0.0
  
  jobs:
    publish:
      runs-on: ubuntu-latest
      steps:
        - uses: actions/checkout@v3
        - uses: actions/setup-node@v3
          with:
            node-version: '18'
            registry-url: 'https://registry.npmjs.org'
        - name: Install dependencies
          run: cd sdk && npm ci
        - name: Build
          run: cd sdk && npm run build
        - name: Test
          run: cd sdk && npm test
        - name: Publish
          run: cd sdk && npm publish --access public
          env:
            NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}
  ```
- [ ] Add `NPM_TOKEN` secret to GitHub repo settings
- [ ] Document release process:
  ```bash
  # Tag new version
  git tag sdk-v1.0.1
  git push origin sdk-v1.0.1
  # GitHub Actions will automatically publish
  ```

### Phase 8: Post-Publish
- [ ] Update main repository README with npm install instructions
- [ ] Remove or deprecate local SDK copy instructions
- [ ] Add npm version badge: `[![npm version](https://badge.fury.io/js/@go-flagship%2Fsdk.svg)](https://www.npmjs.com/package/@go-flagship/sdk)`
- [ ] Announce on social media / discussions
- [ ] Monitor npm download stats

## API Changes

No API changes. This is purely a packaging/distribution task.

## Acceptance Criteria

### Package
- [ ] Package published to npm as `@go-flagship/sdk`
- [ ] Supports ESM, CJS, and TypeScript out of the box
- [ ] `package.json` follows npm best practices
- [ ] All files needed for usage are included (`dist/`, `README.md`, `LICENSE`)
- [ ] Unnecessary files excluded (tests, source, config files)
- [ ] Package size is reasonable (<50KB minified)

### Build
- [ ] Build script generates all required outputs
- [ ] Type definitions are accurate and complete
- [ ] No build errors or warnings
- [ ] Tree-shaking works (dead code elimination)

### Installation
- [ ] Can install via `npm install @go-flagship/sdk`
- [ ] Works in Node.js project (`require()`)
- [ ] Works in modern bundler (Vite, Webpack - `import`)
- [ ] Works in browser (via CDN like unpkg)
- [ ] TypeScript project gets full type support

### Documentation
- [ ] README on npm is clear and helpful
- [ ] Installation instructions are correct
- [ ] Quick start example works
- [ ] API reference is complete
- [ ] CHANGELOG documents all versions

### Automation
- [ ] GitHub Actions publishes new versions automatically
- [ ] Tagged releases trigger npm publish
- [ ] Failed builds don't publish
- [ ] Version in `package.json` matches git tag

## Notes / Risks / Edge Cases

### Risks
- **Breaking Changes**: Publishing means users depend on API stability
  - Mitigation: Follow semver strictly, document breaking changes
- **npm Account Access**: Need to secure npm account (2FA)
  - Mitigation: Enable 2FA, use automation tokens with minimal permissions
- **Build Failures**: Users might get broken packages
  - Mitigation: Test build in CI before publishing
- **Namespace**: Package name might be taken
  - Mitigation: Check npm before committing to name, have backup options

### Edge Cases
- Publishing from CI without manual approval (use protected branches)
- Version conflicts (publishing same version twice → fails)
- Broken builds published to npm (use `prepublishOnly` script)
- Large bundle size due to dependencies (audit and minimize)
- Missing peer dependencies (document requirements)

### Package Naming Options
- `@go-flagship/sdk` (scoped, professional)
- `go-flagship-sdk` (unscoped, simpler)
- `flagship-sdk` (simple but generic, might be taken)
- `goflagship` (very simple, might be taken)

**Recommendation**: `@go-flagship/sdk` (scoped package under organization)

### Build Tool Comparison

**tsup (Recommended)**
- ✅ Zero config for most cases
- ✅ Built on esbuild (fast)
- ✅ Generates all formats easily
- ✅ Active maintenance

**rollup**
- ✅ More control over output
- ✅ Plugin ecosystem
- ❌ More configuration needed
- ❌ Slower than esbuild

**esbuild**
- ✅ Fastest build
- ❌ No TypeScript declaration files out of the box
- ❌ Requires extra tooling for .d.ts files

### Future Enhancements
- Publish to JSR (Deno registry)
- Provide React hooks wrapper (`useFlagship`, `useFlag`)
- Provide Vue composables
- Provide Svelte stores
- Browser bundle via CDN (unpkg, jsdelivr)
- Bundle size visualization
- Performance benchmarks
- Automated security audits

## Implementation Hints

- Current SDK is in `sdk/flagshipClient.ts`
- Example of good npm package: https://www.npmjs.com/package/@vercel/analytics
- tsup documentation: https://tsup.egoist.dev/
- npm publishing guide: https://docs.npmjs.com/packages-and-modules/contributing-packages-to-the-registry
- Semver spec: https://semver.org/
- GitHub Actions npm publish: https://docs.github.com/en/actions/publishing-packages/publishing-nodejs-packages

## Labels

`feature`, `sdk`, `infrastructure`, `good-first-issue` (for documentation), `help-wanted`

## Estimated Effort

**1-2 days**
- Morning: Package configuration + build setup
- Afternoon: Documentation + local testing
- Next day: Publishing + CI/CD + verification
