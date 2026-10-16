# Complete answer

Preserve this narrative verbatim.

## BuildFlow Structured View

<!-- buildflow:view:start -->
```json
{
  "schema_version": 2,
  "stage": "step-04",
  "scope": "stage",
  "package_id": null,
  "state": "ready",
  "blocks": {
    "assumptions": [],
    "risks": [],
    "open_questions": [],
    "sources": [
      {
        "id": "SRC-1",
        "kind": "user_input",
        "locator": "User use case",
        "checked_on": null,
        "confidence": "verified"
      }
    ],
    "packages": [
      {
        "id": "P01",
        "title": "Save documents",
        "goal": "Atomic saves",
        "why_now": "Foundation",
        "languages": [
          "Go"
        ],
        "module_ids": [
          "M-1"
        ],
        "requirement_ids": [
          "FR-1"
        ],
        "depends_on": [],
        "prerequisite_reasons": [],
        "scope_in": [],
        "scope_out": [],
        "files": [
          {
            "path": "store.go",
            "exists": false,
            "action": "create",
            "purpose": "Atomic saves"
          }
        ],
        "consumed_contract_ids": [
          "C-2"
        ],
        "produced_contract_ids": [
          "C-1"
        ],
        "implementation": {
          "notes": [
            "Compare hashes before replacement."
          ],
          "contract_ids": [
            "C-1",
            "C-2"
          ],
          "algorithms": [],
          "error_cases": [],
          "concurrency": [],
          "state_machines": [],
          "surfaces": [],
          "observability": [],
          "decision_ids": [
            "D-1"
          ]
        },
        "tests": [
          {
            "id": "T-1",
            "name": "TestSave",
            "level": "unit",
            "file": "store_test.go",
            "fixtures": [
              "temporary directory"
            ],
            "assertions": [
              "The original survives failure."
            ],
            "failure_path": true,
            "command": "go test ./...",
            "required_for_acceptance": true,
            "requirement_ids": [
              "FR-1"
            ],
            "contract_ids": [
              "C-1"
            ]
          }
        ],
        "acceptance": [
          {
            "id": "AC-1",
            "statement": "A failed save preserves the original.",
            "requirement_ids": [
              "FR-1"
            ],
            "module_ids": [
              "M-1"
            ],
            "test_ids": [
              "T-1"
            ]
          }
        ],
        "command_keys": [
          "build",
          "test",
          "lint"
        ],
        "source_acceptance_ids": [
          "AC-1"
        ],
        "effort": null,
        "risk": null
      }
    ],
    "build_batches": [
      [
        "P01"
      ]
    ],
    "integration_points": [],
    "deliberately_not_created": []
  }
}
```
<!-- buildflow:view:end -->
