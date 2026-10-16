# Complete answer

Preserve this narrative verbatim.

## BuildFlow Structured View

<!-- buildflow:view:start -->
```json
{
  "schema_version": 2,
  "stage": "step-05",
  "scope": "package",
  "package_id": "P01",
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
    "target": {
      "id": "P01",
      "title": "Save documents",
      "goal": "Atomic saves"
    },
    "scope": {
      "included": [],
      "excluded": []
    },
    "files": [
      {
        "path": "store.go",
        "exists": false,
        "action": "create",
        "purpose": "Atomic saves"
      }
    ],
    "interfaces_consumed": [
      {
        "id": "C-2",
        "owner": {
          "kind": "component",
          "id": "CMP-1"
        },
        "referenced_contract_ids": [],
        "kind": "type",
        "name": "Document",
        "language": "Go",
        "declaration": "type Document struct { Data []byte }",
        "input_fields": [],
        "output_fields": [],
        "version": null,
        "stability": "stable",
        "failure_modes": [],
        "configuration": null
      }
    ],
    "interfaces_produced": [
      {
        "id": "C-1",
        "owner": {
          "kind": "component",
          "id": "CMP-1"
        },
        "referenced_contract_ids": [
          "C-2"
        ],
        "kind": "function",
        "name": "Save",
        "language": "Go",
        "declaration": "func Save(path string, data []byte) error\n// Preserve {{literal}}, `ticks`, and Unicode: café.",
        "input_fields": [],
        "output_fields": [],
        "version": null,
        "stability": "stable",
        "failure_modes": [
          "Return I/O errors."
        ],
        "configuration": null
      }
    ],
    "implementation_instructions": [
      {
        "order": 1,
        "files": [
          "store.go"
        ],
        "instruction": "Implement hash-checked atomic replacement."
      }
    ],
    "planned_checks": [
      {
        "kind": "build",
        "command": "go build ./...",
        "expected_result": "Compilation succeeds.",
        "source_test_ids": []
      },
      {
        "kind": "test",
        "command": "go test ./...",
        "expected_result": "Original survives failure.",
        "source_test_ids": [
          "T-1"
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
    "blockers": [],
    "coding_prompt": "Implement P01: save documents atomically in Go 1.24. Compare hashes before replacement. Create store.go and TestSave in store_test.go. Preserve originals on failure. Run go build ./..., go vet ./..., and go test ./...; report actual results and unfinished work."
  }
}
```
<!-- buildflow:view:end -->
