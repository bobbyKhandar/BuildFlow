# Complete answer

Preserve this narrative verbatim.

## BuildFlow Structured View

<!-- buildflow:view:start -->
```json
{
  "schema_version": 2,
  "stage": "step-02-5",
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
    "components": [
      {
        "id": "CMP-1",
        "name": "Store",
        "responsibilities": [
          "Save documents"
        ],
        "owns": [
          "Documents"
        ]
      }
    ],
    "contracts": [
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
      },
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
    "technology": {
      "stack": [
        {
          "id": "ST-1",
          "applies_to": {
            "scope": "global",
            "requirement_ids": [],
            "component_ids": [],
            "module_ids": [],
            "contract_ids": []
          },
          "layer": "Backend",
          "technology": "Go",
          "version": "1.24",
          "rationale": "Documented behavior",
          "runner_up": null,
          "cost_note": "Documented behavior",
          "basis": {
            "confidence": "verified",
            "source_ids": [
              "SRC-1"
            ],
            "assumption_ids": []
          }
        }
      ],
      "tools": [],
      "libraries": [],
      "services": [],
      "environment": {
        "prerequisites": [
          {
            "id": "ENV-prerequisites",
            "text": "Use Go 1.24.",
            "applies_to": {
              "scope": "global",
              "requirement_ids": [],
              "component_ids": [],
              "module_ids": [],
              "contract_ids": []
            }
          }
        ],
        "installation": [
          {
            "id": "ENV-installation",
            "text": "Use Go 1.24.",
            "applies_to": {
              "scope": "global",
              "requirement_ids": [],
              "component_ids": [],
              "module_ids": [],
              "contract_ids": []
            }
          }
        ],
        "version_pinning": [
          {
            "id": "ENV-version_pinning",
            "text": "Use Go 1.24.",
            "applies_to": {
              "scope": "global",
              "requirement_ids": [],
              "component_ids": [],
              "module_ids": [],
              "contract_ids": []
            }
          }
        ],
        "clean_machine_setup": [
          {
            "id": "ENV-clean_machine_setup",
            "text": "Use Go 1.24.",
            "applies_to": {
              "scope": "global",
              "requirement_ids": [],
              "component_ids": [],
              "module_ids": [],
              "contract_ids": []
            }
          }
        ]
      },
      "commands": {
        "build": "go build ./...",
        "test": "go test ./...",
        "lint": "go vet ./...",
        "run": null,
        "release": null
      },
      "security": {
        "secrets": [],
        "dependency_updates": [],
        "vulnerability_scanning": [],
        "data_handling": [],
        "compliance": []
      },
      "observability": []
    },
    "system_context": {
      "mission": {
        "purpose": "Save documents safely.",
        "must_never_do": [
          "Lose a newer document."
        ]
      },
      "repository_layout": [],
      "public_contract_ids": [
        "C-1"
      ],
      "conventions": {
        "naming": [],
        "file_layout": [],
        "errors": [],
        "logging": [],
        "comments": [],
        "time": [],
        "paths": [],
        "configuration": []
      },
      "patterns": [],
      "anti_patterns": [],
      "invariants": [
        {
          "id": "INV-1",
          "rule": "Compare the expected hash.",
          "enforcement": "Before replacement.",
          "applies_to": {
            "scope": "linked",
            "requirement_ids": [
              "FR-1"
            ],
            "component_ids": [],
            "module_ids": [],
            "contract_ids": []
          }
        }
      ],
      "testing_standards": {
        "frameworks": [],
        "levels": [],
        "naming": [],
        "acceptance_bar": []
      },
      "glossary": [],
      "budgets": [],
      "extension_recipe": []
    },
    "costs": {
      "currency": null,
      "build_areas": [],
      "build_total": null,
      "contingency_percent": 30,
      "contingency_person_days": null,
      "total_with_contingency_person_days": null,
      "monthly": [],
      "scaling_triggers": [],
      "ten_times_load": {
        "bottlenecks": [],
        "changes_required": []
      }
    },
    "rejected_technologies": []
  }
}
```
<!-- buildflow:view:end -->
