# Complete answer

Preserve this narrative verbatim.

## BuildFlow Structured View

<!-- buildflow:view:start -->
```json
{
  "schema_version": 2, "schema_version": 2,
  "stage": "step-01",
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
    "problem": "Document saves can lose data.",
    "goals": [
      {
        "id": "G-1",
        "outcome": "Safe saves",
        "success_measure": "No lost updates"
      }
    ],
    "non_goals": [],
    "scenarios": [],
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
    "integrations": [],
    "data_state": [],
    "options": [
      {
        "id": "OPT-1",
        "name": "Atomic replace",
        "description": "Documented behavior",
        "advantage": "Documented behavior",
        "disadvantage": "Documented behavior",
        "riskiest_assumption": "Documented behavior",
        "rejection_reason": null,
        "reconsider_when": null
      },
      {
        "id": "OPT-2",
        "name": "Versioned files",
        "description": "Documented behavior",
        "advantage": "Documented behavior",
        "disadvantage": "Documented behavior",
        "riskiest_assumption": "Documented behavior",
        "rejection_reason": null,
        "reconsider_when": null
      },
      {
        "id": "OPT-3",
        "name": "Transactional store",
        "description": "Documented behavior",
        "advantage": "Documented behavior",
        "disadvantage": "Documented behavior",
        "riskiest_assumption": "Documented behavior",
        "rejection_reason": null,
        "reconsider_when": null
      }
    ],
    "evaluation_criteria": [
      {
        "id": "EC-1",
        "criterion": "Reliability",
        "weight_percent": 100,
        "rewards": "Safe saves",
        "penalizes": "Data loss"
      }
    ]
  }
}
```
<!-- buildflow:view:end -->
