    (function () {
      'use strict';
      // ---------------------------------------------------------------------------
      // Components. Each one sits in a zone (machine, services, auth0) at a column
      // and a row of a virtual grid. A step shows only the components it uses, and
      // the empty columns and rows are squeezed out when the step is drawn.
      // trust: trusted | untrusted | neutral. parent: the CLI box a check lives in.
      // ---------------------------------------------------------------------------
      var NODES = {
        user: { name: 'User / Agent', sub: 'person or AI agent', trust: 'neutral', zone: 'machine', col: 0, row: 0 },
        project: { name: 'Plugin project', sub: 'plugin.json + build', trust: 'untrusted', zone: 'machine', col: 0, row: 1 },
        cli: { name: 'auth0 CLI', trust: 'trusted', zone: 'machine', col: 1, container: true },
        cache: { name: 'Registry cache', sub: 'index.json + .sig', trust: 'trusted', zone: 'machine', col: 2, row: 0 },
        approvals: { name: 'Approvals file', sub: 'approvals.json', trust: 'trusted', zone: 'machine', col: 2, row: 1 },
        store: { name: 'Store + lockfile', sub: 'store/ + plugins.lock.json', trust: 'trusted', zone: 'machine', col: 2, row: 2 },
        devlinks: { name: 'Dev links file', sub: 'dev-links.json', trust: 'trusted', zone: 'machine', col: 2, row: 3 },
        proxy: { name: 'Loopback proxy', sub: 'per-run cert + secret', trust: 'trusted', zone: 'machine', col: 2, row: 0 },
        plugin: { name: 'Plugin process', sub: 'untrusted, runs as you', trust: 'untrusted', zone: 'machine', col: 2, row: 1 },
        repo: { name: 'Plugin repo', sub: 'GitHub', trust: 'untrusted', zone: 'services', col: 3, row: 1 },
        actions: { name: 'Release workflow', sub: 'GitHub Actions', trust: 'untrusted', zone: 'services', col: 4, row: 1 },
        release: { name: 'GitHub release', sub: 'archives + checksums', trust: 'untrusted', zone: 'services', col: 5, row: 1 },
        registry: { name: 'Signed registry', sub: 'index.json + .sig', trust: 'trusted', zone: 'services', col: 5, row: 0 },
        mgmt: { name: 'Management API', sub: 'acme.us.auth0.com', trust: 'neutral', zone: 'auth0', col: 6, row: 0 },
        'c-prescan': { name: 'Pre-scan flags', parent: 'cli', trust: 'trusted', chip: true },
        'c-verify': { name: 'Verify binary', parent: 'cli', trust: 'trusted', chip: true },
        'c-registry': { name: 'Registry check', parent: 'cli', trust: 'trusted', chip: true },
        'c-call': { name: 'Call check', parent: 'cli', trust: 'trusted', chip: true },
        'c-approval': { name: 'Approval check', parent: 'cli', trust: 'trusted', chip: true },
        'c-session': { name: 'Session / login', parent: 'cli', trust: 'trusted', chip: true },
        'c-proxy': { name: 'Proxy start', parent: 'cli', trust: 'trusted', chip: true },
        'c-spawn': { name: 'Spawn plugin', parent: 'cli', trust: 'trusted', chip: true },
        'c-consent': { name: 'Consent screen', parent: 'cli', trust: 'trusted', chip: true },
        'c-scaffold': { name: 'Scaffold project', parent: 'cli', trust: 'trusted', chip: true },
        'c-manifest': { name: 'Manifest check', parent: 'cli', trust: 'trusted', chip: true },
        'c-link': { name: 'Link / unlink', parent: 'cli', trust: 'trusted', chip: true }
      };
      var ZONE_ORDER = ['machine', 'services', 'auth0'];
      var ZONE_LABEL = { machine: 'Your machine', services: 'Trusted services', auth0: 'Auth0' };
      // Links between components. Only the ones whose two ends are both on screen are drawn.
      var EDGES = [
        ['repo', 'actions'], ['actions', 'release'], ['project', 'repo'], ['cli', 'registry'], ['cli', 'release'],
        ['user', 'cli'], ['user', 'project'], ['cli', 'project'], ['cli', 'devlinks'], ['cli', 'cache'],
        ['cli', 'store'], ['cli', 'approvals'], ['cli', 'proxy'], ['cli', 'plugin'], ['proxy', 'plugin'], ['proxy', 'mgmt']
      ];
      // ---------------------------------------------------------------------------
      // Helpers for the data boxes. A line is a string or {k, t}.
      // ---------------------------------------------------------------------------
      function ok(t) { return { k: 'ok', t: t }; }
      function no(t) { return { k: 'no', t: t }; }
      function del(t) { return { k: 'del', t: t }; }
      function add(t) { return { k: 'add', t: t }; }
      function hl(t) { return { k: 'hl', t: t }; }
      function note(t) { return { k: 'note', t: t }; }
      var TENANT = 'acme.us.auth0.com';
      var OTHER = 'example.us.auth0.com';
      var APPROVE = 'auth0 plugin approve auth0/checkmate --tenant ';
      var RUN_CMD = 'auth0 checkmate --validators a --tenant ' + TENANT;
      var DEV = 'AUTH0_CLI_PLUGIN_DEV_MODE=true ';
      var NOTICE = 'Each command runs with only the permissions it needs. Hosts and files are declared by the publisher and are not enforced by the CLI.';
      var FIRST_SCREEN = [
        'This plugin asks to use your Auth0 tenant ' + TENANT + ':',
        '',
        '  Can read     clients, connections, tenant settings',
        '  Hosts        api.github.com    Check SDK versions against GitHub advisories',
        '  Files        $CWD (write)      Save the audit report',
        '  Interactive  no',
        '',
        NOTICE,
        '',
        'Approve for ' + TENANT + '? [y/N/d]  (d shows the exact scopes) y'
      ];
      var FETCH_NOTE = 'Install and update need a current index. If it cannot be refreshed and the saved copy is out of date, the command fails and says what to run once you are online.';
      var AGENT_NOTE = 'In agent mode the CLI prints no human lines (they would be on stderr). The result is JSON on stdout and a failure is one JSON error on stderr.';
      // ---------------------------------------------------------------------------
      // Shared steps (the same object is reused by several scenarios).
      // ---------------------------------------------------------------------------
      var RUN_START = {
        actors: ['user', 'cli'], from: 'user', to: 'cli', label: RUN_CMD,
        title: 'A person or an agent runs the plugin command',
        text: 'Installed plugins appear as normal auth0 commands. The CLI sees the whole command line first, so it can decide what is safe before any plugin code starts.',
        data: [{ label: 'Command line', lines: ['$ ' + RUN_CMD] }]
      };
      var RUN_PRESCAN = {
        actors: ['c-prescan'], from: 'c-prescan', to: 'c-prescan',
        title: 'Pre-scan removes the CLI-owned flags',
        text: 'The CLI takes out only the flags it owns, such as `--tenant`, and applies them itself. Everything else goes to the plugin untouched, so a plugin flag can never shadow a CLI flag.',
        data: [{
          label: 'Arguments', lines: [
            'before:  --validators a --tenant ' + TENANT,
            del('--tenant ' + TENANT),
            hl('plugin gets: --validators a'),
            ok('--tenant becomes AUTH0_PLUGIN_TENANT=' + TENANT),
            note('Everything after a bare -- is passed as it was typed.')]
        }]
      };
      var RUN_VERIFY = {
        actors: ['c-verify', 'store'], from: 'cli', to: 'store', label: 'read lockfile, hash the files',
        title: 'Store.Verify checks the files against the lockfile',
        text: 'Before anything else, the CLI hashes the executable and plugin.json on disk and compares them with the hashes it recorded at install time. This happens on every run, before any login, so a swapped binary never gets a token.',
        data: [{
          label: 'Checks', lines: [
            ok('sha256 of the executable equals binSha256 in plugins.lock.json'),
            ok('sha256 of plugin.json equals manifestSha256'),
            ok('real files only: no symlinks, nothing others can write to, a single hard link'),
            note('A second check runs after the session is open. If anything changed in between, the run stops.')]
        }]
      };
      var RUN_REGISTRY = {
        actors: ['c-registry', 'cache'], from: 'cli', to: 'cache', label: 'load the verified index',
        title: 'Index check: not revoked, still usable',
        text: 'The CLI reads its cached index, which was verified when it was saved. Revocation is checked here, so a withdrawn version stops as soon as the CLI has seen the new index.',
        data: [{
          label: 'Checks', lines: [
            ok('auth0/checkmate 1.0.0 is not on the revocation list'),
            ok('the index is not expired and is inside the 30 day offline grace period'),
            note('A copy older than 24 hours is refreshed for up to 10 seconds. If that fails, the saved copy runs with a warning while it is still inside the grace period.')]
        }]
      };
      var RUN_CALLCHECK = {
        actors: ['c-call'], from: 'c-call', to: 'c-call',
        title: 'The command can run the way it was called',
        text: 'The manifest says what a command supports. A command declared interactive needs a terminal, and `--json`, `--csv` and agent mode work only if the command declares them.',
        data: [{
          label: 'Checks', lines: [
            ok('wizard-free command: not interactive, so no terminal is needed'),
            ok('the output the caller asked for is declared in the manifest')]
        }]
      };
      var RUN_APPROVAL = {
        actors: ['c-approval', 'approvals'], from: 'cli', to: 'approvals', label: 'tenant + plugin + permissions hash',
        title: 'Approval check for this tenant and these permissions',
        text: 'A person must have approved exactly these permissions on exactly this tenant. Nothing has been logged in yet, so a plugin that is not approved can never cause a token to be fetched.',
        data: [{
          label: 'approvals.json lookup', lines: [
            'tenant:  ' + TENANT,
            'plugin:  auth0/checkmate',
            'hash:    consent hash of the installed manifest',
            ok('the recorded hash equals the current hash'),
            note('A pin in AUTH0_CLI_PLUGIN_APPROVALS also counts, see the CI flow.')]
        }]
      };
      var RUN_SESSION = {
        actors: ['c-session'], from: 'c-session', to: 'c-session',
        title: 'Only now: session and token',
        text: 'The CLI uses the session it already has for this tenant, or logs in, and checks that it is the approved tenant. The access token stays in CLI memory. It is not written to the plugins folder and it is not handed to the plugin.',
        data: [{
          label: 'Token handling', lines: [
            ok('token held in CLI memory for the proxy'),
            no('not in argv, not in the plugin environment, not on disk'),
            note('If the session is for another tenant, the run stops with plugin_tenant_mismatch.')]
        }]
      };
      var RUN_PROXY = {
        actors: ['c-proxy', 'proxy'], from: 'cli', to: 'proxy', label: 'start with per-run cert + secret',
        title: 'Start the loopback proxy for this run',
        text: 'The proxy gets a fresh certificate and a random secret, and it is given only the scopes of the command being run. It denies by default, using a scope table and a secret field list that come from the signed index.',
        data: [{
          label: 'Proxy for this run', lines: [
            'listens on  https://127.0.0.1:<random port>',
            'per-run TLS certificate (CA file) and a random secret',
            hl('scopes: read:clients, read:connections, read:tenant_settings'),
            note('Only the scopes of the command that runs, not the union shown on the consent screen.')]
        }]
      };
      var RUN_SPAWN = {
        actors: ['c-spawn', 'plugin'], from: 'cli', to: 'plugin', label: 'spawn with a small environment',
        title: 'Spawn the plugin with a small environment',
        text: 'The plugin receives a short allowlist, the proxy address and the per-run secret, and nothing else. Its output goes straight to your terminal. It runs as you, in its own process group, with the timeout of the command (10m here).',
        data: [
          {
            label: 'Environment the plugin gets', lines: [
              ok('PATH, HOME, LANG, LC_ALL, LC_CTYPE, TERM, TMPDIR when set'),
              ok('HTTP_PROXY, HTTPS_PROXY, NO_PROXY and the lower case forms; NO_PROXY also names loopback'),
              ok('AUTH0_PLUGIN_DOMAIN=127.0.0.1:<port>   (the proxy, not your tenant)'),
              ok('AUTH0_PLUGIN_MANAGEMENT_TOKEN=<per-run secret>'),
              ok('AUTH0_PLUGIN_CA_CERT, SSL_CERT_FILE, NODE_EXTRA_CA_CERTS = <per-run CA file>'),
              ok('AUTH0_PLUGIN_TENANT=' + TENANT),
              ok('AUTH0_PLUGIN_JSON, _JSON_COMPACT, _CSV, _AGENT_MODE, _NO_INPUT, _NO_COLOR, _DEBUG'),
              ok('variables the command declared in permissions.env (none for checkmate)'),
              note('On Windows the list also has SystemRoot, TEMP, TMP, USERPROFILE, APPDATA, LOCALAPPDATA, PATHEXT and ComSpec.')]
          },
          {
            label: 'Environment it does NOT get', lines: [
              no('AUTH0_CLIENT_SECRET, AUTH0_API_TOKEN, AUTH0_DOMAIN'),
              no('every other AUTH0_ name from your shell, even if a manifest asks for it'),
              no('AWS_PROFILE and anything else not declared in permissions.env'),
              no('the real access token'),
              note('Plugins run as your user. The CLI limits the Auth0 calls they make, but they are not sandboxed, so they can read the files your user can read.')]
          }
        ]
      };
      var RUN_API = {
        actors: ['plugin', 'proxy'], from: 'plugin', to: 'proxy', label: 'GET /api/v2/clients (Bearer per-run secret)',
        title: 'The plugin calls the proxy with the per-run secret',
        text: 'The official SDK is pointed at `AUTH0_PLUGIN_DOMAIN` and uses the per-run secret as its token. The plugin never talks to the tenant directly and never sees a real token.',
        data: [{
          label: 'Request', lines: [
            'GET /api/v2/clients?per_page=50 HTTP/1.1',
            'Host: 127.0.0.1:<port>',
            'Authorization: Bearer <per-run secret>']
        }]
      };
      // ---------------------------------------------------------------------------
      // Scenarios. Step fields: actors, from, to, label (shown on the wire), title,
      // text, data, status (ok | denied | fail), dim (components shown as not reached
      // for this step only).
      // A scenario with neverReached lists what a terminal failure protected.
      // ---------------------------------------------------------------------------
      var SCENARIOS = [
        // ===== Install and update =====
        // ===== Install and update =====
        {
          id: 'install', group: 'Install and update', name: 'Install a plugin',
          command: 'auth0 plugin install checkmate',
          summary: 'Fetch and verify the index, show the permissions, download from an allowed host, verify the hash and the manifest, then write the lockfile and the approval.',
          steps: [
            {
              actors: ['user', 'cli'], from: 'user', to: 'cli', label: 'auth0 plugin install checkmate',
              title: 'Run the install command',
              text: 'A person asks the CLI to install checkmate. The name can be `checkmate`, `auth0/checkmate` or `checkmate@1.0.0`. Install needs no login, only a tenant name (' + TENANT + ' here, the default tenant) so the approval can be recorded for it. Nothing has been downloaded yet.',
              data: [{ label: 'Command', lines: ['$ auth0 plugin install checkmate', 'tenant: ' + TENANT + ' (default tenant, or --tenant)'] }]
            },
            {
              actors: ['c-registry', 'registry'], from: 'cli', to: 'registry', label: 'GET index.json and index.json.sig',
              title: 'Fetch the signed index and its signature',
              text: 'The CLI downloads the registry index and the signature from the registry. It trusts neither yet. Plugin records can sit in files that the index pins by hash, and those are checked against that hash as well.',
              data: [{
                label: 'Requests (default registry URL)', lines: [
                  'GET https://raw.githubusercontent.com/auth0/auth0-cli-plugins/registry/index.json',
                  'GET https://raw.githubusercontent.com/auth0/auth0-cli-plugins/registry/index.json.sig',
                  note(FETCH_NOTE)]
              }]
            },
            {
              actors: ['c-registry'], from: 'c-registry', to: 'c-registry',
              title: 'Verify the signature, the expiry and rollback',
              text: 'The Ed25519 signature is checked against public keys compiled into the CLI. No environment variable or config file can add a key. An expired index, or one older than the newest the CLI has accepted, is refused.',
              data: [{
                label: 'Checks', lines: [
                  ok('signature valid for the keyId in index.json.sig'),
                  ok('the index is not expired'),
                  ok('the index is not older than the newest one accepted before (state.json)'),
                  no('never falls back to an index that was not verified'),
                  note('On failure: "The auth0 registry index failed signature verification."')]
              }]
            },
            {
              actors: ['c-registry', 'cache'], from: 'c-registry', to: 'cache', label: 'index + signature + state.json',
              title: 'Cache the verified index and pick the version',
              text: 'Only a verified index is written to the registry cache. The CLI then picks the latest version of checkmate that is not revoked, has a build for this machine, and accepts this CLI version.',
              data: [{
                label: 'Resolution', lines: [
                  ok('auth0/checkmate 1.0.0 is not on the revocation list'),
                  ok('an archive exists for this OS and CPU'),
                  ok('the engines range accepts this CLI version'),
                  note('The cache also lets installed plugins run offline for up to 30 days, with a warning.')]
              }]
            },
            {
              actors: ['c-consent', 'user'], from: 'cli', to: 'user', label: 'Permissions requested for ' + TENANT,
              title: 'Show the permissions before downloading anything',
              text: 'The permissions come from the signed index, so a person who says no has used no bandwidth. Scopes are enforced by the proxy. Hosts and files are declared by the publisher, and the screen says the CLI does not enforce them.',
              data: [{
                label: 'Output (stderr and terminal)', lines: [
                  'auth0/checkmate 1.0.0  A plugin for the Auth0 CLI',
                  'Download: 8.5 MB (darwin-arm64)'].concat(FIRST_SCREEN.slice(0, 8), [
                    'Approve for ' + TENANT + '? [y/N/d]  (d shows the exact scopes)'])
              }]
            },
            {
              actors: ['user', 'c-consent'], from: 'user', to: 'cli', label: 'y',
              title: 'A person approves for this tenant',
              text: 'Only a person at a terminal can approve. Answering `d` first lists the exact scopes. With `--no-input`, in agent mode or without a terminal the plugin still installs, but no approval is recorded and the CLI says which command a person must run. There is no flag that approves for you. If your login may lack a scope the plugin asks for, the CLI warns you here, before the download. It never starts a login for that.',
              data: [{
                label: 'Prompt', lines: [
                  'Approve for ' + TENANT + '? [y/N/d]  (d shows the exact scopes) y',
                  note('The answer is kept in memory. It is written to disk only after the install succeeded.')]
              }]
            },
            {
              actors: ['cli', 'release'], from: 'cli', to: 'release', label: 'GET checkmate_1.0.0_darwin-arm64.tar.gz',
              title: 'Download from an allowed host',
              text: 'The URL comes from the signed index. The CLI follows only https URLs on GitHub release hosts and checks the host again on every redirect. The file is streamed into a temporary folder with a size cap, never straight into the store.',
              data: [{
                label: 'Download', lines: [
                  ok('https, port 443, no user name in the URL'),
                  ok('host is github.com, objects.githubusercontent.com or release-assets.githubusercontent.com'),
                  ok('size and SHA-256 are checked while streaming, saved under plugins/tmp/'),
                  note('The file name here follows the release template. The URL itself is whatever the signed index lists.')]
              }]
            },
            {
              actors: ['c-verify'], from: 'c-verify', to: 'c-verify',
              title: 'Check the SHA-256 against the index',
              text: 'The CLI hashes the file it received and compares it with the hash in the signed index. Nothing is unpacked or made executable until the two match, and a mismatch discards the file.',
              data: [{
                label: 'Hash check', lines: [
                  'expected (signed index):  sha256:<from the index>',
                  'computed (downloaded):    sha256:<same>',
                  ok('equal'),
                  note('On mismatch: "Downloaded file does not match the registry hash. The file was discarded."')]
              }]
            },
            {
              actors: ['c-verify'], from: 'c-verify', to: 'c-verify',
              title: 'Compare the manifest inside with the index record',
              text: 'The archive must hold exactly the executable and plugin.json, with no links and no extra files. The manifest inside is compared with the signed record, including the permissions the person just approved, so a binary cannot ask for more than the index promised.',
              data: [{
                label: 'Compared with the index record', lines: [
                  ok('id and version'), ok('platforms and engines'), ok('commands and whether each is interactive'), ok('permissions'),
                  note('On mismatch: "The artifact does not match the registry record. Nothing was installed."')]
              }]
            },
            {
              actors: ['cli', 'store'], from: 'cli', to: 'store', label: 'tmp/ to store/ and plugins.lock.json',
              title: 'Move into the store and write the lockfile',
              text: 'The verified files are moved into the version folder, and the lockfile is rewritten through a temporary file and a rename. An install either fully applies or leaves what was there before.',
              data: [{
                label: 'On disk (next to config.json)', lines: [
                  'plugins/store/auth0/checkmate/1.0.0/checkmate   (the executable)',
                  'plugins/store/auth0/checkmate/1.0.0/plugin.json',
                  'plugins/plugins.lock.json (0600): hashes of the executable, the manifest and the archive, the permissions hash, active = 1.0.0',
                  note('The active version is a lockfile field, not a symlink.')]
              }]
            },
            {
              actors: ['cli', 'approvals'], from: 'cli', to: 'approvals', label: TENANT + ' + permissions hash',
              title: 'Record the approval, after the install succeeded',
              text: 'The approval is written only now. It is keyed to the hash of the permissions, not to the version, so a release with the same permissions does not ask again.',
              data: [{ label: 'approvals.json (0600)', lines: ['tenant ' + TENANT + ', plugin auth0/checkmate, permissionsHash sha256:<hash>'] }]
            },
            {
              actors: ['cli', 'user'], from: 'cli', to: 'user', label: 'Installed auth0/checkmate 1.0.0.',
              title: 'Installed',
              text: 'The plugin is on disk and approved for ' + TENANT + '. No login or token was needed, and no plugin code has run yet. With `--json` the result is printed as JSON.',
              data: [{ label: 'Output', lines: ['Installed auth0/checkmate 1.0.0.'] }]
            }
          ]
        },
        {
          id: 'update-same', group: 'Install and update', name: 'Update, same permissions',
          command: 'auth0 plugin update checkmate',
          summary: 'A newer version with an unchanged permissions hash installs without a question. The old version stays on disk for a rollback.',
          steps: [
            {
              actors: ['user', 'cli'], from: 'user', to: 'cli', label: 'auth0 plugin update checkmate',
              title: 'Run the update command',
              text: 'Update moves one plugin to the latest version in the index and clears its pin. `--all` updates every plugin that is not pinned. It never goes backwards: if the active version is the latest, nothing is downloaded or asked.',
              data: [{ label: 'Command', lines: ['$ auth0 plugin update checkmate'] }]
            },
            {
              actors: ['c-registry', 'registry'], from: 'cli', to: 'registry', label: 'current, verified index',
              title: 'Fetch and verify a current index',
              text: 'Update needs a current index, and it goes through the same signature, expiry and rollback checks as an install. The verified index is cached.',
              data: [{ label: 'Same checks as install', lines: [ok('signature, expiry, not older than the newest accepted'), note(FETCH_NOTE)] }]
            },
            {
              actors: ['c-registry', 'store'], from: 'cli', to: 'store', label: 'compare permissions hash',
              title: 'Find 1.1.0 and compare the permissions hash',
              text: 'The index lists 1.1.0 (an example), newer than the active 1.0.0 and not revoked. Its permissions hash equals the one the person approved, so no consent screen is shown.',
              data: [{
                label: 'Comparison', lines: [
                  'active 1.0.0   permissions hash sha256:<H1>',
                  'latest 1.1.0   permissions hash sha256:<H1>',
                  ok('equal, so the update asks nothing')]
              }]
            },
            {
              actors: ['cli', 'release'], from: 'cli', to: 'release', label: 'GET checkmate_1.1.0_darwin-arm64.tar.gz',
              title: 'Download the new version from an allowed host',
              text: 'The old version keeps working until the new one is ready. The download follows the same host allowlist, size cap and redirect checks as an install.',
              data: [{ label: 'Download', lines: [ok('https on a GitHub release host, checked on every redirect'), ok('streamed to plugins/tmp/')] }]
            },
            {
              actors: ['c-verify'], from: 'c-verify', to: 'c-verify',
              title: 'Verify the hash and the embedded manifest',
              text: 'The archive hash must match the signed index, and the manifest inside must match the signed record. Only then is anything unpacked.',
              data: [{ label: 'Checks', lines: [ok('sha256 equals the index'), ok('manifest equals the index record, same permissions hash')] }]
            },
            {
              actors: ['cli', 'store'], from: 'cli', to: 'store', label: 'switch active to 1.1.0',
              title: 'Switch the lockfile to the new version',
              text: 'The lockfile is rewritten through a temporary file and a rename, so the update either fully applies or leaves 1.0.0 in place. The version that was active stays on disk as the previous one, for a rollback. Older versions are deleted.',
              data: [{
                label: 'plugins.lock.json', lines: [
                  del('active: 1.0.0'), add('active: 1.1.0'), add('previous: 1.0.0   (kept on disk)'),
                  'pinned: false']
              }]
            },
            {
              actors: ['cli', 'approvals'], from: 'cli', to: 'approvals', label: 'no change',
              title: 'The approval still applies',
              text: 'Approvals are keyed to the permissions hash, so the existing record for ' + TENANT + ' matches the new version. Nothing needs to change and the next run passes the approval check.',
              data: [{ label: 'approvals.json', lines: ['unchanged: ' + TENANT + ' / auth0/checkmate / sha256:<H1>'] }]
            },
            {
              actors: ['cli', 'user'], from: 'cli', to: 'user', label: 'Updated auth0/checkmate from 1.0.0 to 1.1.0.',
              title: 'Updated',
              text: 'The CLI reports the new active version. The next `auth0 checkmate` runs 1.1.0 with the usual checks.',
              data: [{ label: 'Output', lines: ['Updated auth0/checkmate from 1.0.0 to 1.1.0.'] }]
            }
          ]
        },
        {
          id: 'update-changed', group: 'Install and update', name: 'Update, changed permissions',
          command: 'auth0 plugin update checkmate',
          summary: 'A release that asks for different permissions shows only the difference, and is not applied without a person.',
          steps: [
            {
              actors: ['user', 'cli'], from: 'user', to: 'cli', label: 'auth0 plugin update checkmate',
              title: 'Run the update command',
              text: 'The same command as before, but the new release changes what the plugin may do. Consent for the new permissions is asked before anything is downloaded.',
              data: [{ label: 'Command', lines: ['$ auth0 plugin update checkmate'] }]
            },
            {
              actors: ['c-registry', 'registry'], from: 'cli', to: 'registry', label: 'current, verified index',
              title: 'Fetch and verify a current index',
              text: 'The signature, expiry and rollback checks run exactly as in an install. The index record for 1.1.0 carries its permissions, so the CLI can compare before it downloads.',
              data: [{ label: 'Same checks as install', lines: [ok('signature, expiry, not older than the newest accepted')] }]
            },
            {
              actors: ['c-registry', 'store'], from: 'cli', to: 'store', label: 'read installed plugin.json',
              title: 'The permissions hash differs from the installed version',
              text: 'The CLI reads the installed manifest and compares its permissions with the new record. They differ, so an approval is needed for this tenant.',
              data: [{
                label: 'Comparison', lines: [
                  'active 1.0.0   permissions hash sha256:<H1>',
                  'latest 1.1.0   permissions hash sha256:<H2>',
                  hl('different, so ask for ' + TENANT)]
              }]
            },
            {
              actors: ['c-consent', 'user'], from: 'cli', to: 'user', label: 'only what changed',
              title: 'The consent screen shows only what changed',
              text: 'This tenant approved the installed version, so only the difference is shown. If it had not, the person would see the full screen.',
              data: [{
                label: 'Consent screen (example change)', lines: [
                  'auth0/checkmate 1.1.0 asks for different permissions on tenant ' + TENANT,
                  '  Added        can read logs',
                  '  Added        host api.osv.dev    Look up known vulnerabilities',
                  NOTICE,
                  '',
                  'Approve for ' + TENANT + '? [y/N/d]  (d shows the exact scopes)']
              }]
            },
            {
              actors: ['user', 'c-consent'], from: 'user', to: 'cli', label: 'y',
              title: 'A person approves the change',
              text: 'The answer is kept in memory. It is written to disk after the update succeeded, so a failed update is never approved.',
              data: [{ label: 'Prompt', lines: ['Approve for ' + TENANT + '? [y/N/d]  (d shows the exact scopes) y'] }]
            },
            {
              actors: ['cli', 'release'], from: 'cli', to: 'release', label: 'download and verify 1.1.0',
              title: 'Download and verify the new version',
              text: 'The usual allowlist, size cap, hash and manifest checks apply. The permissions in the archive must equal the ones the person just approved.',
              data: [{ label: 'Checks', lines: [ok('sha256 equals the index'), ok('manifest permissions hash equals sha256:<H2>')] }]
            },
            {
              actors: ['cli', 'store'], from: 'cli', to: 'store', label: 'active 1.1.0, previous 1.0.0',
              title: 'Switch the lockfile',
              text: 'The lockfile switches to 1.1.0 in one step, and 1.0.0 stays on disk as the previous version.',
              data: [{ label: 'plugins.lock.json', lines: [del('active: 1.0.0'), add('active: 1.1.0'), add('previous: 1.0.0')] }]
            },
            {
              actors: ['cli', 'approvals'], from: 'cli', to: 'approvals', label: 'replace H1 with H2',
              title: 'Record the new approval',
              text: 'The new hash replaces the old one for this tenant. Other tenants are asked at their next run.',
              data: [{ label: 'approvals.json', lines: [del('sha256:<H1>'), add('sha256:<H2>')] }]
            },
            {
              actors: ['cli'], from: 'cli', to: 'user', label: 'error, exit 1', status: 'fail', dim: ['release', 'store', 'approvals'],
              title: 'The same update with no person: the old version stays active',
              text: 'In agent mode, with `--no-input` or without a terminal, nobody can approve, so nothing is downloaded and nothing changes. The command exits 1 and says why.',
              data: [{
                label: 'Message (agent mode: the same text, in a JSON error)', lines: [
                  'checkmate 1.1.0 asks for permissions that are not approved, and an agent cannot approve plugins. checkmate was not updated, and 1.0.0 stays active. Run this in a terminal to review them: auth0 plugin update checkmate',
                  note('With no terminal the reason reads "there is no terminal to ask in".')]
              }]
            }
          ]
        },
        {
          id: 'rollback', group: 'Install and update', name: 'Roll back to the previous version',
          command: 'auth0 plugin install checkmate@1.0.0',
          summary: 'Going back reuses the version already on disk. If its permissions differ from what the tenant approved, the person is asked again.',
          steps: [
            {
              actors: ['user', 'cli'], from: 'user', to: 'cli', label: 'auth0 plugin install checkmate@1.0.0',
              title: 'Install the exact earlier version',
              text: 'Version 1.1.0 is active and 1.0.0 is still on disk from the last update. Asking for an exact version pins the plugin, so `update --all` skips it.',
              data: [{ label: 'State before', lines: ['active: 1.1.0   (approved hash H2 on ' + TENANT + ')', 'previous: 1.0.0   (permissions hash H1)'] }]
            },
            {
              actors: ['c-registry', 'registry'], from: 'cli', to: 'registry', label: 'current, verified index',
              title: 'Fetch and verify a current index',
              text: 'Install needs a current index even for a rollback. 1.0.0 must still be listed and must not be revoked.',
              data: [{ label: 'Same checks as install', lines: [ok('signature, expiry, not older than the newest accepted'), ok('1.0.0 is listed and not revoked')] }]
            },
            {
              actors: ['c-registry', 'store'], from: 'cli', to: 'store', label: 'compare H1 with active H2',
              title: 'Compare the permissions hash with the active version',
              text: 'An approval is asked for when the plugin is new or its permissions differ from the active version. H1 differs from H2, so the rollback needs one.',
              data: [{
                label: 'Rule', lines: [
                  'active 1.1.0: H2     target 1.0.0: H1',
                  hl('H1 is different from H2, so ask'),
                  note('If the hashes were equal, there would be no screen and the approval would stay as it is.')]
              }]
            },
            {
              actors: ['c-consent', 'user'], from: 'cli', to: 'user', label: 'only what changed vs 1.1.0',
              title: 'Ask again, because the hash is not approved',
              text: 'The tenant has only H2 on record, so the older permissions count as a new request, shown as the difference from the installed version.',
              data: [{
                label: 'Consent screen (example change)', lines: [
                  'auth0/checkmate 1.0.0 asks for different permissions on tenant ' + TENANT,
                  '  Removed      can read logs',
                  '  Removed      host api.osv.dev    Look up known vulnerabilities',
                  note('Without a terminal, or in agent mode, the rollback still installs, no approval is recorded, and running it needs auth0 plugin approve.')]
              }]
            },
            {
              actors: ['user', 'c-consent'], from: 'user', to: 'cli', label: 'y',
              title: 'A person approves',
              text: 'Only a person at a terminal can approve. The answer is held in memory until the install has succeeded.',
              data: [{ label: 'Prompt', lines: ['Approve for ' + TENANT + '? [y/N/d]  (d shows the exact scopes) y'] }]
            },
            {
              actors: ['c-verify', 'store'], from: 'cli', to: 'store', label: 'reuse 1.0.0 on disk, no download',
              title: 'Reuse the version on disk and switch the lockfile',
              text: 'A version that is on disk and verifies is not downloaded again. Reuse needs the stored archive hash and permissions hash to equal the index record. Then the lockfile switches to it.',
              data: [{
                label: 'plugins.lock.json', lines: [
                  ok('archive hash and permissions hash equal the index record'),
                  del('active: 1.1.0'), add('active: 1.0.0'), add('previous: 1.1.0'), add('pinned: true')]
              }]
            },
            {
              actors: ['cli', 'approvals'], from: 'cli', to: 'approvals', label: 'replace H2 with H1',
              title: 'Record the approval for the older permissions',
              text: 'The approval is written after the switch, and it replaces H2 with H1 for this tenant. Moving forward to 1.1.0 later will therefore ask again.',
              data: [{ label: 'approvals.json', lines: [del('sha256:<H2>'), add('sha256:<H1>')] }]
            },
            {
              actors: ['cli', 'user'], from: 'cli', to: 'user', label: 'Switched auth0/checkmate from 1.1.0 to 1.0.0.',
              title: 'Rolled back',
              text: 'The plugin runs 1.0.0 again and shows as pinned in `plugin list`. `plugin update checkmate` by name moves it forward and clears the pin.',
              data: [{
                label: 'Output', lines: [
                  'Switched auth0/checkmate from 1.1.0 to 1.0.0.',
                  'auth0/checkmate is pinned to 1.0.0, so auth0 plugin update --all leaves it alone. Move it to the latest version with: auth0 plugin update checkmate']
              }]
            }
          ]
        },
        // ===== Run =====
        {
          id: 'run', group: 'Run', name: 'Run, happy path',
          command: RUN_CMD,
          summary: 'Every check runs before a token exists. The plugin only ever talks to the proxy.',
          steps: [
            RUN_START, RUN_PRESCAN, RUN_VERIFY, RUN_REGISTRY, RUN_CALLCHECK, RUN_APPROVAL, RUN_SESSION, RUN_PROXY, RUN_SPAWN, RUN_API,
            {
              actors: ['proxy'], from: 'proxy', to: 'proxy',
              title: 'The proxy checks the path, the method and the scope',
              text: 'The path must already be in its canonical form. The method must be on the allowlist. Then the call is looked up in a table that comes from the signed index. Anything not on the table, or not covered by the scopes of this command, is denied.',
              data: [{
                label: 'Proxy decision', lines: [
                  ok('path is canonical and under /api/v2: ".." and encoded separators are rejected, not repaired'),
                  ok('method GET is on the allowlist'),
                  ok('rule: GET /api/v2/clients needs read:clients (the most specific rule wins)'),
                  ok('read:clients is among the scopes of this run, so: allow')]
              }]
            },
            {
              actors: ['proxy', 'mgmt'], from: 'proxy', to: 'mgmt', label: 'GET /api/v2/clients (real token)',
              title: 'The proxy swaps in the real token and forwards',
              text: 'Only the proxy holds the access token. It drops the plugin\'s Authorization header, keeps a short list of other headers, adds the real token and sends the call to the tenant. It does not follow redirects.',
              data: [{
                label: 'Outgoing request', lines: [
                  'GET https://' + TENANT + '/api/v2/clients?per_page=50',
                  'Authorization: Bearer <real access token>   (never visible to the plugin)']
              }]
            },
            {
              actors: ['mgmt', 'proxy'], from: 'mgmt', to: 'proxy', label: '200 OK with client_secret',
              title: 'The tenant answers',
              text: 'The raw response can contain secrets, such as a client secret. At this point it reaches only the proxy.',
              data: [{ label: 'Raw response (inside the CLI only)', lines: ['200 OK', '[{ "client_id": "abc", "name": "Acme Web", "client_secret": "<secret value>" }]'] }]
            },
            {
              actors: ['proxy', 'plugin'], from: 'proxy', to: 'plugin', label: '200 OK, secret fields removed',
              title: 'Secret fields are stripped before the plugin sees the response',
              text: 'The proxy removes the secret field names listed in the signed index, wherever they appear in the JSON. Plugins that need secrets stay core CLI features.',
              data: [{ label: 'Response to the plugin', lines: ['200 OK', '[{ "client_id": "abc", "name": "Acme Web" }]', del('"client_secret": "<secret value>"')] }]
            },
            {
              actors: ['plugin', 'cli'], from: 'plugin', to: 'cli', label: 'exit 0',
              title: 'The plugin finishes',
              text: 'The plugin writes its report to the current folder (a declared permission, not enforced) and exits 0. Its output went straight to your terminal. The CLI never captured or rewrote it.',
              data: [{ label: 'Result', lines: ['exit code 0'] }]
            },
            {
              actors: ['cli', 'user'], from: 'cli', to: 'user', label: 'summary of denied calls',
              title: 'The CLI prints one line about denied calls',
              text: 'After each run the CLI reports the calls the proxy denied, so a plugin that swallows errors cannot hide missing results. Nothing was denied in this run, so nothing is printed. The denied-call flow shows the line.',
              data: [{ label: 'stderr', lines: ['(empty: no call was denied)'] }]
            },
            {
              actors: ['cli', 'user'], from: 'cli', to: 'user', label: 'hint about a newer version',
              title: 'A hint, at most once a day',
              text: 'After a successful run of an installed plugin, the CLI compares the installed version with the saved registry index. If a newer version can run, or this one is deprecated, it prints one line on stderr and notes the time in `hint.json`. It stays quiet for 24 hours, in agent mode, with `--no-input`, with a machine output format, when the output is not a terminal, for linked builds, and when `AUTH0_CLI_PLUGIN_UPDATE_HINT` is false. It reads the saved index only and never changes the result of the run.',
              data: [{ label: 'stderr (only when there is something to say)', lines: ['checkmate 1.1.0 is available. Run: auth0 plugin update checkmate'] }]
            },
            {
              actors: ['cli', 'proxy'], from: 'cli', to: 'proxy', label: 'close',
              title: 'The proxy closes',
              text: 'The CLI closes the proxy and the per-run certificate goes with it. The secret stops working, so a copy kept by the plugin is useless afterwards. For plugins of the built-in registry the CLI then sends one usage event, unless you set `AUTH0_CLI_ANALYTICS=false`.',
              data: [{
                label: 'After the run', lines: [
                  no('per-run secret is dead'),
                  no('per-run certificate is gone'),
                  note('The event holds the plugin name and version, the CLI version, OS and CPU, whether the run succeeded, and for a failed run its reason code (or "other" when it is not one of the known codes). It goes out with the install ID that other CLI events use. Nothing about the arguments, the tenant or the output.'),
                  note('Only plugins of the auth0 registry are counted. Linked builds are never counted, and nothing is sent when AUTH0_CLI_ANALYTICS=false is set.')]
              }]
            }
          ]
        },
        {
          id: 'denied', group: 'Run', name: 'Denied call: missing scope',
          command: 'DELETE /api/v2/clients/abc from a plugin that declared only read scopes',
          summary: 'The proxy answers 403 without ever contacting the tenant, and the CLI mentions it after the run.',
          steps: [
            {
              actors: ['cli', 'proxy', 'plugin'], from: 'cli', to: 'plugin', label: 'plugin running',
              title: 'A run is in progress with only the declared scopes',
              text: 'This continues the happy path. The proxy was started with the scopes of the command being run, which are read scopes only. `delete:clients` is not among them.',
              data: [{ label: 'Scopes given to this run', lines: [ok('read:clients, read:connections, read:tenant_settings'), no('delete:clients')] }]
            },
            {
              actors: ['plugin', 'proxy'], from: 'plugin', to: 'proxy', label: 'DELETE /api/v2/clients/abc',
              title: 'The plugin asks to delete a client',
              text: 'The call is well formed and carries the right per-run secret. Whether it is allowed depends on the scope table, not on the secret.',
              data: [{ label: 'Request', lines: ['DELETE /api/v2/clients/abc HTTP/1.1', 'Authorization: Bearer <per-run secret>'] }]
            },
            {
              actors: ['proxy'], from: 'proxy', to: 'proxy', status: 'denied', dim: ['mgmt'],
              title: 'No scope covers this call',
              text: 'The rule for this endpoint needs `delete:clients`. The plugin did not declare it, so the proxy denies the call itself. The tenant is never contacted, even though the real token of the CLI might have allowed it.',
              data: [{
                label: 'Proxy decision', lines: [
                  'rule: DELETE /api/v2/clients/{id} needs delete:clients',
                  no('delete:clients is not in the scopes of this run: deny'),
                  note('Calls missing from the table are denied too (no_scope_rule), and so are paths outside /api/v2 (forbidden_path).')]
              }]
            },
            {
              actors: ['proxy', 'plugin'], from: 'proxy', to: 'plugin', label: '403 insufficient_scope', status: 'denied',
              title: 'The plugin gets a 403 that names the missing scope',
              text: 'The error body is shaped like a Management API error, so SDKs raise a normal 403. It tells the author exactly what to add to the manifest.',
              data: [{
                label: 'Response', lines: [
                  '403 Forbidden',
                  '{ "statusCode": 403, "error": "Forbidden",',
                  '  "message": "Plugin auth0/checkmate did not declare scope \\"delete:clients\\" required for DELETE /api/v2/clients/abc. Add it to permissions.scopes in plugin.json.",',
                  '  "errorCode": "insufficient_scope" }']
              }]
            },
            {
              actors: ['plugin', 'cli'], from: 'plugin', to: 'cli', label: 'exit 0 (error swallowed)',
              title: 'The plugin swallows the error and finishes',
              text: 'Plugins are not trusted to report their own failures. This one exits 0 as if nothing had happened.',
              data: [{ label: 'Result', lines: ['exit code 0'] }]
            },
            {
              actors: ['cli', 'user'], from: 'cli', to: 'user', label: '1 API call was denied',
              title: 'The CLI summary says it anyway',
              text: 'Because the proxy counted the denial, the CLI prints one line on stderr after the run, with the scope to add. In agent mode the line is not printed. When the run fails, the same text is in `error.details.deniedCalls` of the JSON error.',
              data: [{ label: 'stderr', lines: ['Plugin auth0/checkmate: 1 API call was denied (delete:clients for DELETE /api/v2/clients/abc). Review permissions.scopes in plugin.json.'] }]
            }
          ]
        },
        {
          id: 'strip', group: 'Run', name: 'Secret stripping',
          command: 'GET /api/v2/clients/abc from a running plugin',
          summary: 'The tenant answers with a client secret. The proxy removes it before the plugin sees the response.',
          steps: [
            {
              actors: ['plugin', 'proxy'], from: 'plugin', to: 'proxy', label: 'GET /api/v2/clients/abc',
              title: 'The plugin reads one client',
              text: 'The call is covered by `read:clients`, so the proxy allows it. Allowing the call does not mean allowing everything in the answer.',
              data: [{ label: 'Request', lines: ['GET /api/v2/clients/abc HTTP/1.1', 'Authorization: Bearer <per-run secret>', ok('rule: GET /api/v2/clients/{id} needs read:clients')] }]
            },
            {
              actors: ['proxy', 'mgmt'], from: 'proxy', to: 'mgmt', label: 'GET /api/v2/clients/abc (real token)',
              title: 'The proxy forwards with the real token',
              text: 'The per-run secret is replaced by the real access token, which only the proxy holds.',
              data: [{ label: 'Outgoing request', lines: ['GET https://' + TENANT + '/api/v2/clients/abc', 'Authorization: Bearer <real access token>'] }]
            },
            {
              actors: ['mgmt', 'proxy'], from: 'mgmt', to: 'proxy', label: '200 OK',
              title: 'The tenant answers with a secret in the body',
              text: 'The Management API returns the client secret to a token that may read clients. This is the data the plugin must never receive.',
              data: [{
                label: 'Raw response (inside the CLI only)', lines: [
                  '200 OK',
                  '{ "client_id": "abc",', '  "name": "Acme Web",', hl('  "client_secret": "<secret value>"'), '}']
              }]
            },
            {
              actors: ['proxy'], from: 'proxy', to: 'proxy',
              title: 'The proxy removes the secret fields',
              text: 'The secret field names in the signed index are matched without regard to case, at any depth, and the member is removed with its value. The list is data in the signed index, so it can grow without a CLI release.',
              data: [{
                label: 'Redaction', lines: [
                  '{ "client_id": "abc",', '  "name": "Acme Web",', del('  "client_secret": "<secret value>"'), '}',
                  note('A response must be exactly one JSON value to be inspected. Anything else is refused (unsupported_response).')]
              }]
            },
            {
              actors: ['proxy', 'plugin'], from: 'proxy', to: 'plugin', label: '200 OK, client_secret removed',
              title: 'The plugin receives the redacted response',
              text: 'The plugin gets everything it needs for an audit except the secret. Even if it asks for the field by name, it is removed.',
              data: [{ label: 'Response to the plugin', lines: ['200 OK', '{ "client_id": "abc", "name": "Acme Web" }'] }]
            }
          ]
        },
        {
          id: 'timeout', group: 'Run', name: 'Timeout',
          command: RUN_CMD,
          summary: 'The plugin outlives its timeout. The CLI stops the whole process tree and reports a structured error.',
          steps: [
            {
              actors: ['c-spawn', 'plugin'], from: 'cli', to: 'plugin', label: 'spawn, timeout 10m, own process group',
              title: 'All checks passed and the plugin starts with a timeout',
              text: 'The timeout is the `timeout` of the command in the manifest (10m here), or the registry default of 10 minutes. A command that is not interactive runs in its own process group on Unix, and in a job object on Windows, so everything it starts can be stopped with it.',
              data: [{
                label: 'Spawn', lines: [
                  'timeout: 10m (manifest)',
                  'Unix, not interactive: own process group',
                  'Windows: the plugin and what it starts belong to a job object',
                  note('An interactive command stays in the group of the CLI, so the terminal keeps working. On Unix a timeout then stops only the plugin process itself.')]
              }]
            },
            {
              actors: ['plugin'], from: 'plugin', to: 'plugin',
              title: 'The plugin hangs',
              text: 'Suppose a validator waits forever on a slow network, and a helper process it started keeps running as well. The CLI is still waiting and the clock is running.',
              data: [{ label: 'State', lines: ['plugin process: running', 'child of the plugin: running', 'elapsed: 10m'] }]
            },
            {
              actors: ['cli', 'plugin'], from: 'cli', to: 'plugin', label: 'SIGTERM to the process group', status: 'fail',
              title: 'Timeout reached: ask the whole group to stop',
              text: 'The CLI asks the plugin and everything it started to stop. Ctrl-C or SIGTERM sent to the CLI is passed on the same way.',
              data: [{ label: 'Signal', lines: ['SIGTERM, to the process group', note('On Windows the request is a Ctrl-Break to the group, on a best effort basis.')] }]
            },
            {
              actors: ['plugin'], from: 'plugin', to: 'plugin', status: 'fail',
              title: 'A 5 second grace period',
              text: 'A well behaved plugin gets a few seconds to clean up. This one ignores the signal.',
              data: [{ label: 'Grace', lines: ['5 s, plugin still running'] }]
            },
            {
              actors: ['cli', 'plugin'], from: 'cli', to: 'plugin', label: 'SIGKILL to the process group', status: 'fail',
              title: 'Kill the whole group',
              text: 'The group is killed, including the helper process, so nothing keeps running after the CLI reports the failure. On Windows the CLI ends the job object.',
              data: [{ label: 'Signal', lines: ['SIGKILL, to the process group', ok('plugin and its child are gone')] }]
            },
            {
              actors: ['cli', 'proxy'], from: 'cli', to: 'proxy', label: 'close',
              title: 'The proxy closes even on failure',
              text: 'The per-run secret and certificate are discarded, so nothing the plugin left behind can reach the tenant.',
              data: [{ label: 'After the run', lines: [no('per-run secret is dead')] }]
            },
            {
              actors: ['cli', 'user'], from: 'cli', to: 'user', label: 'error, exit 1', status: 'fail',
              title: 'A structured error with a reason',
              text: 'The CLI exits 1. The error class is "plugin" and the reason says what happened, so scripts can tell a plugin failure from a CLI failure.',
              data: [
                { label: 'Human mode (stderr)', lines: ['Checkmate 1.0.0 did not finish within 10m0s and was stopped.'] },
                { label: 'Agent mode (stderr)', lines: ['{"error":{"code":"plugin","reason":"plugin_timeout","message":"checkmate 1.0.0 did not finish within 10m0s and was stopped","details":{"pluginId":"auth0/checkmate","version":"1.0.0"}}}'] },
                { label: 'If the plugin exits by itself with code 7', lines: ['{"error":{"code":"plugin","reason":"plugin_exit","message":"checkmate 1.0.0 exited with code 7","details":{"pluginId":"auth0/checkmate","version":"1.0.0","exitCode":7}}}', note('details.exitCode carries the code of the plugin. The CLI process itself exits 1. Other reasons: plugin_interrupted and plugin_start_failed.')] }
              ]
            }
          ]
        },
        // ===== Blocked paths =====
        {
          id: 'tamper', group: 'Blocked paths', name: 'Blocked: tampered binary',
          command: RUN_CMD,
          summary: 'The binary on disk no longer matches the lockfile, so the run stops before any login.',
          neverReached: ['Registry check', 'Call check', 'Approval check', 'Session / login', 'Proxy start', 'Spawn plugin'],
          neverActors: ['c-registry', 'c-call', 'c-approval', 'c-session', 'c-proxy', 'c-spawn', 'cache', 'approvals', 'proxy', 'plugin', 'mgmt'],
          protectedNote: 'Protected: no login, no token, no proxy. The modified file never ran.',
          steps: [
            RUN_START, RUN_PRESCAN,
            {
              actors: ['c-verify', 'store'], from: 'cli', to: 'store', label: 'hash differs from the lockfile', status: 'fail',
              title: 'Store.Verify fails: the binary changed',
              text: 'Something replaced or edited the executable after the install. The hash no longer matches the lockfile, so the CLI refuses to run it, and it does so before any login. The same happens when plugin.json changes.',
              data: [
                {
                  label: 'Human mode (stderr)', lines: [
                    'Checkmate no longer matches what was verified when it was installed (the hash of checkmate changed), so it will not run. Reinstall it with: auth0 plugin install checkmate.',
                    no('lockfile hash  !=  hash of the file on disk')]
                },
                { label: 'Agent mode (stderr)', lines: ['{"error":{"code":"validation","reason":"plugin_verification_failed","message":"checkmate no longer matches what was verified when it was installed (the hash of checkmate changed), so it will not run. Reinstall it with: auth0 plugin install checkmate"}}'] }
              ]
            }
          ]
        },
        {
          id: 'revoked', group: 'Blocked paths', name: 'Blocked: revoked version',
          command: RUN_CMD,
          summary: 'The binary is intact, but the signed index has revoked this version, so it no longer runs.',
          neverReached: ['Call check', 'Approval check', 'Session / login', 'Proxy start', 'Spawn plugin'],
          neverActors: ['c-call', 'c-approval', 'c-session', 'c-proxy', 'c-spawn', 'approvals', 'proxy', 'plugin', 'mgmt'],
          protectedNote: 'Protected: no login, no token, no proxy. A revoked version never starts.',
          steps: [
            RUN_START, RUN_PRESCAN, RUN_VERIFY,
            {
              actors: ['c-registry', 'cache'], from: 'cli', to: 'cache', label: 'auth0/checkmate 1.0.0 is revoked', status: 'fail',
              title: 'Index check fails: this version was revoked',
              text: 'Revocation lives in the signed index, so it works as a fast kill switch even though the hash of the binary is still valid. Revocations are kept, so a machine cannot run a revoked version after it refreshes its index.',
              data: [{ label: 'Message (stderr)', lines: ['Checkmate 1.0.0 was revoked: <reason>. Run: auth0 plugin update checkmate.', note('<reason> is the text of the revocation entry in the index. The reason code is plugin_revoked.')] }]
            }
          ]
        },
        {
          id: 'unapproved', group: 'Blocked paths', name: 'Blocked: not approved on this tenant',
          command: RUN_CMD,
          summary: 'The binary and the index are fine, but no person approved these permissions on this tenant. No login happens.',
          neverReached: ['Session / login', 'Proxy start', 'Spawn plugin'],
          neverActors: ['c-session', 'c-proxy', 'c-spawn', 'proxy', 'plugin', 'mgmt'],
          protectedNote: 'Protected: no login happened, no token was fetched, no proxy started, and the plugin never ran.',
          steps: [
            RUN_START, RUN_PRESCAN, RUN_VERIFY, RUN_REGISTRY, RUN_CALLCHECK,
            {
              actors: ['c-approval', 'approvals'], from: 'cli', to: 'approvals', label: 'no record for ' + TENANT, status: 'fail',
              title: 'Approval check fails: nobody approved this here',
              text: 'Approval is per tenant. Here the plugin was installed in agent or no-input mode, so no approval was recorded. The CLI stops and prints the exact command a person must run. If the tenant had approved other permissions earlier, the message says the plugin "changed what it asks for since you approved it".',
              data: [
                { label: 'Human mode (stderr)', lines: ['Checkmate is not approved for ' + TENANT + '. Run: ' + APPROVE + TENANT + '.', no('no token was requested')] },
                { label: 'Agent mode (stderr)', lines: ['{"error":{"code":"validation","reason":"plugin_not_approved","message":"checkmate is not approved for ' + TENANT + '. Run: ' + APPROVE + TENANT + '"}}'] }
              ]
            }
          ]
        },
        {
          id: 'second-tenant', group: 'Blocked paths', name: 'Approve on a second tenant',
          command: 'auth0 checkmate --tenant ' + OTHER,
          summary: 'Approval on ' + TENANT + ' does not carry over. Only a person at a terminal can approve the other tenant. An agent cannot.',
          steps: [
            {
              actors: ['user', 'cli'], from: 'user', to: 'cli', label: 'auth0 checkmate --tenant ' + OTHER,
              title: 'Run the plugin against another tenant',
              text: 'The plugin is approved on ' + TENANT + ' only. The command targets ' + OTHER + ', which nobody has approved.',
              data: [{ label: 'approvals.json before', lines: [ok(TENANT + ' / auth0/checkmate / sha256:<H1>'), no(OTHER + ': no record')] }]
            },
            {
              actors: ['c-verify', 'c-registry'], from: 'cli', to: 'store', label: 'binary and index checks',
              title: 'The binary and index checks pass',
              text: 'The binary hash matches the lockfile and the version is not revoked. These checks do not depend on the tenant.',
              data: [{ label: 'Checks', lines: [ok('hashes equal the lockfile'), ok('not revoked, index usable')] }]
            },
            {
              actors: ['c-approval', 'approvals'], from: 'cli', to: 'approvals', label: 'no record for ' + OTHER, status: 'fail', dim: ['c-session', 'c-proxy', 'c-spawn', 'proxy', 'plugin', 'mgmt'],
              title: 'The approval check fails for this tenant',
              text: 'Approvals are keyed to tenant and plugin. The run stops with the exact command, and no login or token is involved.',
              data: [{ label: 'Message (stderr)', lines: ['Checkmate is not approved for ' + OTHER + '. Run: ' + APPROVE + OTHER + '.'] }]
            },
            {
              actors: ['user', 'cli'], from: 'user', to: 'cli', label: 'approve, in agent mode', status: 'fail',
              title: 'An agent tries to approve and is refused',
              text: 'Only a person can approve. The installed copy is checked first, then the refusal comes before the consent screen and nothing is written. There is no `--yes` flag that approves.',
              data: [{
                label: 'Message (agent mode, stderr)', lines: [
                  '{"error":{"code":"unknown","reason":"unclassified","message":"only a person can approve a plugin. Run this in a terminal: ' + APPROVE + OTHER + '"}}',
                  note('Without a terminal in human mode: "Only a person can approve a plugin. Run this in a terminal: ..."')]
              }]
            },
            {
              actors: ['c-consent', 'user'], from: 'cli', to: 'user', label: 'Permissions requested for ' + OTHER,
              title: 'A person runs approve in a terminal',
              text: 'The person runs `auth0 plugin approve auth0/checkmate --tenant ' + OTHER + '` and sees the full consent screen for that tenant.',
              data: [{
                label: 'Output', lines: ['auth0/checkmate 1.0.0  A plugin for the Auth0 CLI', ''].concat(
                  FIRST_SCREEN.slice(0, 8).map(function (l) { return l.replace(TENANT, OTHER); }),
                  ['Approve for ' + OTHER + '? [y/N/d]  (d shows the exact scopes)'])
              }]
            },
            {
              actors: ['user', 'c-consent'], from: 'user', to: 'cli', label: 'y',
              title: 'The person approves',
              text: 'A person answering at a terminal is the only way a new approval is created.',
              data: [{ label: 'Prompt', lines: ['Approve for ' + OTHER + '? [y/N/d]  (d shows the exact scopes) y'] }]
            },
            {
              actors: ['cli', 'approvals'], from: 'cli', to: 'approvals', label: 'add ' + OTHER,
              title: 'The approval is recorded for the second tenant',
              text: 'A second record is added next to the first. The record for ' + TENANT + ' is not touched.',
              data: [{ label: 'approvals.json after', lines: [TENANT + ' / auth0/checkmate / sha256:<H1>', add(OTHER + ' / auth0/checkmate / sha256:<H1>'), 'Approved auth0/checkmate 1.0.0 for ' + OTHER + '.'] }]
            },
            {
              actors: ['c-approval', 'approvals'], from: 'cli', to: 'approvals', label: 'approved',
              title: 'The next run passes the approval check',
              text: 'The same command now clears the approval check. Session, proxy and spawn follow exactly as in the happy path.',
              data: [{ label: 'Approval check', lines: [ok(OTHER + ' / auth0/checkmate / sha256:<H1> is on record')] }]
            }
          ]
        },
        // ===== Agent mode and CI =====
        {
          id: 'agent-install', group: 'Agent mode and CI', name: 'Agent installs a plugin',
          command: 'auth0 plugin install checkmate   (run by an AI agent)',
          summary: 'An agent can install a plugin, but it can never approve one. The install succeeds without an approval, and the JSON result says which command a person must run.',
          steps: [
            {
              actors: ['user', 'cli'], from: 'user', to: 'cli', label: 'auth0 plugin install checkmate',
              title: 'An agent runs the install command',
              text: 'The CLI turns agent mode on when it sees an agent in the environment (for example `AI_AGENT`, `CLAUDECODE` or `CURSOR_AGENT`), or when you pass `--agent-mode` or set `AUTH0_AGENT_MODE=true`. `AUTH0_AGENT_MODE=false` turns it off. Agent mode means JSON output, no prompts and no color.',
              data: [{ label: 'Command', lines: ['$ auth0 plugin install checkmate', 'agent mode: on', note(AGENT_NOTE)] }]
            },
            {
              actors: ['c-registry', 'registry'], from: 'cli', to: 'registry', label: 'current, verified index',
              title: 'Fetch and verify the index, as for a person',
              text: 'Nothing is relaxed for an agent. The same signature, expiry and rollback checks run, and the index must be current.',
              data: [{ label: 'Same checks as install', lines: [ok('signature, expiry, not older than the newest accepted'), ok('1.0.0 is not revoked')] }]
            },
            {
              actors: ['c-consent'], from: 'c-consent', to: 'c-consent',
              title: 'No consent screen: an agent is never asked',
              text: 'The consent screen is for a person. An agent is not asked even if it runs in a terminal, because only a person approves a plugin. The install goes on and records nothing.',
              data: [{ label: 'Rule', lines: [no('no screen, no prompt, no approval recorded'), note('A new plugin installs without an approval. An update that needs new permissions does not, see the update flow.')] }]
            },
            {
              actors: ['cli', 'release', 'store'], from: 'cli', to: 'release', label: 'download, verify, move into the store',
              title: 'Download, verify and store, as for a person',
              text: 'The host allowlist, the size cap, the SHA-256 check, the manifest comparison and the atomic lockfile write all apply. The plugin is on disk and verified, and it cannot run yet.',
              data: [{ label: 'Same checks as install', lines: [ok('allowed GitHub release host'), ok('sha256 equals the signed index'), ok('manifest equals the index record'), ok('lockfile written atomically')] }]
            },
            {
              actors: ['cli', 'user'], from: 'cli', to: 'user', label: 'JSON result, exit 0',
              title: 'The JSON result says an approval is needed',
              text: 'The command exits 0 and prints the result on stdout. `approvalRequired` is true and `approveCommand` is the exact command for a person.',
              data: [{
                label: 'stdout', lines: [
                  '{',
                  '  "id": "auth0/checkmate",',
                  '  "version": "1.0.0",',
                  '  "previousVersion": "",',
                  '  "action": "installed",',
                  '  "pinned": false,',
                  '  "approvalRequired": true,',
                  '  "approveCommand": "' + APPROVE + TENANT + '"',
                  '}']
              }]
            },
            {
              actors: ['c-approval', 'approvals'], from: 'cli', to: 'approvals', label: 'no record for ' + TENANT, status: 'fail', dim: ['c-session', 'c-proxy', 'c-spawn', 'proxy', 'plugin', 'mgmt'],
              title: 'Running it is refused until a person approves',
              text: 'The agent runs `auth0 checkmate`. The binary and index checks pass, and then the approval check fails, before any login.',
              data: [{ label: 'stderr', lines: ['{"error":{"code":"validation","reason":"plugin_not_approved","message":"checkmate is not approved for ' + TENANT + '. Run: ' + APPROVE + TENANT + '"}}'] }]
            },
            {
              actors: ['user', 'cli'], from: 'user', to: 'cli', label: 'approve, in agent mode', status: 'fail',
              title: 'The agent cannot approve it either',
              text: 'The installed copy is checked first. Then the refusal comes before the consent screen, so nothing is written. A person approves in a terminal, or a pipeline pins the permissions with `AUTH0_CLI_PLUGIN_APPROVALS` (see the CI flow).',
              data: [{ label: 'stderr', lines: ['{"error":{"code":"unknown","reason":"unclassified","message":"only a person can approve a plugin. Run this in a terminal: ' + APPROVE + TENANT + '"}}'] }]
            }
          ]
        },
        {
          id: 'agent-run', group: 'Agent mode and CI', name: 'Agent runs a plugin',
          command: 'auth0 checkmate --validators a   (run by an AI agent, already approved)',
          summary: 'The checks are the same as for a person. The output is JSON, a failure is one JSON error, and the plugin gets agent mode in its environment.',
          steps: [
            {
              actors: ['user', 'cli'], from: 'user', to: 'cli', label: 'auth0 checkmate --validators a',
              title: 'An agent runs an approved plugin',
              text: 'The plugin was approved by a person on ' + TENANT + ' earlier. The agent passes no flags for it. Agent mode comes from the environment.',
              data: [{ label: 'Command line', lines: ['$ auth0 checkmate --validators a', 'agent mode: on (JSON output, no prompts, no color)'] }]
            },
            {
              actors: ['c-prescan'], from: 'c-prescan', to: 'c-prescan',
              title: 'Pre-scan passes agent mode on as variables',
              text: 'The plugin cannot read the flags of the CLI, so the CLI turns them into variables. The plugin should print JSON and must not ask questions.',
              data: [{ label: 'Environment of the plugin', lines: [ok('AUTH0_PLUGIN_AGENT_MODE=true'), ok('AUTH0_PLUGIN_NO_INPUT=true'), ok('AUTH0_PLUGIN_NO_COLOR=true'), ok('AUTH0_PLUGIN_JSON=true')] }]
            },
            RUN_VERIFY, RUN_REGISTRY,
            {
              actors: ['c-call'], from: 'c-call', to: 'c-call',
              title: 'The command must declare that it supports agent mode',
              text: 'A command works in agent mode only if its manifest says `output.agentMode: true`, and it works with `--json` only if it says `output.json: true`. The plugin author promises that the output is JSON and that nothing waits for input.',
              data: [{
                label: 'Checks', lines: [
                  ok('checkmate declares output.agentMode: true'),
                  note('Without it: {"error":{"code":"usage","reason":"plugin_output_unsupported","message":"auth0 checkmate does not support agent mode (turn it off with --agent-mode=false), because its manifest does not declare output.agentMode"}}')]
              }]
            },
            RUN_APPROVAL, RUN_SESSION, RUN_PROXY, RUN_SPAWN,
            {
              actors: ['plugin', 'cli'], from: 'plugin', to: 'cli', label: 'JSON on stdout, exit 0',
              title: 'The plugin prints JSON and exits 0',
              text: 'The output goes straight to stdout, untouched. The CLI adds nothing to it, and it prints no summary line of denied calls in agent mode.',
              data: [{ label: 'stdout (written by the plugin)', lines: ['{"validators":["a"],"findings":[]}'] }]
            },
            {
              actors: ['plugin', 'cli'], from: 'plugin', to: 'cli', label: 'exit 7', status: 'fail',
              title: 'If the plugin fails, the CLI wraps it in one JSON error',
              text: 'A plugin that exits non-zero, runs out of time, is interrupted or cannot start becomes one error with class "plugin" and a reason. The CLI exits 1, and `details.exitCode` has the code of the plugin. If the proxy denied calls, their summary is in `details.deniedCalls`.',
              data: [{
                label: 'stderr', lines: [
                  '{"error":{"code":"plugin","reason":"plugin_exit","message":"checkmate 1.0.0 exited with code 7","details":{"pluginId":"auth0/checkmate","version":"1.0.0","exitCode":7}}}']
              }]
            }
          ]
        },
        {
          id: 'agent-interactive', group: 'Agent mode and CI', name: 'Agent and an interactive command',
          command: 'auth0 checkmate wizard   (a command declared interactive, run by an agent)',
          summary: 'A command that needs a person at the keyboard is refused in agent mode with a structured error, before any approval or login.',
          neverReached: ['Approval check', 'Session / login', 'Proxy start', 'Spawn plugin'],
          neverActors: ['c-approval', 'c-session', 'c-proxy', 'c-spawn', 'approvals', 'proxy', 'plugin', 'mgmt'],
          protectedNote: 'Protected: nothing was approved, no login, no proxy, and the plugin never started.',
          steps: [
            {
              actors: ['user', 'cli'], from: 'user', to: 'cli', label: 'auth0 checkmate wizard',
              title: 'An agent runs a command that is declared interactive',
              text: '`wizard` is a made up example. Its manifest says `"interactive": true`. The caller is an AI agent, so agent mode is on.',
              data: [{ label: 'Command line', lines: ['$ auth0 checkmate wizard', 'manifest: { "name": "wizard", "interactive": true }'] }]
            },
            {
              actors: ['c-prescan'], from: 'c-prescan', to: 'c-prescan',
              title: 'Pre-scan finds the command and the CLI flags',
              text: 'The CLI applies agent mode to itself. The plugin would get `AUTH0_PLUGIN_AGENT_MODE=true` and the name of the subcommand `wizard` as its first argument, if it ran.',
              data: [{ label: 'Arguments', lines: ['plugin would get: wizard', ok('AUTH0_PLUGIN_AGENT_MODE=true')] }]
            },
            RUN_VERIFY, RUN_REGISTRY,
            {
              actors: ['c-call'], from: 'c-call', to: 'user', label: 'error, exit 1', status: 'fail',
              title: 'Refused: an interactive command cannot run in agent mode',
              text: 'Commands declared interactive are refused in agent mode, with `--no-input`, and when input or output is not a terminal. The refusal is a usage error, and it comes before the approval check.',
              data: [
                { label: 'Agent mode (stderr)', lines: ['{"error":{"code":"usage","reason":"plugin_interactive","message":"auth0 checkmate wizard is interactive, so it needs a terminal. It cannot run in agent mode, with --no-input, or when input or output is not a terminal"}}'] },
                { label: 'Human mode, no terminal (stderr)', lines: ['Auth0 checkmate wizard is interactive, so it needs a terminal. It cannot run in agent mode, with --no-input, or when input or output is not a terminal.'] }
              ]
            }
          ]
        },
        {
          id: 'ci-pin', group: 'Agent mode and CI', name: 'CI with an approval pin',
          command: 'AUTH0_CLI_PLUGIN_APPROVALS=auth0/checkmate@sha256:<hash> auth0 checkmate --agent-mode',
          summary: 'A pipeline runs a plugin without a person, because a person pinned the exact permissions hash. Pins are never written to disk.',
          steps: [
            {
              actors: ['user', 'cli'], from: 'user', to: 'cli', label: 'auth0 checkmate --agent-mode --validators a',
              title: 'A CI job runs the plugin with a pin',
              text: 'The job signs in from the environment and sets a pin. A person created the pin earlier from the permissions hash of the plugin, which `auth0 plugin info` shows.',
              data: [{
                label: 'CI environment', lines: [
                  'AUTH0_CLI_AUTH_MODE=env',
                  'AUTH0_DOMAIN=' + TENANT + '   AUTH0_CLIENT_ID and AUTH0_CLIENT_SECRET set',
                  'AUTH0_CLI_PLUGIN_APPROVALS=auth0/checkmate@sha256:<64 hex digits>',
                  '$ auth0 checkmate --agent-mode --validators a']
              }]
            },
            {
              actors: ['c-prescan'], from: 'c-prescan', to: 'c-prescan',
              title: 'Pre-scan: agent mode and the tenant from the environment',
              text: 'In env mode the tenant comes from `AUTH0_DOMAIN`, unless `--tenant` is given and matches. The CLI-owned `--agent-mode` is removed and passed on as `AUTH0_PLUGIN_AGENT_MODE=true`.',
              data: [{ label: 'Arguments', lines: ['plugin gets: --validators a', ok('AUTH0_PLUGIN_AGENT_MODE=true'), ok('AUTH0_PLUGIN_TENANT=' + TENANT)] }]
            },
            RUN_VERIFY,
            RUN_REGISTRY,
            RUN_CALLCHECK,
            {
              actors: ['c-approval', 'approvals'], from: 'cli', to: 'approvals', label: 'pin matches the permissions hash',
              title: 'The pin approves the plugin, even in agent mode',
              text: 'The pin is read strictly: a typo is an error, never "no pins". If its hash equals the permissions hash of the installed manifest, the plugin counts as approved on every tenant without touching approvals.json.',
              data: [{
                label: 'Approval check', lines: [
                  ok('pin auth0/checkmate@sha256:<hash> equals the installed permissions hash'),
                  no('nothing is written to approvals.json: pins are never saved'),
                  note('A bad entry in the variable is an error that names it.')]
              }]
            },
            {
              actors: ['c-session'], from: 'c-session', to: 'c-session',
              title: 'Session from the environment credentials',
              text: 'The environment credentials are exchanged for a token only now, after the approval check. The client secret stays in the CLI. The plugin never receives it.',
              data: [{ label: 'Token handling', lines: [ok('token held in CLI memory for the proxy'), no('AUTH0_CLIENT_SECRET is not in the plugin environment')] }]
            },
            RUN_PROXY,
            RUN_SPAWN,
            {
              actors: ['plugin', 'cli'], from: 'plugin', to: 'cli', label: 'exit 0',
              title: 'The plugin finishes without a person',
              text: 'The proxy applied the same scope table and secret stripping as in a run by a person. The job gets the output of the plugin on stdout and, on failure, the JSON error.',
              data: [{ label: 'Result', lines: ['exit code 0'] }]
            },
            {
              actors: ['c-approval', 'approvals'], from: 'cli', to: 'approvals', label: 'pin no longer matches', status: 'fail', dim: ['c-session', 'c-proxy', 'c-spawn', 'proxy', 'plugin', 'mgmt'],
              title: 'Later: a release changes the permissions, so the old pin stops matching',
              text: 'A pin is bound to a permissions hash, not to a name. If a version with different permissions gets installed, the run is refused until a person pins the new hash.',
              data: [{ label: 'stderr', lines: ['{"error":{"code":"validation","reason":"plugin_not_approved","message":"checkmate is not approved for ' + TENANT + '. Run: ' + APPROVE + TENANT + '"}}', note('In CI the fix is to update the pin, not to approve.')] }]
            }
          ]
        },
        // ===== Build a plugin =====
        {
          id: 'dev-create', group: 'Build a plugin', name: 'Create a plugin project',
          command: DEV + 'auth0 plugin dev create checkmate --lang go --author "Platform team" --bugs https://github.com/acme/checkmate/issues',
          summary: 'The author tools exist only in dev mode. Create writes a manifest and a small starter project in Go or TypeScript, and never overwrites anything.',
          steps: [
            {
              actors: ['user', 'cli'], from: 'user', to: 'cli', label: 'plugin dev create checkmate --lang go',
              title: 'The author runs create',
              text: 'The `plugin dev` commands are for plugin authors and exist only when `AUTH0_CLI_PLUGIN_DEV_MODE=true` is set. Missing values are asked for in a terminal and are an error otherwise. They need no login and no registry.',
              data: [{
                label: 'Command', lines: [
                  '$ ' + DEV + 'auth0 plugin dev create checkmate --lang go \\',
                  '    --author "Platform team" --bugs https://github.com/acme/checkmate/issues',
                  note('Without --author in a script: "--author is required: Team or person that owns the plugin. It becomes owner.team in plugin.json."')]
              }]
            },
            {
              actors: ['c-scaffold'], from: 'c-scaffold', to: 'c-scaffold',
              title: 'Check the name and the folder',
              text: 'The name is the short name of the plugin: lowercase letters, digits and hyphens, starting with a letter. The plugin will run as `auth0 checkmate`. The folder may not exist or must be empty. An existing project is never overwritten.',
              data: [{ label: 'Checks', lines: [ok('name checkmate is valid'), ok('./checkmate does not exist'), ok('id defaults to auth0/checkmate')] }]
            },
            {
              actors: ['cli', 'project'], from: 'cli', to: 'project', label: 'write plugin.json and the starter files',
              title: 'Write the project',
              text: 'The CLI writes the manifest and a starter that calls the Management API through the proxy. The Go starter has 27 files and the TypeScript starter has 31. With `--from` the starters come from a folder of your own.',
              data: [{
                label: 'Go starter (top level)', lines: [
                  'plugin.json            the manifest: id, version 0.1.0, platforms, commands, permissions',
                  'main.go, commands/, plugin/, go.mod, .gitignore',
                  'Makefile, .goreleaser.yaml, .golangci.yml, README.md',
                  '.github/workflows/ci.yml and release.yml, plus .github/actions, .github/scripts and dependabot.yml',
                  'scripts/release/       the program behind make release']
              }]
            },
            {
              actors: ['cli', 'user'], from: 'cli', to: 'user', label: 'Created auth0/checkmate',
              title: 'Created, with the next steps',
              text: 'The CLI prints what to do next. The starter asks for the scope `read:clients` only. You add scopes, hosts and files to `plugin.json` as the plugin needs them.',
              data: [{
                label: 'Output', lines: [
                  'Created auth0/checkmate in /path/to/checkmate (27 files).',
                  'Next:',
                  '  cd checkmate',
                  '  go mod tidy && go build -o checkmate .',
                  '  AUTH0_CLI_PLUGIN_DEV_MODE=true auth0 plugin dev link',
                  '  auth0 plugin approve checkmate',
                  '  auth0 checkmate',
                  'To publish it, push the project to a new GitHub repository: the first push to main releases version 0.1.0. Release again with: make release BUMP=minor']
              }]
            }
          ]
        },
        {
          id: 'dev-validate', group: 'Build a plugin', name: 'Validate the manifest',
          command: DEV + 'auth0 plugin dev validate ./checkmate',
          summary: 'Validate checks plugin.json against the schema and the rules the CLI applies, without a login or the registry. It lists every problem with its path.',
          steps: [
            {
              actors: ['user', 'cli'], from: 'user', to: 'cli', label: 'plugin dev validate ./checkmate',
              title: 'The author runs validate',
              text: 'The folder is optional and defaults to the current one. Nothing is installed, linked or run.',
              data: [{ label: 'Command', lines: ['$ ' + DEV + 'auth0 plugin dev validate ./checkmate'] }]
            },
            {
              actors: ['c-manifest', 'project'], from: 'cli', to: 'project', label: 'read plugin.json',
              title: 'Read and check the manifest',
              text: 'The CLI reads plugin.json and checks it against the plugin schema, then against the rules that apply at install, such as scope names, timeouts and platform names. It reports every problem, not only the first.',
              data: [{ label: 'Checks', lines: ['schema: field names, types, patterns', 'scopes look like verb:resource', 'timeouts are durations such as 5m', 'platforms are known names'] }]
            },
            {
              actors: ['cli', 'user'], from: 'cli', to: 'user', label: 'The plugin manifest has 2 problems.', status: 'fail',
              title: 'Problems are listed with their path',
              text: 'Each row names the field and what is wrong with it. The command exits 1. With `--json` or in agent mode the same list is printed as JSON, with `valid: false`, and the error has the reason `plugin_manifest_invalid`.',
              data: [{
                label: 'Output', lines: [
                  '  PATH                               PROBLEM',
                  '  commands[0].permissions.scopes[1]  \'nope\' does not match pattern \'^[a-z]+:[a-z0-9_]+$\'',
                  '  commands[0].timeout                \'5 minutes\' does not match pattern \'^([0-9]+(\\\\.[0-9]+)?(ns|us|ms|s|m|h))+$\'',
                  '',
                  '===  error',
                  '',
                  'The plugin manifest has 2 problems.']
              }]
            },
            {
              actors: ['cli', 'user'], from: 'cli', to: 'user', label: 'The plugin manifest is valid.',
              title: 'After the fix: valid',
              text: 'The author edits the two fields and runs validate again. The command exits 0.',
              data: [{ label: 'Output', lines: ['The plugin manifest is valid.'] }]
            }
          ]
        },
        {
          id: 'dev-link', group: 'Build a plugin', name: 'Link and run a local build',
          command: DEV + 'auth0 plugin dev link',
          summary: 'Link runs your local build as `auth0 <name>` through the same proxy, permissions and approval as an installed plugin. Only the checks that make no sense for an unpublished build are skipped.',
          steps: [
            {
              actors: ['user', 'project'], from: 'user', to: 'project', label: 'go build -o checkmate .',
              title: 'The author builds the plugin',
              text: 'The build is an ordinary executable in the project folder, named like the plugin. For another file, pass `--bin`.',
              data: [{ label: 'Commands', lines: ['$ cd checkmate', '$ go mod tidy && go build -o checkmate .'] }]
            },
            {
              actors: ['user', 'cli'], from: 'user', to: 'cli', label: 'plugin dev link',
              title: 'The author links the folder',
              text: 'The folder is optional and defaults to the current one. Linking an already linked plugin replaces the link, and a linked plugin takes the place of an installed one with the same name.',
              data: [{ label: 'Command', lines: ['$ ' + DEV + 'auth0 plugin dev link', note('With another file: auth0 plugin dev link ./checkmate --bin ./checkmate/dist/checkmate')] }]
            },
            {
              actors: ['c-link', 'project'], from: 'cli', to: 'project', label: 'read plugin.json, check the executable',
              title: 'Read the manifest and check the executable',
              text: 'The folder must hold a valid plugin.json, and the executable must be a regular file that can run. If the build is missing, the CLI says so and nothing is linked.',
              data: [{
                label: 'Checks', lines: [
                  ok('plugin.json is valid'),
                  ok('./checkmate/checkmate is a regular, executable file'),
                  note('Without a build: "The executable of the plugin cannot be used: stat ...: no such file or directory. Build it there, or give another file with --bin."')]
              }]
            },
            {
              actors: ['cli', 'devlinks'], from: 'cli', to: 'devlinks', label: 'add the link',
              title: 'Save the link',
              text: 'The link is one entry in a small file next to the plugin store. It holds the folder and the executable, not a copy of either.',
              data: [{ label: 'plugins/dev-links.json', lines: [add('auth0/checkmate: the folder and the executable of the build')] }]
            },
            {
              actors: ['cli', 'user'], from: 'cli', to: 'user', label: 'Linked auth0/checkmate 0.1.0',
              title: 'Linked, and it is not approved yet',
              text: 'A person must approve the permissions of the build, just as for an installed plugin. Dev mode does not change that.',
              data: [{
                label: 'Output', lines: [
                  'Linked auth0/checkmate 0.1.0 from /path/to/checkmate. Run it with: auth0 checkmate',
                  'Every run prints this line on stderr: Running the linked build of auth0/checkmate 0.1.0 from /path/to/checkmate. Its files are not checked against a release.',
                  'auth0/checkmate is not approved on any tenant. Log in with auth0 login, then approve it in a terminal with: auth0 plugin approve auth0/checkmate --tenant <tenant>']
              }]
            },
            {
              actors: ['c-consent', 'user'], from: 'user', to: 'cli', label: 'plugin approve checkmate --tenant ' + TENANT,
              title: 'The author approves the build on a tenant',
              text: 'The consent screen shows the permissions of the manifest as it is on disk now. Approval is stored like any other, keyed to the permissions hash.',
              data: [{ label: 'Command', lines: ['$ auth0 plugin approve checkmate --tenant ' + TENANT, 'Approved auth0/checkmate 0.1.0 for ' + TENANT + '.'] }]
            },
            {
              actors: ['c-verify', 'project'], from: 'cli', to: 'project', label: 'read the manifest and the executable',
              title: 'Run: the files are read as they are',
              text: 'On `auth0 checkmate` the CLI prints the banner on stderr, then reads the manifest and the executable from the folder. It does not hash them and it does not use the lockfile, because a build changes all the time. A rebuild needs no new link.',
              data: [{
                label: 'Skipped for a linked build', lines: [
                  no('hashes of the files'),
                  no('the lockfile'),
                  no('the revocation list, and usage events'),
                  note('Everything else runs as for an installed plugin: the index check, the call check, the approval, the proxy and its scopes, secret stripping, the environment and the timeout.')]
              }]
            },
            {
              actors: ['c-approval', 'approvals'], from: 'cli', to: 'approvals', label: 'permissions hash of the manifest on disk', status: 'fail', dim: ['c-session', 'c-proxy', 'c-spawn', 'proxy', 'plugin', 'mgmt'],
              title: 'Editing the permissions asks for a new approval',
              text: 'If the author adds a scope to plugin.json, the permissions hash changes and the old approval no longer matches. The run stops until the author approves again. The manifest is read a second time after the login, so a build edited in between cannot run with more than was approved.',
              data: [{ label: 'Message (stderr)', lines: ['Checkmate changed what it asks for since you approved it for ' + TENANT + '. Run: ' + APPROVE + TENANT + '.'] }]
            },
            {
              actors: ['c-link'], from: 'c-link', to: 'c-link',
              title: 'Unlink when you are done',
              text: 'Unlink removes the entry from dev-links.json. An installed plugin with the same name, if any, runs again.',
              data: [{ label: 'Command', lines: ['$ ' + DEV + 'auth0 plugin dev unlink auth0/checkmate', 'Unlinked auth0/checkmate.'] }]
            }
          ]
        },
        // ===== Release a plugin =====
        {
          id: 'release', group: 'Release a plugin', name: 'Release from the starter template',
          command: 'make release BUMP=patch      (TypeScript starter: bun run release)',
          summary: 'One command opens a release pull request. Merging it builds the archives for every platform and publishes a GitHub release. Signing, attestation and publishing to the catalog are not part of it yet.',
          steps: [
            {
              actors: ['user', 'project'], from: 'user', to: 'project', label: 'make release BUMP=patch',
              title: 'The author starts the release',
              text: 'Nothing is edited by hand. The command needs `git`, `gh` (signed in with `gh auth login`) and a clean `main` that matches `origin`. Use `VERSION=1.2.3` for an exact version, or `DRY_RUN=1` to see the steps without changing anything.',
              data: [{ label: 'Commands', lines: ['$ make release BUMP=patch', '$ make release VERSION=1.2.3', '$ make release BUMP=patch DRY_RUN=1'] }]
            },
            {
              actors: ['project', 'repo'], from: 'project', to: 'repo', label: 'branch release/v0.1.1, pull request',
              title: 'It opens a release pull request',
              text: 'The program creates the branch, raises `version` in plugin.json, commits with your own git settings, pushes and opens the pull request.',
              data: [{ label: 'What it does', lines: ['branch   release/v<version>', 'commit   chore: release v<version>', 'file     plugin.json, version raised', 'opens    a pull request on GitHub'] }]
            },
            {
              actors: ['repo'], from: 'repo', to: 'repo',
              title: 'Someone merges the pull request',
              text: 'The release is decided by the merge. The Release workflow listens for a change to plugin.json on `main`. The first push of a new project releases the version the starter wrote, 0.1.0.',
              data: [{ label: 'Trigger', lines: ['push to main that changes plugin.json', 'or: run the workflow by hand on main to finish a release that failed half way'] }]
            },
            {
              actors: ['repo', 'actions'], from: 'repo', to: 'actions', label: 'Release workflow starts',
              title: 'The workflow reads the version',
              text: 'The first job reads the version from plugin.json and works out the tag, `v<version>`. It also checks whether that release already exists. If it does, the rest is skipped, so running the workflow again never changes a finished release.',
              data: [{ label: 'Job: version', lines: ['tag = v<version> from plugin.json', 'release exists? then skip everything else'] }]
            },
            {
              actors: ['actions'], from: 'actions', to: 'actions',
              title: 'The tests run on the merged commit',
              text: 'The workflow calls `ci.yml`: vet, test and build on Linux, macOS and Windows, and the linter. If anything fails, nothing is published.',
              data: [{ label: 'Job: ci', lines: ['go vet, go test, go build on ubuntu, macos and windows', 'golangci-lint'] }]
            },
            {
              actors: ['actions'], from: 'actions', to: 'actions',
              title: 'It builds one archive for each platform',
              text: 'The Go starter uses GoReleaser. The TypeScript starter builds with `bun build --compile` once for each platform. Both give seven archives. Each archive holds exactly the executable and plugin.json at its root, because that is all the CLI accepts.',
              data: [{
                label: 'Archives', lines: [
                  'checkmate_0.1.1_darwin-arm64.tar.gz, darwin-x64',
                  'checkmate_0.1.1_linux-x64.tar.gz, linux-arm64',
                  'checkmate_0.1.1_linux-x64-musl.tar.gz, linux-arm64-musl',
                  'checkmate_0.1.1_windows-x64.zip',
                  'checksums.txt  (SHA-256 of every archive)']
              }]
            },
            {
              actors: ['actions', 'release'], from: 'actions', to: 'release', label: 'tag v0.1.1, upload archives',
              title: 'It publishes a GitHub release',
              text: 'The workflow tags the commit, creates the release with notes written by GitHub, and uploads the archives and `checksums.txt`. A second run replaces the assets of the first, which finishes a half made release.',
              data: [{ label: 'Result', lines: [ok('GitHub release v0.1.1 with 7 archives and checksums.txt')] }]
            },
            {
              actors: ['release', 'registry'], from: 'release', to: 'registry', label: 'planned', status: 'info',
              title: 'Here the template stops',
              text: 'Signing the release, a build attestation, and listing the plugin in the registry catalog are planned. Until a plugin is listed, `auth0 plugin install` cannot find it. During development you use `plugin dev link`.',
              data: [{ label: 'Planned', lines: ['signing of releases', 'build attestation', 'listing in the registry catalog'] }]
            }
          ]
        },
        // ===== Housekeeping =====
        {
          id: 'uninstall', group: 'Housekeeping', name: 'Uninstall',
          command: 'auth0 plugin uninstall checkmate',
          summary: 'A person confirms, the store removes every version, and the approvals on every tenant go with it. An agent needs --force.',
          steps: [
            {
              actors: ['user', 'cli'], from: 'user', to: 'cli', label: 'auth0 plugin uninstall checkmate',
              title: 'Run the uninstall command',
              text: 'Uninstall works without the registry, with plugins turned off, and with wrong approval pins, so a broken setup can always be cleaned up. `remove` is an alias.',
              data: [{ label: 'Command', lines: ['$ auth0 plugin uninstall checkmate'] }]
            },
            {
              actors: ['cli', 'store'], from: 'cli', to: 'store', label: 'is it installed?',
              title: 'The CLI checks the lockfile',
              text: 'If the plugin is not in the lockfile, the command stops with an error that says so, and nothing changes.',
              data: [{ label: 'Lockfile', lines: [ok('auth0/checkmate is installed'), note('Otherwise: "the plugin auth0/checkmate is not installed. Installed plugins: <names>". With none installed: "the plugin auth0/checkmate is not installed, and no plugin is. Find one with: auth0 plugin search".')] }]
            },
            {
              actors: ['cli', 'user'], from: 'cli', to: 'user', label: 'confirmation prompt',
              title: 'Ask for confirmation',
              text: 'In a terminal the person is asked first. `--force` skips the question.',
              data: [{ label: 'Prompt', lines: ['Are you sure you want to uninstall auth0/checkmate? This removes every version and its approvals.'] }]
            },
            {
              actors: ['user', 'cli'], from: 'user', to: 'cli', label: 'yes',
              title: 'The person confirms',
              text: 'Answering no leaves everything as it was and prints that the plugin was not uninstalled.',
              data: [{ label: 'Answer', lines: ['yes', note('If no: "auth0/checkmate was not uninstalled."')] }]
            },
            {
              actors: ['cli', 'store'], from: 'cli', to: 'store', label: 'remove versions and lockfile entry',
              title: 'The store removes the plugin',
              text: 'Every version folder and the lockfile entry are removed while the store is locked, and the freed size is counted.',
              data: [{ label: 'Removed', lines: [del('plugins/store/auth0/checkmate/1.1.0/'), del('plugins/store/auth0/checkmate/1.0.0/'), del('plugins.lock.json entry for auth0/checkmate')] }]
            },
            {
              actors: ['cli', 'approvals'], from: 'cli', to: 'approvals', label: 'remove approvals for the plugin',
              title: 'The approvals of that plugin are removed',
              text: 'Approvals can only shrink here. The approvals of the plugin on every tenant go, and a damaged approvals file is deleted instead of blocking the uninstall.',
              data: [{ label: 'approvals.json', lines: [del(TENANT + ' / auth0/checkmate'), del(OTHER + ' / auth0/checkmate')] }]
            },
            {
              actors: ['cli', 'user'], from: 'cli', to: 'user', label: 'Uninstalled auth0/checkmate and freed 8.5 MB.',
              title: 'The space that was freed is printed',
              text: 'Installing it again later starts from scratch, including a new consent screen. The CLI also sends one usage event for the uninstall, unless you opted out.',
              data: [{ label: 'Output', lines: ['Uninstalled auth0/checkmate and freed 8.5 MB.'] }]
            },
            {
              actors: ['cli'], from: 'cli', to: 'user', label: 'refused without --force', status: 'fail', dim: ['store', 'approvals'],
              title: 'What if an agent runs it without --force?',
              text: 'In agent mode nobody can confirm, so the command refuses and removes nothing. With `--force` it goes ahead without a prompt. Outside agent mode, with `--no-input` or without a terminal, nothing can be asked, so the command removes the plugin without one.',
              data: [{ label: 'stderr (agent mode, no --force)', lines: ['{"error":{"code":"unknown","reason":"unclassified","message":"this is a destructive command; re-run with --force to proceed without a confirmation prompt"}}'] }]
            }
          ]
        },
        {
          id: 'logout', group: 'Housekeeping', name: 'Logout removes approvals',
          command: 'auth0 logout ' + TENANT,
          summary: 'Consent never outlives the session it was given in. Logging out deletes the approvals of the tenant first.',
          steps: [
            {
              actors: ['user', 'cli'], from: 'user', to: 'cli', label: 'auth0 logout ' + TENANT,
              title: 'Log out of a tenant',
              text: 'The person logs out of ' + TENANT + '. The plugin stays installed, and the approvals on other tenants are not affected.',
              data: [{ label: 'Command', lines: ['$ auth0 logout ' + TENANT] }]
            },
            {
              actors: ['cli', 'approvals'], from: 'cli', to: 'approvals', label: 'remove approvals of ' + TENANT,
              title: 'The approvals of that tenant are deleted first',
              text: 'They go before the tenant and its secrets. If something fails, the tenant is still logged in and logout can simply be tried again.',
              data: [{ label: 'approvals.json', lines: [del(TENANT + ' / auth0/checkmate / sha256:<H1>'), OTHER + ' / auth0/checkmate / sha256:<H1>   (kept)'] }]
            },
            {
              actors: ['cli'], from: 'cli', to: 'cli',
              title: 'The tenant and its keyring secrets are removed',
              text: 'Only then does the CLI remove the tenant from its config and delete the secrets of the tenant from the operating system keyring.',
              data: [{ label: 'Removed', lines: [del('tenant ' + TENANT + ' from config'), del('keyring secrets for ' + TENANT)] }]
            },
            {
              actors: ['cli', 'user'], from: 'cli', to: 'user', label: 'Successfully logged out',
              title: 'Logged out',
              text: 'The CLI confirms the logout.',
              data: [{ label: 'Output', lines: ['Successfully logged out from tenant: ' + TENANT] }]
            },
            {
              actors: ['user', 'cli'], from: 'user', to: 'cli', label: 'auth0 login, then auth0 checkmate',
              title: 'Later: the person logs in again and runs the plugin',
              text: 'The binary and the index are still fine, and the same plugin is still installed. The old approval is gone, though.',
              data: [{ label: 'Commands', lines: ['$ auth0 login', '$ ' + RUN_CMD] }]
            },
            {
              actors: ['c-approval', 'approvals'], from: 'cli', to: 'approvals', label: 'no record for ' + TENANT, status: 'fail', dim: ['c-session', 'c-proxy', 'c-spawn', 'proxy', 'plugin', 'mgmt'],
              title: 'Logging in again means approving again',
              text: 'Consent was given in the old session, so it was deleted with it. The plugin will not run on this tenant until a person approves it again.',
              data: [{ label: 'Message (stderr)', lines: ['Checkmate is not approved for ' + TENANT + '. Run: ' + APPROVE + TENANT + '.'] }]
            }
          ]
        }
      ];
      // ---------------------------------------------------------------------------
      // Rendering
      // ---------------------------------------------------------------------------
      var W = 800;          // viewBox width. The docs column is about 800 px, so 1 unit is about 1 px.
      var ZT = 8;           // top of the zone frames
      var CT = 62;          // top of the content area (room for the zone label and the trust badges)
      var MARGIN = 8, PAD = 14, ZGAP = 14, SLOT = 36, BOX_H = 64, CHECK_H = 26, ROW_GAP = 64;
      var BADGE = { trusted: 'trusted', untrusted: 'plugin code', neutral: 'outside' };
      var STATUS_TEXT = { ok: '✓ OK', fail: '✕ Blocked or failed', denied: '✕ Call denied', info: 'ℹ Planned' };
      var DRAW_MS = 400, DOT_MS = 900, STEP_MS = 1800;
      var svg = document.getElementById('plugin-flow-diagram');
      var root = svg.closest('.plugin-flow');
      var NS = svg.namespaceURI;
      var mq = window.matchMedia ? window.matchMedia('(prefers-reduced-motion: reduce)') : null;
      var reduced = !!(mq && mq.matches);
      var el = {};
      ['scenario', 'speed', 'prev', 'play', 'next', 'dots', 'summary',
        'p-count', 'p-status', 'p-title', 'p-text', 'p-protected', 'p-data',
        'g-zones', 'g-edges', 'g-nodes', 'g-wire', 'g-packet'].forEach(function (id) { el[id] = document.getElementById('plugin-flow-' + id); });
      function mk(tag, attrs, parent, text) {
        var n = document.createElementNS(NS, tag);
        if (attrs) Object.keys(attrs).forEach(function (k) { n.setAttribute(k, attrs[k]); });
        if (text !== undefined) n.textContent = text;
        if (parent) parent.appendChild(n);
        return n;
      }
      function h(tag, cls, text) {
        var n = document.createElement(tag);
        if (cls) n.className = cls;
        if (text !== undefined) n.textContent = text;
        return n;
      }
      // Text with `code` spans, built with DOM nodes only.
      function richText(parent, str) {
        str.split('`').forEach(function (part, i) {
          if (i % 2 === 1) parent.appendChild(h('code', '', part));
          else parent.appendChild(document.createTextNode(part));
        });
      }
      function resolve(id) { return NODES[id] && NODES[id].parent ? NODES[id].parent : id; }
      // Rough text width in viewBox units for the system font stack.
      function tw(str, px) { return str.length * px * 0.54; }
      function wrap(str, max) {
        var lines = [], cur = '';
        str.split(' ').forEach(function (word) {
          if (cur && (cur + ' ' + word).length > max) { lines.push(cur); cur = word; }
          else cur = cur ? cur + ' ' + word : word;
        });
        if (cur) lines.push(cur);
        return lines;
      }
      function labelBox(label) {
        var lines = wrap(label, 28), w = 0;
        lines.forEach(function (l) { w = Math.max(w, tw(l, 14)); });
        return { lines: lines, w: Math.ceil(w) + 20, h: lines.length * 18 + 10 };
      }
      // The checks this scenario uses, in the order they first appear.
      var checkCache = {};
      function checksOf(s) {
        if (!checkCache[s.id]) {
          var out = [];
          s.steps.forEach(function (st) {
            st.actors.forEach(function (a) { if (NODES[a].chip && out.indexOf(a) === -1) out.push(a); });
          });
          checkCache[s.id] = out;
        }
        return checkCache[s.id];
      }
      function isFailing(st) { return st.status === 'fail' || st.status === 'denied'; }
      // The boxes a step shows: its actors plus the two ends of the arrow, with checks folded into the CLI.
      function visibleIds(st) {
        var out = [];
        function add(id) { var r = resolve(id); if (out.indexOf(r) === -1) out.push(r); }
        st.actors.forEach(add);
        add(st.from);
        add(st.to);
        return out;
      }
      function nodeSize(id, nChecks) {
        var n = NODES[id];
        if (n.container) return nChecks ? { w: 214, h: 50 + nChecks * CHECK_H + 12 } : { w: 170, h: BOX_H };
        var w = Math.max(tw(n.name, 15) * 1.1, n.sub ? tw(n.sub, 14) : 0) + 28;
        return { w: Math.ceil(Math.min(230, Math.max(150, w))), h: BOX_H };
      }
      // Places the visible boxes. y values are relative to the top of the content area.
      function layout(s, st) {
        var ids = visibleIds(st), nChecks = checksOf(s).length;
        var size = {}, colW = {}, rowSet = {}, zoneOfCol = {}, i;
        ids.forEach(function (id) {
          var n = NODES[id];
          size[id] = nodeSize(id, nChecks);
          colW[n.col] = Math.max(colW[n.col] || 0, size[id].w);
          zoneOfCol[n.col] = n.zone;
          if (!n.container) rowSet[n.row] = true;
        });
        var num = function (a, b) { return a - b; };
        var cols = Object.keys(colW).map(Number).sort(num);
        var rows = Object.keys(rowSet).map(Number).sort(num);
        var hasCli = ids.indexOf('cli') !== -1;
        var span = rows.length ? (rows.length - 1) * (BOX_H + ROW_GAP) + BOX_H : BOX_H;
        var hc = Math.max(span, hasCli ? size.cli.h : BOX_H);
        // Gaps between neighbouring columns. The gap that holds the arrow label is widened.
        var slots = [];
        for (i = 1; i < cols.length; i++) slots.push(zoneOfCol[cols[i]] === zoneOfCol[cols[i - 1]] ? SLOT : 2 * PAD + ZGAP);
        var ea = edgeEnds(st);
        if (ea && st.label) {
          var ia = cols.indexOf(NODES[ea[0]].col), ib = cols.indexOf(NODES[ea[1]].col);
          if (Math.abs(ia - ib) === 1) {
            var k = Math.min(ia, ib);
            slots[k] = Math.max(slots[k], labelBox(st.label).w + 28);
          }
        }
        var sumCols = 0, sumSlots = 0;
        cols.forEach(function (c) { sumCols += colW[c]; });
        slots.forEach(function (g) { sumSlots += g; });
        var extra = W - (2 * MARGIN + 2 * PAD + sumCols + sumSlots), left = MARGIN + PAD;
        if (slots.length && extra < 0) {
          for (i = 0; i < slots.length; i++) slots[i] = Math.max(24, slots[i] + extra / slots.length);
        } else if (extra > 0) {
          if (slots.length) {
            var add = Math.min(extra / slots.length, 110);
            for (i = 0; i < slots.length; i++) slots[i] += add;
            extra -= add * slots.length;
          }
          left += extra / 2;
        }
        var colX = {}, x = left;
        cols.forEach(function (c, j) { colX[c] = x; x += colW[c] + (slots[j] || 0); });
        var zones = [];
        ZONE_ORDER.forEach(function (key) {
          var zc = cols.filter(function (c) { return zoneOfCol[c] === key; });
          if (!zc.length) return;
          var a = colX[zc[0]] - PAD, b = colX[zc[zc.length - 1]] + colW[zc[zc.length - 1]] + PAD;
          if (b - a < 240) { var grow = (240 - (b - a)) / 2; a -= grow; b += grow; }
          zones.push({ key: key, x0: a, x1: b });
        });
        var pos = {};
        ids.forEach(function (id) {
          var n = NODES[id], sz = size[id], px = colX[n.col] + (colW[n.col] - sz.w) / 2, cy;
          if (n.container) { pos[id] = { x: px, y: 0, w: sz.w, h: hc, cli: true }; return; }
          var ri = rows.indexOf(n.row);
          cy = rows.length === 1 ? hc / 2 : BOX_H / 2 + ri * (hc - BOX_H) / (rows.length - 1);
          pos[id] = { x: px, y: cy - sz.h / 2, w: sz.w, h: sz.h };
        });
        return { ids: ids, pos: pos, zones: zones, hc: hc, nChecks: nChecks };
      }
      // The two boxes joined by the step arrow, or null for a step that stays inside one box.
      function edgeEnds(st) {
        var a = resolve(st.from), b = resolve(st.to);
        return a === b ? null : [a, b];
      }
      var sceneCache = {};
      function sceneHeight(s) {
        if (!sceneCache[s.id]) {
          var hc = 0;
          s.steps.forEach(function (st) { hc = Math.max(hc, layout(s, st).hc); });
          sceneCache[s.id] = { hc: hc, h: CT + hc + 24 + ZT };
        }
        return sceneCache[s.id];
      }
      // Arrow points from box A to box B (absolute y values, oy is the content offset).
      function route(A, B, oy) {
        var acx = A.x + A.w / 2, bcx = B.x + B.w / 2, p, q, ay, by, xm;
        if (Math.abs(acx - bcx) < 1) {
          if (A.y < B.y) return [[acx, oy + A.y + A.h], [acx, oy + B.y]];
          return [[acx, oy + A.y], [acx, oy + B.y + B.h]];
        }
        var acy = oy + A.y + A.h / 2, bcy = oy + B.y + B.h / 2;
        ay = A.cli ? bcy : acy;
        by = B.cli ? acy : bcy;
        p = A.x < B.x ? A.x + A.w : A.x;
        q = A.x < B.x ? B.x : B.x + B.w;
        if (Math.abs(ay - by) < 1) return [[p, ay], [q, by]];
        xm = (p + q) / 2;
        return [[p, ay], [xm, ay], [xm, by], [q, by]];
      }
      function pathD(pts) {
        return pts.map(function (p, i) { return (i ? 'L' : 'M') + p[0] + ' ' + p[1]; }).join(' ');
      }
      function polyLen(pts) {
        var t = 0, i;
        for (i = 1; i < pts.length; i++) t += Math.hypot(pts[i][0] - pts[i - 1][0], pts[i][1] - pts[i - 1][1]);
        return t;
      }
      function pointAt(pts, dist) {
        var i, d;
        for (i = 1; i < pts.length; i++) {
          d = Math.hypot(pts[i][0] - pts[i - 1][0], pts[i][1] - pts[i - 1][1]);
          if (dist <= d || i === pts.length - 1) {
            var t = d ? Math.min(1, dist / d) : 1;
            return [pts[i - 1][0] + (pts[i][0] - pts[i - 1][0]) * t, pts[i - 1][1] + (pts[i][1] - pts[i - 1][1]) * t];
          }
          dist -= d;
        }
        return pts[0];
      }
      // Dot animation along a polyline, with ease-in-out and a delay so it starts after the arrow is drawn.
      var raf = 0;
      function stopPacket() {
        if (raf) cancelAnimationFrame(raf);
        raf = 0;
        el['g-packet'].textContent = '';
      }
      function runPacket(pts, bad) {
        stopPacket();
        if (reduced || !pts) return;
        var total = polyLen(pts), sp = speed(), delay = DRAW_MS / sp, dur = DOT_MS / sp, start = null;
        var dot = mk('circle', { 'class': 'packet' + (bad ? ' bad' : ''), r: 7, cx: pts[0][0], cy: pts[0][1], opacity: 0 }, el['g-packet']);
        function frame(ts) {
          if (start === null) start = ts;
          var t = Math.max(0, Math.min(1, (ts - start - delay) / dur));
          var e = t < 0.5 ? 2 * t * t : 1 - Math.pow(-2 * t + 2, 2) / 2;
          var p = pointAt(pts, e * total);
          dot.setAttribute('cx', p[0]);
          dot.setAttribute('cy', p[1]);
          if (ts - start >= delay) dot.setAttribute('opacity', 1);
          raf = t < 1 ? requestAnimationFrame(frame) : 0;
        }
        raf = requestAnimationFrame(frame);
      }
      // ---------------------------------------------------------------------------
      // Drawing one step
      // ---------------------------------------------------------------------------
      function checkState(s, c, i) {
        var cur = s.steps[i], k;
        if (cur.actors.indexOf(c) !== -1) return isFailing(cur) ? 'fail' : 'current';
        for (k = 0; k < i; k++) if (s.steps[k].actors.indexOf(c) !== -1) return 'done';
        return 'todo';
      }
      function drawNode(s, id, p, oy, ctx) {
        var n = NODES[id], x = p.x, y = oy + p.y, cls = 'node ' + n.trust + (n.container ? ' container' : '');
        if (ctx.fail[id]) cls += ' fail';
        else if (ctx.active[id]) cls += ' active';
        if (ctx.target === id) cls += ' target';
        var g = mk('g', { 'class': cls, 'data-id': id }, el['g-nodes']);
        mk('title', null, g, n.name + (n.sub ? ': ' + n.sub : ''));
        mk('rect', { 'class': 'halo', x: x - 5, y: y - 5, width: p.w + 10, height: p.h + 10, rx: 8 }, g);
        mk('rect', { 'class': 'box', x: x, y: y, width: p.w, height: p.h, rx: 8 }, g);
        mk('text', { 'class': 'name', x: x + 14, y: y + 30 }, g, n.name);
        if (n.sub) mk('text', { 'class': 'sub', x: x + 14, y: y + 50 }, g, n.sub);
        if (n.container) {
          checksOf(s).forEach(function (c, i) {
            var state = checkState(s, c, ctx.step), ry = y + 44 + i * CHECK_H;
            var row = mk('g', { 'class': 'check ' + state }, g);
            if (state === 'current' || state === 'fail') mk('rect', { 'class': 'row', x: x + 8, y: ry, width: p.w - 16, height: CHECK_H - 2, rx: 8 }, row);
            mk('text', { 'class': 'mark', x: x + 24, y: ry + 17, 'text-anchor': 'middle' }, row, { done: '✓', current: '●', fail: '✕', todo: '○' }[state]);
            mk('text', { 'class': 'ctext', x: x + 40, y: ry + 17 }, row, NODES[c].name);
          });
        }
        var label = BADGE[n.trust], bw = Math.round(tw(label, 14) + 16);
        var b = mk('g', { 'class': 'badge' }, g);
        mk('rect', { x: x + p.w - bw - 10, y: y - 11, width: bw, height: 22, rx: 8 }, b);
        mk('text', { x: x + p.w - 10 - bw / 2, y: y + 5, 'text-anchor': 'middle' }, b, label);
        var cross = mk('g', { 'class': 'badge-x' }, g);
        mk('circle', { cx: x, cy: y, r: 11 }, cross);
        mk('text', { x: x, y: y + 5, 'text-anchor': 'middle' }, cross, '✕');
      }
      function drawArrow(pts, bad) {
        var n = pts.length, tip = pts[n - 1], prev = pts[n - 2], dx = tip[0] - prev[0], dy = tip[1] - prev[1];
        var len = Math.hypot(dx, dy) || 1, ux = dx / len, uy = dy / len, sp = speed();
        var line = pts.slice(0, n - 1).concat([[tip[0] - ux * 10, tip[1] - uy * 10]]);
        var cls = 'edge-active' + (bad ? ' bad' : '') + (reduced ? '' : ' draw');
        var path = mk('path', { 'class': cls, d: pathD(line) }, el['g-edges']);
        if (!reduced) {
          path.style.setProperty('--len', String(Math.ceil(polyLen(line)) + 2));
          path.style.animationDuration = (DRAW_MS / sp) + 'ms';
        }
        var head = mk('path', {
          'class': 'arrow-head' + (bad ? ' bad' : '') + (reduced ? '' : ' anim'),
          d: 'M' + tip[0] + ' ' + tip[1] + ' L' + (tip[0] - ux * 13 - uy * 6.5) + ' ' + (tip[1] - uy * 13 + ux * 6.5) +
            ' L' + (tip[0] - ux * 13 + uy * 6.5) + ' ' + (tip[1] - uy * 13 - ux * 6.5) + ' Z'
        }, el['g-edges']);
        if (!reduced) head.style.animationDelay = ((DRAW_MS * 0.75) / sp) + 'ms';
      }
      function drawLabel(pts, label, vertical) {
        var box = labelBox(label), total = polyLen(pts), mid = pointAt(pts, total / 2);
        var cx = vertical ? mid[0] + 12 + box.w / 2 : mid[0], cy = mid[1];
        var g = mk('g', { 'class': 'wire' }, el['g-wire']);
        mk('rect', { x: cx - box.w / 2, y: cy - box.h / 2, width: box.w, height: box.h, rx: 8 }, g);
        box.lines.forEach(function (l, i) {
          mk('text', { x: cx, y: cy - box.h / 2 + 5 + 14 + i * 18 - 2, 'text-anchor': 'middle' }, g, l);
        });
        return { x: cx - box.w / 2, y: cy - box.h / 2, w: box.w, h: box.h };
      }
      function drawScene(s, st, i) {
        var L = layout(s, st), sc = sceneHeight(s), oy = CT + (sc.hc - L.hc) / 2, failing = isFailing(st);
        ['g-zones', 'g-edges', 'g-nodes', 'g-wire'].forEach(function (k) { el[k].textContent = ''; });
        stopPacket();
        L.zones.forEach(function (z) {
          var g = mk('g', { 'class': 'zone' }, el['g-zones']);
          mk('rect', { x: z.x0, y: ZT, width: z.x1 - z.x0, height: sc.h - 2 * ZT, rx: 8 }, g);
          mk('text', { x: z.x0 + 14, y: ZT + 26 }, g, ZONE_LABEL[z.key]);
        });
        var ends = edgeEnds(st), ctx = { active: {}, fail: {}, target: ends ? ends[1] : resolve(st.actors[0]), step: i };
        st.actors.forEach(function (a) { (failing ? ctx.fail : ctx.active)[resolve(a)] = true; });
        EDGES.forEach(function (e) {
          if (!L.pos[e[0]] || !L.pos[e[1]]) return;
          if (ends && ((ends[0] === e[0] && ends[1] === e[1]) || (ends[0] === e[1] && ends[1] === e[0]))) return;
          mk('path', { 'class': 'edge', d: pathD(route(L.pos[e[0]], L.pos[e[1]], oy)) }, el['g-edges']);
        });
        L.ids.forEach(function (id) { drawNode(s, id, L.pos[id], oy, ctx); });
        var pts = null;
        if (ends) {
          pts = route(L.pos[ends[0]], L.pos[ends[1]], oy);
          drawArrow(pts, failing);
          if (st.label) drawLabel(pts, st.label, pts.length === 2 && pts[0][0] === pts[1][0]);
        }
        runPacket(pts, failing);
        svg.setAttribute('aria-label', 'Step ' + (i + 1) + ': ' + L.ids.map(function (id) { return NODES[id].name; }).join(', ') +
          '. The words for this step are in the text below the diagram.');
      }
      // ---------------------------------------------------------------------------
      // State and updates
      // ---------------------------------------------------------------------------
      var sIdx = 0, step = 0, playing = false, timer = 0;
      function speed() { return parseFloat(el.speed.value) || 1; }
      function scn() { return SCENARIOS[sIdx]; }
      function firstFail(s) {
        for (var i = 0; i < s.steps.length; i++) if (s.steps[i].status === 'fail') return i;
        return -1;
      }
      // A scenario with neverReached stops at its first failing step.
      function stopsAtFail(s) { return !!s.neverReached; }
      function render() {
        var s = scn(), st = s.steps[step], last = s.steps.length - 1;
        var fi = stopsAtFail(s) ? firstFail(s) : -1;
        var atFail = fi >= 0 && step >= fi;
        drawScene(s, st, step);
        // Caption
        el['p-count'].textContent = 'Step ' + (step + 1) + ' of ' + s.steps.length;
        var stKey = st.status || 'ok';
        el['p-status'].className = 'cap-status ' + stKey;
        el['p-status'].textContent = STATUS_TEXT[stKey];
        el['p-title'].textContent = st.title;
        el['p-text'].textContent = '';
        richText(el['p-text'], st.text);
        el['p-protected'].textContent = '';
        if (atFail && s.protectedNote) {
          var pr = h('div', 'protected');
          pr.appendChild(h('strong', '', '✕ What was protected. '));
          pr.appendChild(document.createTextNode(s.protectedNote.replace(/^Protected: /, '')));
          if (s.neverReached) pr.appendChild(h('div', 'never', 'Never reached: ' + s.neverReached.join(', ') + '.'));
          el['p-protected'].appendChild(pr);
        }
        el['p-data'].textContent = '';
        (st.data || []).forEach(function (box) {
          var d = h('div', 'databox');
          d.appendChild(h('div', 'dlabel', box.label));
          var body = h('div', 'dlines');
          box.lines.forEach(function (ln) {
            var o = typeof ln === 'string' ? { k: '', t: ln } : ln;
            var row = h('div', 'l ' + o.k);
            var mark = { ok: '✓', no: '✗', del: '−', add: '+' }[o.k];
            if (mark) row.appendChild(h('span', 'm', mark));
            row.appendChild(document.createTextNode(o.t));
            body.appendChild(row);
          });
          d.appendChild(body);
          el['p-data'].appendChild(d);
        });
        // Controls
        el.prev.disabled = step === 0;
        el.next.disabled = step === last;
        updateDots(s);
      }
      function buildDots(s) {
        el.dots.textContent = '';
        s.steps.forEach(function (st, i) {
          var b = h('button', 'dot');
          b.type = 'button';
          b.setAttribute('aria-label', 'Step ' + (i + 1) + ': ' + st.title);
          b.title = 'Step ' + (i + 1) + ': ' + st.title;
          b.addEventListener('click', function () { setPlaying(false); go(i); });
          el.dots.appendChild(b);
        });
      }
      function updateDots(s) {
        var dots = el.dots.childNodes;
        s.steps.forEach(function (st, i) {
          var b = dots[i], failed = isFailing(st) && i <= step;
          b.className = 'dot ' + (i === step ? 'current' : (i < step ? 'done' : 'upcoming')) + (failed ? ' failed' : '');
          if (i === step) b.setAttribute('aria-current', 'step');
          else b.removeAttribute('aria-current');
        });
      }
      function go(i) {
        step = Math.max(0, Math.min(scn().steps.length - 1, i));
        render();
        schedule();
      }
      function schedule() {
        clearTimeout(timer);
        if (!playing) return;
        timer = setTimeout(function () {
          if (step < scn().steps.length - 1) go(step + 1);
          else setPlaying(false);
        }, STEP_MS / speed());
      }
      function setPlaying(on) {
        playing = on;
        el.play.textContent = on ? '❚❚ Pause' : '▶ Play';
        if (!on) clearTimeout(timer);
      }
      function togglePlay() {
        if (playing) { setPlaying(false); return; }
        setPlaying(true);
        if (step >= scn().steps.length - 1) go(0); else schedule();
      }
      function reset() { setPlaying(false); go(0); }
      function selectScenario(i) {
        sIdx = i;
        setPlaying(false);
        var s = scn();
        el.summary.textContent = s.summary;
        svg.setAttribute('viewBox', '0 0 ' + W + ' ' + sceneHeight(s).h);
        buildDots(s);
        step = 0;
        render();
      }
      function buildSelect() {
        var groups = {};
        SCENARIOS.forEach(function (s, i) {
          if (!groups[s.group]) {
            groups[s.group] = document.createElement('optgroup');
            groups[s.group].label = s.group;
            el.scenario.appendChild(groups[s.group]);
          }
          var o = document.createElement('option');
          o.value = String(i);
          o.textContent = s.name + ' (' + s.steps.length + ' steps)';
          groups[s.group].appendChild(o);
        });
      }
      function onMotionChange() {
        reduced = !!(mq && mq.matches);
        root.classList.toggle('still', reduced);
        render();
      }
      // ---------------------------------------------------------------------------
      // Wiring
      // ---------------------------------------------------------------------------
      buildSelect();
      root.classList.toggle('still', reduced);
      el.scenario.addEventListener('change', function () { selectScenario(parseInt(el.scenario.value, 10)); });
      el.speed.addEventListener('change', function () { schedule(); });
      el.prev.addEventListener('click', function () { setPlaying(false); go(step - 1); });
      el.next.addEventListener('click', function () { setPlaying(false); go(step + 1); });
      el.play.addEventListener('click', togglePlay);
      if (mq) {
        if (mq.addEventListener) mq.addEventListener('change', onMotionChange);
        else if (mq.addListener) mq.addListener(onMotionChange);
      }
      document.addEventListener('keydown', function (ev) {
        if (!root.contains(document.activeElement)) return;
        if (ev.ctrlKey || ev.metaKey || ev.altKey) return;
        var tag = ev.target && ev.target.tagName;
        if (tag === 'SELECT' || tag === 'TEXTAREA' || tag === 'INPUT') return;
        if (ev.key === 'ArrowRight') { ev.preventDefault(); setPlaying(false); go(step + 1); }
        else if (ev.key === 'ArrowLeft') { ev.preventDefault(); setPlaying(false); go(step - 1); }
        else if (ev.key === 'r' || ev.key === 'R') { ev.preventDefault(); reset(); }
        else if ((ev.key === ' ' || ev.code === 'Space') && tag !== 'BUTTON') { ev.preventDefault(); togglePlay(); }
      });
      setPlaying(false);
      selectScenario(0);
    })();
