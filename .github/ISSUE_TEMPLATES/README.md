# GitHub Issues for go-flagship Roadmap

This directory contains comprehensive GitHub Issues for the next roadmap features of go-flagship.

## Overview

Each issue is:
- **Actionable**: Clear tasks with acceptance criteria
- **Scoped**: 1-3 days of work (some can be split further)
- **Detailed**: Problem statement, solution, implementation hints
- **Professional**: Ready for public open-source usage

## Issues List

### Core Infrastructure

1. **[01-persistence-first-class.md](01-persistence-first-class.md)** - Unify Store Interface for Memory and Postgres Persistence
   - Labels: `feature`, `backend`, `refactor`, `performance`
   - Effort: 2-3 days
   - Priority: High (foundation for other features)

2. **[02-auth-security-hardening.md](02-auth-security-hardening.md)** - Implement Auth and Security Hardening
   - Labels: `feature`, `backend`, `security`, `high-priority`
   - Effort: 3 days
   - Priority: High (security critical)

3. **[04-automated-tests-etag-sse.md](04-automated-tests-etag-sse.md)** - Add Automated Tests for ETag and SSE Semantics
   - Labels: `feature`, `backend`, `testing`, `good-first-issue`
   - Effort: 2-3 days
   - Priority: High (quality assurance)

### Developer Experience

4. **[03-validation-error-ux.md](03-validation-error-ux.md)** - Improve Validation and Error UX
   - Labels: `feature`, `backend`, `frontend`, `ui`, `dx`
   - Effort: 2-3 days
   - Priority: Medium

5. **[08-publish-sdk-npm.md](08-publish-sdk-npm.md)** - Publish TypeScript SDK to npm
   - Labels: `feature`, `sdk`, `infrastructure`, `good-first-issue`
   - Effort: 1-2 days
   - Priority: Medium

6. **[09-cli-tool-ops-ci.md](09-cli-tool-ops-ci.md)** - Build CLI Tool for Ops and CI Usage
   - Labels: `feature`, `cli`, `tooling`, `help-wanted`
   - Effort: 3-4 days
   - Priority: Medium

### Feature Flags Advanced Features

7. **[05-rollout-engine-user-hashing.md](05-rollout-engine-user-hashing.md)** - Implement Rollout Engine with Percentage Splits and User ID Hashing
   - Labels: `feature`, `backend`, `sdk`, `algorithm`
   - Effort: 3 days
   - Priority: High (core feature flag capability)

8. **[06-targeting-rules-expressions.md](06-targeting-rules-expressions.md)** - Implement Targeting Rules with Expression DSL
   - Labels: `feature`, `backend`, `sdk`, `enhancement`
   - Effort: 2-3 days
   - Priority: Medium
   - Depends on: #5 (for rollout context)

9. **[07-context-aware-evaluation-endpoint.md](07-context-aware-evaluation-endpoint.md)** - Implement Context-Aware Evaluation Endpoint
   - Labels: `feature`, `backend`, `api`, `enhancement`
   - Effort: 2 days
   - Priority: Medium
   - Depends on: #5, #6 (for full evaluation logic)

### Admin UI & User Interface

10. **[10-admin-ui-improvements.md](10-admin-ui-improvements.md)** - Enhance Admin UI with Search, Filters, Env Tabs, JSON Editor, and Audit View
    - Labels: `feature`, `frontend`, `ui`, `enhancement`
    - Effort: 4-5 days (can be split into 3 smaller PRs)
    - Priority: Medium
    - Can be split into:
      - 10a: Search, Filters, Env Tabs (2 days)
      - 10b: JSON Editor & Audit View (2 days)
      - 10c: Bulk Actions & Responsive Design (2 days)

### Enterprise Features

11. **[11-projects-namespaces-multitenant.md](11-projects-namespaces-multitenant.md)** - Implement Projects/Namespaces for Multi-Tenant Support
    - Labels: `feature`, `backend`, `frontend`, `sdk`, `database`, `breaking-change`
    - Effort: 4-5 days
    - Priority: Low (nice-to-have, but significant refactor)

12. **[12-audit-log-backend.md](12-audit-log-backend.md)** - Implement Backend Audit Log
    - Labels: `feature`, `backend`, `security`, `compliance`, `database`
    - Effort: 2-3 days
    - Priority: Medium (important for compliance)

