# Implement Projects/Namespaces for Multi-Tenant Support

## Problem / Motivation

Currently, all flags exist in a flat namespace with environment (dev/staging/prod) as the only organizational dimension. This doesn't work for:

1. **Multi-Tenant SaaS**: Can't isolate flags per customer/org
2. **Multiple Products**: Can't separate flags for different products/services
3. **Team Isolation**: Can't prevent teams from interfering with each other's flags
4. **Namespace Conflicts**: Different teams might want same flag key
5. **Access Control**: Can't grant permissions per project (all-or-nothing)

Real-world example:
- **Acme Corp** hosts go-flagship for multiple customers
  - Customer A: E-commerce platform (100 flags)
  - Customer B: Analytics dashboard (80 flags)
  - Customer C: Mobile app (60 flags)
- Each customer needs isolated flag namespace
- Each customer has separate dev/staging/prod environments

## Proposed Solution

Introduce **Projects** (or **Namespaces**) as a top-level organization unit:

```
Project (tenant)
  └─ Environment (dev/staging/prod)
      └─ Flags
```

Example hierarchy:
```
Project: customer-a
  ├─ dev: 5 flags
  ├─ staging: 10 flags
  └─ prod: 100 flags

Project: customer-b
  ├─ dev: 3 flags
  └─ prod: 80 flags
```

## Concrete Tasks

### Phase 1: Database Schema Changes
- [ ] Create `projects` table:
  ```sql
  CREATE TABLE projects (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    key TEXT UNIQUE NOT NULL,  -- URL-safe: customer-a, analytics-prod
    name TEXT NOT NULL,         -- Display name: Customer A
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
  );
  ```
- [ ] Add `project_id` column to `flags` table:
  ```sql
  ALTER TABLE flags ADD COLUMN project_id UUID REFERENCES projects(id);
  ALTER TABLE flags DROP CONSTRAINT flags_key_key;  -- Remove unique constraint
  CREATE UNIQUE INDEX flags_project_env_key ON flags(project_id, env, key);
  ```
- [ ] Create migration file: `202501160001_add_projects.sql`
- [ ] Add default project for backward compatibility:
  ```sql
  INSERT INTO projects (key, name, description)
  VALUES ('default', 'Default Project', 'Legacy flags before multi-tenancy');
  
  UPDATE flags SET project_id = (SELECT id FROM projects WHERE key = 'default');
  ALTER TABLE flags ALTER COLUMN project_id SET NOT NULL;
  ```

### Phase 2: Backend Project Management
- [ ] Create sqlc queries in `internal/db/queries/projects.sql`:
  ```sql
  -- name: CreateProject :one
  INSERT INTO projects (key, name, description)
  VALUES ($1, $2, $3)
  RETURNING *;
  
  -- name: GetProject :one
  SELECT * FROM projects WHERE key = $1;
  
  -- name: ListProjects :many
  SELECT * FROM projects ORDER BY name;
  
  -- name: UpdateProject :exec
  UPDATE projects SET name = $2, description = $3, updated_at = now()
  WHERE key = $1;
  
  -- name: DeleteProject :exec
  DELETE FROM projects WHERE key = $1;
  ```
- [ ] Update flag queries to include project filtering:
  ```sql
  -- name: GetAllFlags :many
  SELECT * FROM flags WHERE project_id = $1 AND env = $2 ORDER BY key;
  ```
- [ ] Create `internal/api/projects.go` handler:
  ```go
  func (s *Server) handleListProjects(w, r)
  func (s *Server) handleCreateProject(w, r)
  func (s *Server) handleGetProject(w, r)
  func (s *Server) handleUpdateProject(w, r)
  func (s *Server) handleDeleteProject(w, r)
  ```

### Phase 3: API Changes
- [ ] Add project management endpoints:
  ```
  GET    /v1/projects           - List all projects
  POST   /v1/projects           - Create project
  GET    /v1/projects/:key      - Get project details
  PUT    /v1/projects/:key      - Update project
  DELETE /v1/projects/:key      - Delete project (and all flags)
  ```
- [ ] Update flag endpoints to include project:
  ```
  GET  /v1/projects/:project/flags/snapshot?env=prod
  GET  /v1/projects/:project/flags/stream?env=prod
  POST /v1/projects/:project/flags
  DELETE /v1/projects/:project/flags?key=x&env=prod
  ```
- [ ] Maintain backward compatibility:
  ```
  GET /v1/flags/snapshot  →  GET /v1/projects/default/flags/snapshot
  ```
- [ ] Add default project header for backward compat:
  ```
  X-Flagship-Project: customer-a
  # If not provided, use "default"
  ```

