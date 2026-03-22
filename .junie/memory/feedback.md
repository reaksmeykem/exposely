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

[2026-03-21 10:39] - Updated by Junie
{
    "TYPE": "correction",
    "CATEGORY": "File deletion verification",
    "EXPECTATION": "They want generate_keys.go, sign_License.go, and verify_license.go actually removed and verified gone.",
    "NEW INSTRUCTION": "WHEN user reports files still exist THEN re-delete with exact names and verify by listing directory"
}

[2026-03-21 10:50] - Updated by Junie
{
    "TYPE": "correction",
    "CATEGORY": "Logo update failure",
    "EXPECTATION": "After updating the logo and rebuilding, the built app should display the new logo.",
    "NEW INSTRUCTION": "WHEN updating app logo or icon THEN clean and rebuild, bump version, and verify in exe"
}

[2026-03-21 10:51] - Updated by Junie
{
    "TYPE": "correction",
    "CATEGORY": "Logo still old",
    "EXPECTATION": "They want the built/installed app to actually show the new logo, not the old one.",
    "NEW INSTRUCTION": "WHEN user reports logo still old THEN clear icon cache, uninstall old app, rebuild, reinstall, verify logo"
}

[2026-03-21 11:08] - Updated by Junie
{
    "TYPE": "correction",
    "CATEGORY": "Windows installer packaging",
    "EXPECTATION": "They expect a proper Windows installer, not just a standalone .exe.",
    "NEW INSTRUCTION": "WHEN providing Windows build THEN deliver NSIS installer .exe with Start Menu and Uninstall"
}

[2026-03-22 16:56] - Updated by Junie
{
    "TYPE": "negative",
    "CATEGORY": "HTML project failure",
    "EXPECTATION": "The HTML Project option should successfully serve the folder and be reachable via the Cloudflare tunnel without errors.",
    "NEW INSTRUCTION": "WHEN user reports HTML project not working THEN collect logs and verify server and tunnel"
}

[2026-03-22 16:58] - Updated by Junie
{
    "TYPE": "negative",
    "CATEGORY": "HTML project unreachable",
    "EXPECTATION": "The HTML Project should be accessible via the provided Cloudflare URL without errors.",
    "NEW INSTRUCTION": "WHEN user says site can't be reached THEN collect logs, verify local server and tunnel health"
}

[2026-03-22 17:03] - Updated by Junie
{
    "TYPE": "negative",
    "CATEGORY": "Tunnel origin unreachable",
    "EXPECTATION": "The HTML project should be reachable via the Cloudflare tunnel with the origin responding.",
    "NEW INSTRUCTION": "WHEN origin unreachable error and cloudflared not in PATH THEN instruct install cloudflared and verify"
}

[2026-03-22 17:43] - Updated by Junie
{
    "TYPE": "negative",
    "CATEGORY": "HTML project still failing",
    "EXPECTATION": "They want the HTML project to run locally and be accessible via the Cloudflare tunnel without errors.",
    "NEW INSTRUCTION": "WHEN user says HTML project still not running THEN ask for logs, verify local server, confirm cloudflared installed"
}

[2026-03-22 17:50] - Updated by Junie
{
    "TYPE": "negative",
    "CATEGORY": "HTML vs Laravel behavior",
    "EXPECTATION": "The HTML Project should run and be accessible just like the Laravel project via the tunnel.",
    "NEW INSTRUCTION": "WHEN HTML project not running like Laravel THEN match Laravel host/port/tunnel args and request logs"
}