13. **[13-webhooks-on-change.md](13-webhooks-on-change.md)** - Implement Webhooks on Flag Changes
    - Labels: `feature`, `backend`, `integration`
    - Effort: 3-4 days
    - Priority: Medium

## Recommended Implementation Order

### Phase 1: Foundation (Week 1-2)
1. Issue #1: Store Interface (enables testing)
2. Issue #4: Automated Tests (quality foundation)
3. Issue #2: Auth & Security (security foundation)

### Phase 2: Core Flag Features (Week 3-4)
4. Issue #5: Rollout Engine (percentage rollouts)
5. Issue #6: Targeting Rules (expression-based targeting)
6. Issue #7: Evaluation Endpoint (server-side evaluation)

### Phase 3: Developer Experience (Week 5-6)
7. Issue #3: Validation & Error UX (better DX)
8. Issue #8: Publish SDK to npm (distribution)
9. Issue #9: CLI Tool (ops tooling)

### Phase 4: UI & Enterprise (Week 7-9)
10. Issue #10: Admin UI Improvements (better management)
11. Issue #12: Audit Log (compliance)
12. Issue #13: Webhooks (integrations)
13. Issue #11: Multi-tenancy (if needed)

## Usage

### Creating GitHub Issues

You can either:

1. **Manual Creation**: Copy content from each `.md` file into GitHub issue creation UI
   - Go to: https://github.com/TimurManjosov/goflagship/issues/new
   - Copy/paste title and content from each file

2. **GitHub CLI** (recommended):
   ```bash
   # Install gh CLI if not already installed
   # https://cli.github.com/
   
   # Create all issues at once
   for file in .github/ISSUE_TEMPLATES/*.md; do
     title=$(head -n 1 "$file" | sed 's/^# //')
     body=$(tail -n +2 "$file")
     gh issue create --title "$title" --body "$body" --label "roadmap"
   done
   ```

3. **Script** (automated):
   ```bash
   #!/bin/bash
   for file in .github/ISSUE_TEMPLATES/*.md; do
     echo "Creating issue from $file..."
     title=$(head -n 1 "$file" | sed 's/^# //')
     body=$(tail -n +3 "$file")
     gh issue create \
       --title "$title" \
       --body "$body" \
       --label "roadmap,enhancement"
   done
   ```

### Customization

Before creating issues, you may want to:

- Add organization-specific labels
- Adjust effort estimates based on your team
- Modify priorities based on business needs
- Split large issues (e.g., #10) into smaller ones
- Add milestone associations
- Assign issues to team members

## Statistics

- **Total Issues**: 13
- **Total Lines**: ~4,500 lines of detailed specifications
- **Total Estimated Effort**: ~35-45 days (varies by team)
- **Coverage**: Backend, Frontend, SDK, CLI, Infrastructure

## Labels Used

- `feature` - New feature
- `backend` - Backend/API work
- `frontend` - Frontend/UI work
- `sdk` - TypeScript SDK work
- `cli` - CLI tool work
- `testing` - Test infrastructure
- `security` - Security-related
- `database` - Database schema changes
- `breaking-change` - Breaking API changes
- `good-first-issue` - Good for new contributors
- `help-wanted` - Community help appreciated
- `high-priority` - Should be done soon
- `enhancement` - Improvement to existing feature
- `refactor` - Code refactoring
- `performance` - Performance optimization
- `dx` - Developer experience
- `ui` - User interface
- `compliance` - Compliance/audit related
- `integration` - External integrations
- `infrastructure` - Build/deploy/packaging
- `algorithm` - Algorithm implementation
- `api` - API changes

## Contributing

These issues are designed to be beginner-friendly where marked with `good-first-issue`. Each issue includes:

- Clear problem statement
- Proposed solution
- Step-by-step tasks
- Implementation hints
- Acceptance criteria
- Edge cases to consider

Contributors should:
1. Read the full issue before starting
2. Ask questions in issue comments
3. Break down work into small PRs
4. Write tests for all changes
5. Update documentation

## License

These issue templates are part of the go-flagship project and follow the same MIT license.

---

**Generated**: 2025-01-22  
**Repository**: https://github.com/TimurManjosov/goflagship  
**Author**: AI-assisted roadmap planning