### Phase 4: Snapshot per Project
- [ ] Change snapshot storage from single global to per-project:
  ```go
  // Before: single snapshot
  var current unsafe.Pointer  // *Snapshot
  
  // After: map of snapshots by project
  var snapshots sync.Map  // map[projectID]unsafe.Pointer → *Snapshot
  ```
- [ ] Update snapshot functions:
  ```go
  func Load(projectID string) *Snapshot
  func Update(projectID string, snap *Snapshot)
  func Subscribe(projectID string) (chan string, func())
  ```
- [ ] Update SSE to be project-aware:
  ```go
  GET /v1/projects/:project/flags/stream
  ```
- [ ] Rebuild snapshot per project on flag changes

### Phase 5: SDK Changes
- [ ] Update SDK constructor to accept project:
  ```typescript
  const client = new FlagshipClient({
    baseUrl: 'http://localhost:8080',
    project: 'customer-a',  // New required field
    user: { id: 'user-123' }
  });
  ```
- [ ] Update SDK API calls to include project in URL:
  ```typescript
  fetch(`${baseUrl}/v1/projects/${project}/flags/snapshot`)
  ```
- [ ] Default to "default" project for backward compatibility

### Phase 6: Admin UI Updates
- [ ] Add project selector at top of UI:
  ```html
  <select id="project-selector">
    <option value="default">Default Project</option>
    <option value="customer-a">Customer A</option>
    <option value="customer-b">Customer B</option>
  </select>
  ```
- [ ] Add project management page:
  - List all projects
  - Create new project
  - Edit/delete projects
  - View project details (flag count, environments)
- [ ] Update flag list to filter by project
- [ ] Add breadcrumb navigation: `Home > customer-a > prod > flags`

### Phase 7: Access Control (Future Enhancement)
- [ ] (Optional) Add `project_permissions` table:
  ```sql
  CREATE TABLE project_permissions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID REFERENCES projects(id),
    api_key_id UUID REFERENCES api_keys(id),  -- Requires Issue #2
    role TEXT NOT NULL,  -- read, write, admin
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
  );
  ```
- [ ] Update auth middleware to check project permissions
- [ ] Add endpoints to manage permissions per project

### Phase 8: Migration & Documentation
- [ ] Create migration guide for existing users:
  - Explain project concept
  - Show how existing flags move to "default" project
  - Document API changes
  - Provide migration script if needed
- [ ] Update README with project examples
- [ ] Add multi-tenant deployment guide
- [ ] Document project naming conventions (kebab-case, no spaces)

## API Changes

### New Endpoints

**Project Management**
```
GET    /v1/projects                List all projects
POST   /v1/projects                Create project
GET    /v1/projects/:key           Get project details
PUT    /v1/projects/:key           Update project
DELETE /v1/projects/:key           Delete project (cascade delete flags)
```

**Updated Flag Endpoints**
```
GET    /v1/projects/:project/flags/snapshot?env=prod
GET    /v1/projects/:project/flags/stream?env=prod
POST   /v1/projects/:project/flags
DELETE /v1/projects/:project/flags?key=x&env=prod
GET    /v1/projects/:project/flags/evaluate  (if Issue #7 implemented)
```

### Request/Response Examples

**Create Project**
```bash
curl -X POST http://localhost:8080/v1/projects \
  -H "Authorization: Bearer $ADMIN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "key": "customer-a",
    "name": "Customer A",
    "description": "Flags for Customer A"
  }'

# Response
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "key": "customer-a",
  "name": "Customer A",
  "description": "Flags for Customer A",
  "created_at": "2025-01-15T10:30:00Z",
  "updated_at": "2025-01-15T10:30:00Z"
}
```

**List Projects**
```bash
curl http://localhost:8080/v1/projects \
  -H "Authorization: Bearer $ADMIN_API_KEY"

# Response
{
  "projects": [
    {
      "key": "default",
      "name": "Default Project",
      "flag_count": 47
    },
    {
      "key": "customer-a",
      "name": "Customer A",
      "flag_count": 100
    }
  ]
}
```

**Create Flag in Project**
```bash
curl -X POST http://localhost:8080/v1/projects/customer-a/flags \
  -H "Authorization: Bearer $ADMIN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "key": "new_feature",
    "enabled": true,
    "env": "prod",
    "config": {}
  }'
```

### SDK Constructor
```typescript
const client = new FlagshipClient({
  baseUrl: 'http://localhost:8080',
  project: 'customer-a',           // Required
  user: { id: 'user-123' }
});
```

