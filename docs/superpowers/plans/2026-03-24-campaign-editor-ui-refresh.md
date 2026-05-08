# Campaign Editor UI Refresh Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refresh the `webspa-admin` campaign editor so it is easier to scan, safer to operate, and visually stronger without changing the underlying campaign data contract.

**Architecture:** Keep the existing Angular reactive form and service APIs, but reorganize the editor into a clearer page shell with a stronger header, grouped advanced sections, and a sticky operator action rail. Add route-leave protection for unsaved changes and a small shared-shell polish in the default header/layout so the campaign workspace feels like a first-class admin tool.

**Tech Stack:** Angular 18, Angular Material, CoreUI, SCSS, Jasmine/Karma

---

### Task 1: Add editor-safety regression coverage

**Files:**
- Create: `frontend/webspa-admin/src/app/features/campaign/campaign-form/campaign-form.component.spec.ts`

- [ ] **Step 1: Write failing tests for unsaved-change protection**
- [ ] **Step 2: Run the campaign form spec to confirm the new expectations fail**
  Run: `npm test -- --watch=false --include src/app/features/campaign/campaign-form/campaign-form.component.spec.ts`
- [ ] **Step 3: Implement the minimal component/guard behavior to satisfy the tests**
- [ ] **Step 4: Re-run the focused spec until green**

### Task 2: Rebuild the campaign editor workflow

**Files:**
- Modify: `frontend/webspa-admin/src/app/features/campaign/campaign-form/campaign-form.component.ts`
- Modify: `frontend/webspa-admin/src/app/features/campaign/campaign-form/campaign-form.component.html`
- Modify: `frontend/webspa-admin/src/app/features/campaign/campaign-form/campaign-form.component.scss`
- Modify: `frontend/webspa-admin/src/app/features/campaign/campaign-routing.module.ts`
- Create: `frontend/webspa-admin/src/app/core/guards/pending-changes.guard.ts`

- [ ] **Step 1: Add the route-leave protection and beforeunload handling**
- [ ] **Step 2: Restructure the form into a wider two-column editor layout with a sticky operator rail**
- [ ] **Step 3: Break the advanced section into clearer subsections and improve helper copy, URL input semantics, and icon-button accessibility**
- [ ] **Step 4: Keep mobile behavior intact with responsive action placement**

### Task 3: Polish the shared admin shell

**Files:**
- Modify: `frontend/webspa-admin/src/app/layout/default-layout/default-layout.component.html`
- Modify: `frontend/webspa-admin/src/app/layout/default-layout/default-layout.component.scss`
- Modify: `frontend/webspa-admin/src/app/layout/default-layout/default-header/default-header.component.html`
- Modify: `frontend/webspa-admin/src/app/layout/default-layout/default-header/default-header.component.scss`

- [ ] **Step 1: Strengthen the default header spacing and breadcrumb bar**
- [ ] **Step 2: Widen the main content frame for admin workspaces**
- [ ] **Step 3: Verify the shell changes stay compatible with the rest of the admin app**

### Task 4: Validate and hand off

**Files:**
- Modify: `docs/agent/AgentMD.md` (only if a concrete repo-specific discovery would save time later)

- [ ] **Step 1: Run the focused spec**
  Run: `npm test -- --watch=false --include src/app/features/campaign/campaign-form/campaign-form.component.spec.ts`
- [ ] **Step 2: Run a production build**
  Run: `npm run build`
- [ ] **Step 3: Review the changed UI files against the web interface guidelines**
- [ ] **Step 4: Summarize files changed, commands run, evidence, and residual risks**
