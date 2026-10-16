# Complete answer

Preserve this narrative verbatim.

## BuildFlow Structured View

<!-- buildflow:view:start -->
```json
{
  "schema_version": 2,
  "stage": "step-02",
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
    "decision": {
      "applies_to": {
        "scope": "global",
        "requirement_ids": [],
        "component_ids": [],
        "module_ids": [],
        "contract_ids": []
      },
      "id": "D-1",
      "choice": "Atomic replace",
      "reasons": [],
      "tradeoffs": [],
      "revisit_when": "Documented behavior",
      "basis": {
        "confidence": "verified",
        "source_ids": [
          "SRC-1"
        ],
        "assumption_ids": []
      }
    },
    "alternatives": [],
    "requirements": [
      {
        "id": "FR-1",
        "kind": "functional",
        "area": "Storage",
        "statement": "Save documents atomically.",
        "measurable_target": null,
        "rationale": "Prevent data loss.",
        "basis": {
          "confidence": "verified",
          "source_ids": [
            "SRC-1"
          ],
          "assumption_ids": []
        }
      }
    ],
    "constraints": [
      {
        "applies_to": {
          "scope": "global",
          "requirement_ids": [],
          "component_ids": [],
          "module_ids": [],
          "contract_ids": []
        },
        "id": "CON-1",
        "statement": "Use Go.",
        "strength": "hard",
        "negotiable": false,
        "basis": {
          "confidence": "verified",
          "source_ids": [
            "SRC-1"
          ],
          "assumption_ids": []
        }
      }
    ],
    "design": {
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
      "interactions": [],
      "entities": [],
      "key_flows": [],
      "interfaces": [
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
      "error_handling": [],
      "security": {
        "threats": [],
        "controls": [],
        "sensitive_data": [],
        "trust_boundaries": []
      },
      "data_policies": {
        "ownership": [],
        "retention": [],
        "privacy": []
      }
    },
    "scope": {
      "included": [],
      "deferred": [],
      "never": []
    },
    "delivery_plan": [],
    "settled_decisions": []
  }
}
```
<!-- buildflow:view:end -->
