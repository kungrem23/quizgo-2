#!/usr/bin/env python3
"""Reproduce bounded, local audit scenarios using a temporary Go test overlay."""

import json
from pathlib import Path
import subprocess
import sys
import tempfile


def main() -> int:
    evidence = Path(__file__).resolve().parent
    root = evidence.parent.parent
    replacements = {
        "services/quiz/tests/http/auth/security_audit_repro_test.go": "auth_repro_test.go.txt",
        "services/game/tests/application/security_audit_repro_test.go": "application_repro_test.go.txt",
        "services/game/internal/application/security_audit_repro_test.go": "actor_repro_test.go.txt",
    }
    with tempfile.TemporaryDirectory(prefix="quizgo-audit-") as temp:
        overlay = Path(temp) / "overlay.json"
        overlay.write_text(json.dumps({
            "Replace": {
                str(root / virtual): str(evidence / source)
                for virtual, source in replacements.items()
            }
        }))
        result = subprocess.run([
            "go", "test", f"-overlay={overlay}", "-count=1", "-run", "TestAudit", "-v",
            "./services/game/internal/application",
            "./services/quiz/tests/http/auth",
            "./services/game/tests/application",
        ], cwd=root, check=False)
        return result.returncode


if __name__ == "__main__":
    sys.exit(main())
