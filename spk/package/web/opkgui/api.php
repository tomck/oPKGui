<?php
// Read-only opkg query endpoint. `$action` only ever selects a key into a
// hardcoded command table below -- it is never concatenated into a shell
// string, so there is no injection surface to sanitize against.

header('Content-Type: application/json');

const OPKG_BIN = '/opt/bin/opkg';

$commands = [
    'installed'  => OPKG_BIN . ' list-installed',
    'upgradable' => OPKG_BIN . ' list-upgradable',
];

$action = $_GET['action'] ?? '';

if (!array_key_exists($action, $commands)) {
    http_response_code(400);
    echo json_encode(['error' => 'invalid action']);
    exit;
}

if (!is_executable(OPKG_BIN)) {
    http_response_code(503);
    echo json_encode(['error' => 'opkg not found at ' . OPKG_BIN . ' -- is Entware installed?']);
    exit;
}

exec($commands[$action] . ' 2>&1', $output, $exit_code);

if ($exit_code !== 0) {
    http_response_code(500);
    echo json_encode(['error' => 'opkg exited with status ' . $exit_code, 'output' => $output]);
    exit;
}

// `opkg list-installed` lines look like: "name - version"
// `opkg list-upgradable` lines look like: "name - old_version - new_version"
$packages = [];
foreach ($output as $line) {
    $parts = array_map('trim', explode(' - ', $line));
    if (count($parts) < 2) {
        continue;
    }
    $packages[] = [
        'name'        => $parts[0],
        'version'     => $parts[1],
        'new_version' => $parts[2] ?? null,
    ];
}

echo json_encode(['packages' => $packages]);
