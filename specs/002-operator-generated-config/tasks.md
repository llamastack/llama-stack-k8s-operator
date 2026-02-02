# Tasks: Operator-Generated Server Configuration

**Input**: Design documents from `/specs/002-operator-generated-config/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Tests are included as this is a Kubernetes operator requiring E2E validation (Ginkgo/Gomega).

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

Based on plan.md:
- **API Types**: `api/v1alpha1/`
- **Controllers**: `controllers/`
- **Config Generation**: `pkg/configgen/`
- **E2E Tests**: `tests/e2e/`
- **Samples**: `config/samples/`

---

## Phase 1: Setup (Project Initialization)

**Purpose**: Create new files and basic structure for the feature

- [x] T001 Create provider_types.go with base type definitions in api/v1alpha1/provider_types.go
- [x] T002 [P] Create distributions.go with embedded distribution configs in pkg/configgen/distributions.go
- [x] T003 [P] Create condition constants in controllers/conditions.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**CRITICAL**: No user story work can begin until this phase is complete

### CRD Schema Types

- [x] T004 Add SecretKeyRefSource type in api/v1alpha1/provider_types.go
- [x] T005 [P] Add ProviderEntry type with kubebuilder markers in api/v1alpha1/provider_types.go
- [x] T006 [P] Add ProviderConfigOrList polymorphic type with custom JSON marshaling in api/v1alpha1/provider_types.go
- [x] T007 [P] Add ProvidersSpec container type in api/v1alpha1/provider_types.go
- [x] T008 [P] Add StorageConfigSpec type in api/v1alpha1/provider_types.go
- [x] T009 [P] Add ResourcesSpec, ModelEntry, ModelMetadata types in api/v1alpha1/provider_types.go
- [x] T010 [P] Add ServerTLSConfig type in api/v1alpha1/provider_types.go

### ServerSpec Extensions

- [x] T011 Add Providers, Disabled, Storage, Resources, Port, TLS fields to ServerSpec in api/v1alpha1/llamastackdistribution_types.go
- [x] T012 Add CEL validation rule for mutual exclusivity (providers vs userConfig) in api/v1alpha1/llamastackdistribution_types.go
- [x] T013 Add GeneratedConfigMap field to LlamaStackDistributionStatus in api/v1alpha1/llamastackdistribution_types.go

### CEL Validation Rules

- [x] T014 Add CEL validation for bedrock requires region in api/v1alpha1/provider_types.go
- [x] T015 [P] Add CEL validation for azure requires deploymentName in api/v1alpha1/provider_types.go
- [x] T016 [P] Add CEL validation for postgres requires connectionString in api/v1alpha1/provider_types.go
- [x] T017 [P] Add CEL validation for valid disabled provider types in api/v1alpha1/llamastackdistribution_types.go
- [x] T018 [P] Add CEL validation for unique provider IDs in api/v1alpha1/provider_types.go

### Code Generation

- [x] T019 Run make generate to regenerate deep copy methods
- [x] T020 Run make manifests to regenerate CRD YAML in config/crd/bases/

### Config Generator Core

- [x] T021 Create generator.go with Generator struct and Generate method in pkg/configgen/generator.go
- [x] T022 [P] Create provider_mapper.go with provider type mapping logic in pkg/configgen/provider_mapper.go
- [x] T023 [P] Create secret_resolver.go with env var name generation in pkg/configgen/secret_resolver.go
- [x] T024 Implement loadBaseConfig to read embedded distribution config in pkg/configgen/generator.go
- [x] T024a [P] Create unit test for loadBaseConfig distribution config loading in pkg/configgen/generator_test.go
- [x] T025 Implement ConfigMapName with SHA256 hash (10 chars) in pkg/configgen/generator.go

### Controller Status Infrastructure

- [x] T026 Add condition type constants (ValidationSucceeded, SecretsResolved, ConfigReady) in controllers/conditions.go
- [x] T027 [P] Add reason constants (ReasonValidationSucceeded, ReasonSecretNotFound, etc.) in controllers/conditions.go
- [x] T028 [P] Add message constants for status conditions in controllers/conditions.go
- [x] T029 Implement setCondition helper method in controllers/status.go

**Checkpoint**: Foundation ready - CRD types defined, generator core ready, status infrastructure in place

---

## Phase 3: User Story 1 - Simple Provider Configuration (Priority: P1) MVP

**Goal**: Enable users to configure LlamaStack with single inference provider using minimal YAML

**Independent Test**: Deploy LLSD CR with single inference provider and verify generated config.yaml

### Tests for User Story 1

- [x] T030 [P] [US1] Create unit test for single provider expansion in pkg/configgen/generator_test.go
- [x] T031 [P] [US1] Create unit test for endpoint to url mapping in pkg/configgen/provider_mapper_test.go
- [x] T032 [P] [US1] Create unit test for secret reference to env var in pkg/configgen/secret_resolver_test.go
- [ ] T033 [US1] Create E2E test for single provider deployment in tests/e2e/provider_config_test.go

### Implementation for User Story 1

- [x] T034 [US1] Implement applyProviders for single provider case in pkg/configgen/generator.go
- [x] T035 [US1] Implement mapProviderType to add remote:: prefix in pkg/configgen/provider_mapper.go
- [x] T035a [US1] Implement validateProviderType to check against supported provider list in pkg/configgen/provider_mapper.go
- [x] T035b [P] [US1] Create unit test for provider type validation (valid/invalid types) in pkg/configgen/provider_mapper_test.go
- [x] T036 [US1] Implement mapEndpoint to config.url in pkg/configgen/provider_mapper.go
- [x] T037 [US1] Implement resolveSecretRef to generate env var name and value in pkg/configgen/secret_resolver.go
- [x] T037a [US1] Implement validateSecretExists to check secret reference exists in pkg/configgen/secret_resolver.go
- [x] T037b [P] [US1] Create unit test for secret existence validation (found/not found) in pkg/configgen/secret_resolver_test.go
- [x] T038 [US1] Implement generateEnvVars to create deployment env vars in pkg/configgen/secret_resolver.go
- [x] T039 [US1] Add reconcileGeneratedConfig method to controller in controllers/llamastackdistribution_controller.go
- [x] T040 [US1] Implement ConfigMap creation with owner reference in controllers/llamastackdistribution_controller.go
- [x] T041 [US1] Update deployment to mount generated ConfigMap in controllers/resource_helper.go
- [x] T042 [US1] Update deployment to inject secret env vars in controllers/resource_helper.go
- [x] T043 [US1] Update status conditions on successful generation in controllers/llamastackdistribution_controller.go
- [x] T043a [P] [US1] Create unit test verifying error messages include field path in pkg/configgen/generator_test.go
- [x] T044 [US1] Create sample manifest example-minimal-inference.yaml in config/samples/example-minimal-inference.yaml

**Checkpoint**: User Story 1 complete - single provider configuration works end-to-end

---

## Phase 4: User Story 2 - Multiple Providers Configuration (Priority: P2)

**Goal**: Support multiple inference providers with explicit IDs

**Independent Test**: Deploy LLSD CR with list of providers and verify all appear in config.yaml

### Tests for User Story 2

- [x] T045 [P] [US2] Create unit test for multiple provider expansion in pkg/configgen/generator_test.go
- [x] T046 [P] [US2] Create unit test for provider ID uniqueness validation in pkg/configgen/generator_test.go
- [ ] T047 [US2] Create E2E test for multiple providers deployment in tests/e2e/provider_config_test.go

### Implementation for User Story 2

- [x] T048 [US2] Extend applyProviders to handle list form in pkg/configgen/generator.go
- [x] T049 [US2] Implement validateProviderIDs for uniqueness check in pkg/configgen/generator.go
- [x] T050 [US2] Add validation error for missing ID in list form in pkg/configgen/generator.go
- [x] T051 [US2] Create sample manifest example-multiple-providers.yaml in config/samples/example-multiple-providers.yaml

**Checkpoint**: User Story 2 complete - multiple providers with explicit IDs work

---

## Phase 5: User Story 3 - Per-Provider Custom Configuration (Priority: P2)

**Goal**: Support pass-through config field for provider-specific settings

**Independent Test**: Deploy LLSD CR with config.* fields and verify they appear in generated config.yaml

### Tests for User Story 3

- [x] T052 [P] [US3] Create unit test for config pass-through in pkg/configgen/generator_test.go
- [ ] T053 [US3] Create E2E test for custom config fields in tests/e2e/provider_config_test.go

### Implementation for User Story 3

- [x] T054 [US3] Implement applyConfig to merge RawExtension fields in pkg/configgen/generator.go
- [x] T055 [US3] Ensure simplified fields override config fields (precedence) in pkg/configgen/generator.go
- [x] T056 [US3] Create sample manifest example-custom-config.yaml in config/samples/example-custom-config.yaml

**Checkpoint**: User Story 3 complete - custom config pass-through works

---

## Phase 6: User Story 4 - Resource Registration (Priority: P2)

**Goal**: Support models, tools, and shields registration

**Independent Test**: Deploy LLSD CR with resources and verify registered_resources section in config.yaml

### Tests for User Story 4

- [x] T057 [P] [US4] Create unit test for simple model string expansion in pkg/configgen/generator_test.go
- [x] T058 [P] [US4] Create unit test for detailed model with provider in pkg/configgen/generator_test.go
- [x] T059 [P] [US4] Create unit test for tools and shields mapping in pkg/configgen/generator_test.go
- [ ] T060 [US4] Create E2E test for resource registration in tests/e2e/provider_config_test.go

### Implementation for User Story 4

- [x] T061 [US4] Implement applyResources method in pkg/configgen/generator.go
- [x] T062 [US4] Implement simple model string to first provider mapping in pkg/configgen/generator.go
- [x] T063 [US4] Implement detailed model with explicit provider in pkg/configgen/generator.go
- [x] T064 [US4] Implement tools to tool_groups mapping in pkg/configgen/generator.go
- [x] T065 [US4] Implement shields mapping in pkg/configgen/generator.go
- [x] T066 [US4] Create sample manifest example-with-resources.yaml in config/samples/example-with-resources.yaml

**Checkpoint**: User Story 4 complete - resource registration works

---

## Phase 7: User Story 5 - Disable Providers/APIs (Priority: P3)

**Goal**: Allow disabling provider types to reduce attack surface

**Independent Test**: Deploy LLSD CR with disabled list and verify providers are removed from config.yaml

### Tests for User Story 5

- [x] T067 [P] [US5] Create unit test for disabled provider removal in pkg/configgen/generator_test.go
- [x] T068 [P] [US5] Create unit test for warning when disabled not in distribution in pkg/configgen/generator_test.go
- [ ] T069 [US5] Create E2E test for disabled providers in tests/e2e/provider_config_test.go

### Implementation for User Story 5

- [x] T070 [US5] Implement removeDisabled method in pkg/configgen/generator.go
- [x] T071 [US5] Add logging for warning when disabled type not in distribution in pkg/configgen/generator.go
- [x] T072 [US5] Create sample manifest example-with-disabled.yaml in config/samples/example-with-disabled.yaml

**Checkpoint**: User Story 5 complete - provider disabling works

---

## Phase 8: User Story 6 - ConfigMap Escape Hatch (Priority: P3)

**Goal**: Maintain backward compatibility with full ConfigMap approach

**Independent Test**: Deploy LLSD CR with userConfig.configMapName and verify operator uses it directly

### Tests for User Story 6

- [ ] T073 [P] [US6] Create unit test for userConfig precedence in controllers/llamastackdistribution_controller_test.go
- [ ] T074 [P] [US6] Create unit test for mutual exclusivity error in controllers/llamastackdistribution_controller_test.go
- [ ] T075 [US6] Create E2E test for ConfigMap escape hatch in tests/e2e/provider_config_test.go

### Implementation for User Story 6

- [x] T076 [US6] Add skip logic in reconcileGeneratedConfig when userConfig is set in controllers/llamastackdistribution_controller.go
- [x] T077 [US6] Verify CEL validation prevents providers+userConfig in E2E test in tests/e2e/provider_config_test.go

**Checkpoint**: User Story 6 complete - escape hatch works, mutual exclusivity enforced

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Integration, documentation, and final verification

### Storage Configuration

- [x] T078 [P] Implement applyStorage for sqlite/postgres in pkg/configgen/generator.go
- [x] T079 [P] Create unit test for storage configuration in pkg/configgen/generator_test.go

### Server Settings

- [x] T080 [P] Implement port and TLS config application in pkg/configgen/generator.go
- [x] T081 [P] Create unit test for server settings in pkg/configgen/generator_test.go

### Integration with Spec 001 (External Providers)

- [ ] T082 Implement config merge point for external providers in controllers/llamastackdistribution_controller.go
- [ ] T083 Add warning for provider ID collision in controllers/llamastackdistribution_controller.go
- [ ] T084 Create E2E test for external provider merge in tests/e2e/provider_config_test.go

### Documentation

- [x] T085 [P] Create comprehensive sample example-with-providers.yaml in config/samples/example-with-providers.yaml
- [ ] T086 [P] Update README with operator-generated config section
- [ ] T087 Create migration guide from ConfigMap to providers schema

### Edge Case Handling

- [ ] T088 [P] Add config.yaml version detection and warning for unsupported versions in pkg/configgen/generator.go
- [ ] T089 [P] Create unit test for schema version mismatch warning in pkg/configgen/generator_test.go

### Final Validation

- [x] T090 Run make test to verify all unit tests pass
- [ ] T091 Run make e2e to verify all E2E tests pass
- [ ] T092 Run quickstart.md verification checklist
- [ ] T093 Verify CRD validation rejects invalid configs manually

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3-8)**: All depend on Foundational phase completion
  - User stories can then proceed in parallel (if staffed)
  - Or sequentially in priority order (P1, P2, P2, P2, P3, P3)
- **Polish (Phase 9)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - No dependencies on other stories
- **User Story 2 (P2)**: Can start after Foundational (Phase 2) - Extends US1 provider handling
- **User Story 3 (P2)**: Can start after Foundational (Phase 2) - Independent of US1/US2
- **User Story 4 (P2)**: Can start after Foundational (Phase 2) - Independent of US1/US2/US3
- **User Story 5 (P3)**: Can start after Foundational (Phase 2) - Independent
- **User Story 6 (P3)**: Can start after Foundational (Phase 2) - Independent

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Models/types before generator logic
- Generator logic before controller integration
- Controller integration before E2E tests pass
- Story complete before moving to next priority

### Parallel Opportunities

- All Setup tasks (T001-T003) can run in parallel
- All type definitions (T004-T010) can run in parallel
- All CEL validations (T014-T018) can run in parallel
- All unit tests within a story marked [P] can run in parallel
- Different user stories can be worked on in parallel by different team members

---

## Parallel Example: User Story 1

```bash
# Launch all unit tests for User Story 1 together:
Task: "T030 Create unit test for single provider expansion"
Task: "T031 Create unit test for endpoint to url mapping"
Task: "T032 Create unit test for secret reference to env var"

