finding_id: J-C02
status: done
files_changed:
  - cmd/dndnd/main.go
  - cmd/dndnd/auth_open5e_cache_test.go
  - cmd/dndnd/auth_open5e_search_test.go
test_command_that_validates: go test ./cmd/dndnd/ -run "TestOpen5e(SearchGET_JC02|CachePOST_F14)" -v
acceptance_criterion_met: yes
notes: Moved Open5e search routes (GET /api/open5e/monsters, GET /api/open5e/spells) from the public router to the dmAuthMw-protected group in main.go. The RegisterPublicRoutes call was removed from the bare router and added to the existing dmAuthMw group alongside RegisterProtectedRoutes. Updated the existing F-14 test to assert GET routes are now blocked, and added a new J-C02 test that explicitly verifies unauthenticated GET requests return 401. Both `make test` and `make cover-check` pass.
follow_ups:
  - Consider renaming RegisterPublicRoutes to RegisterSearchRoutes since it is no longer public
  - Frontend (Svelte SPA) must now include auth credentials when calling /api/open5e/monsters and /api/open5e/spells
