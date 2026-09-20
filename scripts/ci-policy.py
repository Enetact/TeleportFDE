"""Select CI work from GitHub event data without API calls or write permissions."""

import json
import os
from pathlib import Path


def select_checks(event_name, ref, event):
    """Keep main/tag validation full and make PR validation explicitly opt-in."""
    if event_name == "pull_request":
        action = event.get("action", "")
        if action in ("labeled", "unlabeled"):
            if event.get("label", {}).get("name") != "ci:full":
                return False, False, "Unrelated label edit; existing CI is unchanged."
        if action not in ("opened", "synchronize", "reopened", "labeled", "unlabeled"):
            return False, False, "No PR checks for this activity; merges are tested by the main push."
        labels = {label["name"] for label in event.get("pull_request", {}).get("labels", [])}
        enabled = "ci:full" in labels
        reason = ("Full PR checks enabled by ci:full." if enabled else
                  "PR checks are off. Add the ci:full label in the PR sidebar to enable them.")
        return enabled, enabled, reason

    if event_name not in ("push", "workflow_dispatch"):
        raise ValueError(f"Unsupported CI event: {event_name}")
    if event_name == "push" and event.get("deleted", False):
        return False, False, "Deleted ref; no source to validate."
    if ref == "refs/heads/main" or ref.startswith("refs/tags/v"):
        return True, True, "Full validation for main or a version tag."
    if event_name == "workflow_dispatch":
        # GitHub's event JSON may encode workflow inputs as strings or booleans.
        full = event.get("inputs", {}).get("cluster_tests", False) in (True, "true")
        return True, full, "Manual full validation." if full else "Manual source-quality checks."
    return True, False, "Branch push: source-quality checks without container or KIND jobs."


def main():
    event = json.loads(Path(os.environ["GITHUB_EVENT_PATH"]).read_text(encoding="utf-8"))
    quality, full, reason = select_checks(
        os.environ["GITHUB_EVENT_NAME"], os.environ["GITHUB_REF"], event
    )
    with Path(os.environ["GITHUB_OUTPUT"]).open("a", encoding="utf-8") as output:
        output.write(f"quality={str(quality).lower()}\nfull={str(full).lower()}\n")
    with Path(os.environ["GITHUB_STEP_SUMMARY"]).open("a", encoding="utf-8") as summary:
        summary.write(f"## CI selection\n\n{reason}\n\n")
        summary.write(f"- Source quality and vulnerability scan: {'on' if quality else 'off'}\n")
        summary.write(f"- Container tests/build and KIND levels 1–5: {'on' if full else 'off'}\n")
        summary.write(f"- Rollout probes for levels 3–5: {'on' if full else 'off'}\n")
    print(reason)


if __name__ == "__main__":
    main()