# Then run implementation in dependency order:
Task: "T034 Implement applyProviders for single provider case"
Task: "T035-T036 Implement provider mapping functions" (parallel)
Task: "T037-T038 Implement secret resolution" (parallel)
Task: "T039-T043 Controller integration" (sequential)
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Test single provider config independently
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational -> Foundation ready
2. Add User Story 1 -> Test independently -> Deploy/Demo (MVP!)
3. Add User Story 2 -> Test independently -> Multiple providers work
4. Add User Story 3 -> Test independently -> Custom config works
5. Add User Story 4 -> Test independently -> Resources work
6. Add User Story 5 -> Test independently -> Disable feature works
7. Add User Story 6 -> Test independently -> Escape hatch preserved
8. Polish phase -> Integration, docs, final verification

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1 (MVP - highest priority)
   - Developer B: User Story 4 (resources - independent)
   - Developer C: User Story 5 (disabled - independent)
3. After US1 complete:
   - Developer A: User Story 2 (extends US1)
   - Developer B: User Story 3 (config pass-through)
   - Developer C: User Story 6 (escape hatch)
4. Stories complete and integrate independently

---

## Summary

| Phase | Task Count | Parallel Tasks |
|-------|------------|----------------|
| Setup | 3 | 2 |
| Foundational | 27 | 16 |
| User Story 1 | 21 | 8 |
| User Story 2 | 7 | 2 |
| User Story 3 | 5 | 1 |
| User Story 4 | 10 | 3 |
| User Story 5 | 6 | 2 |
| User Story 6 | 5 | 2 |
| Polish | 16 | 6 |
| **Total** | **100** | **42** |

**MVP Scope**: Phase 1 + Phase 2 + Phase 3 = 51 tasks

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify tests fail before implementing
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Run `make generate manifests` after any type changes
