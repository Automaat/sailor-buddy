# Implementation Plan

## Phase 1: Project Scaffolding - DONE
- [x] Monorepo structure (backend/ + frontend/)
- [x] Go module + dependencies (chi, jwt, sqlite, sqlc, excelize)
- [x] SvelteKit 5 + Tailwind CSS
- [x] mise.toml with pinned tool versions
- [x] docker-compose.yml + Dockerfiles
- [x] .air.toml for hot reload
- [x] CLAUDE.md
- [x] GitHub repo + branch protection

## Phase 2: Database Schema - DONE
- [x] 7 migration files (users, refresh_tokens, yachts, crew_members, cruises, crew_assignments, trainings, voyage_opinions)
- [x] Embedded migrations with auto-run
- [x] sqlc queries + generated Go code

## Phase 3: Auth - DONE
- [x] JWT access tokens (15min) + refresh tokens (30d)
- [x] bcrypt password hashing
- [x] Refresh token rotation
- [x] Chi auth middleware
- [x] Frontend auth store (Svelte 5 runes + localStorage)
- [x] Auto-refresh on 401

## Phase 4: Core CRUD + API - DONE
- [x] Auth handlers (register/login/refresh/logout)
- [x] Cruise CRUD handlers
- [x] Yacht CRUD handlers
- [x] Crew member CRUD handlers
- [x] Crew assignment handlers
- [x] Training CRUD handlers
- [x] Dashboard stats handler
- [x] Router with all routes wired

## Phase 5: XLSX Import - DONE
- [x] Upload + parse handler (excelize)
- [x] Preview endpoint
- [x] Confirm endpoint (creates yachts/crew/cruises/trainings)
- [x] Excel serial date conversion

## Phase 6: Frontend Pages - DONE
- [x] Login/register page
- [x] Dashboard (KPI cards + yearly table)
- [x] Cruise list, detail, create, edit
- [x] Crew directory + detail
- [x] Yacht registry
- [x] Training tracker
- [x] XLSX import wizard
- [x] Settings page
- [x] Nav layout with nautical theme

## Phase 7: Remaining Work - TODO
- [ ] Document generation (HTML->PDF via chromedp, DOCX)
- [ ] Voyage opinion templates
- [ ] Image upload endpoint
- [ ] Charts (echarts integration)
- [ ] Cruise edit page crew assignment UI
- [ ] Multi-user sharing (friends view cruises they participated in)
- [ ] E2E testing
- [ ] Production deployment config
