[2026-03-20 13:51] - Updated by Junie
{
    "TYPE": "correction",
    "CATEGORY": "UI styling",
    "EXPECTATION": "The Logs item should be plain text labeled 'Logs', not a button.",
    "NEW INSTRUCTION": "WHEN adding or updating the Logs sidebar item THEN render plain text label 'Logs' without button styling"
}

[2026-03-20 14:14] - Updated by Junie
{
    "TYPE": "correction",
    "CATEGORY": "Auth requirement for EXE",
    "EXPECTATION": "They want the shared .exe to work for others without requiring Cloudflare login.",
    "NEW INSTRUCTION": "WHEN discussing builds for friends THEN ensure no Cloudflare login is required for end users"
}

[2026-03-20 23:43] - Updated by Junie
{
    "TYPE": "correction",
    "CATEGORY": "Build delivery/update",
    "EXPECTATION": "After installation, the app should reflect the latest changes, not the previous version.",
    "NEW INSTRUCTION": "WHEN delivering a new build THEN increment version and confirm old install is replaced"
}

[2026-03-20 23:51] - Updated by Junie
{
    "TYPE": "correction",
    "CATEGORY": "Build not updated",
    "EXPECTATION": "They want the app rebuilt so the installed version reflects the latest changes.",
    "NEW INSTRUCTION": "WHEN user says build didn't update THEN rebuild, bump version, and verify installer replaces"
}