## Acceptance Criteria

### Database
- [ ] `projects` table created with proper indexes
- [ ] `flags` table has `project_id` foreign key
- [ ] Unique constraint on `(project_id, env, key)`
- [ ] Migration handles existing flags (moves to "default" project)
- [ ] Cascading delete works (delete project → delete all flags)

### Backend API
- [ ] All project CRUD endpoints work
- [ ] Flag endpoints include project in URL
- [ ] Backward compatibility: old URLs redirect or use "default" project
- [ ] Snapshot is per-project
- [ ] SSE streams are per-project

### SDK
- [ ] SDK accepts project in constructor
- [ ] SDK calls correct project-specific endpoints
- [ ] SDK defaults to "default" project if not specified

### Admin UI
- [ ] Project selector switches between projects
- [ ] Project management page works (list/create/edit/delete)
- [ ] Flag list filters by selected project
- [ ] Breadcrumb shows current project

### Documentation
- [ ] README explains project concept
- [ ] API documentation updated
- [ ] Migration guide for existing users
- [ ] Multi-tenant deployment guide

## Notes / Risks / Edge Cases

### Risks
- **Breaking Change**: URL structure changes (mitigated by backward compat)
- **Migration Complexity**: Existing flags need to move to "default" project
- **Snapshot Memory**: Multiple projects increase memory usage
  - Mitigation: Lazy load snapshots, evict unused projects
- **Performance**: Queries now need to filter by project
  - Mitigation: Proper indexes on `(project_id, env, key)`

### Edge Cases
- Delete project with many flags (cascade delete, confirm?)
- Project key conflicts (ensure unique constraint)
- Flag key unique per project but can repeat across projects
- Empty project (no flags) - should be allowed
- Rename project key (update all API calls, SDK configs)
- Very long project name (truncate in UI)
- Special characters in project key (validate: alphanumeric + hyphen)

### Backward Compatibility Strategy

**Option A: Redirect** (Recommended)
```
GET /v1/flags/snapshot → 301 /v1/projects/default/flags/snapshot
```

**Option B: Header-Based**
```
GET /v1/flags/snapshot
# Uses X-Flagship-Project header, defaults to "default"
```

**Option C: Dual Support**
```
# Both work:
GET /v1/flags/snapshot              (uses "default")
GET /v1/projects/default/flags/snapshot
```

### Naming: Projects vs Namespaces vs Tenants?

- **Projects**: Good for product-oriented (product-a, product-b)
- **Namespaces**: Good for technical users (team-frontend, team-backend)
- **Tenants**: Good for SaaS (customer-a, customer-b)
- **Recommendation**: Use "Projects" in docs/UI, internally call them tenants

### Performance Considerations
- Snapshot memory: N projects × M environments × K flags
- For 100 projects × 3 envs × 100 flags = 30k flag views in memory
- Each FlagView ~500 bytes = 15MB total (acceptable)
- Consider: Lazy load snapshots (only load when accessed)
- Consider: TTL eviction (evict unused projects after 1 hour)

### Future Enhancements
- Project settings (default env, retention, rate limits)
- Project-level API keys (scoped to single project)
- Cross-project flag copy (copy flag from one project to another)
- Project templates (create new project from template)
- Project quotas (max flags per project)
- Project analytics (usage stats, popular flags)
- Hierarchical projects (parent/child relationships)

## Implementation Hints

- Database migrations are in `internal/db/migrations/`
- sqlc queries in `internal/db/queries/`
- Snapshot logic in `internal/snapshot/snapshot.go`
- API handlers in `internal/api/server.go`
- SDK in `sdk/flagshipClient.ts`
- Admin UI in `sdk/admin.html`
- Example multi-key map in Go:
  ```go
  type snapshotStore struct {
    mu    sync.RWMutex
    byProject map[string]unsafe.Pointer  // projectID → *Snapshot
  }
  
  func (s *snapshotStore) Load(projectID string) *Snapshot {
    s.mu.RLock()
    defer s.mu.RUnlock()
    ptr := s.byProject[projectID]
    if ptr == nil {
      return &Snapshot{...}  // empty snapshot
    }
    return (*Snapshot)(ptr)
  }
  ```

## Labels

`feature`, `backend`, `frontend`, `sdk`, `database`, `breaking-change`

## Estimated Effort

**4-5 days** (significant refactor)
- Day 1: Database schema + migration + backward compat
- Day 2: Backend project CRUD + updated flag queries
- Day 3: Snapshot refactor + per-project SSE
- Day 4: SDK updates + tests
- Day 5: Admin UI updates + documentation + end-to-end testing
