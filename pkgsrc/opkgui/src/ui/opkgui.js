/*
 * oPKGui DSM desktop window.
 *
 * Real ExtJS 3.x widgets (SYNOCOMMUNITY.OPKGui.* namespace, extending
 * SYNO.SDS.AppInstance/AppWindow -- the documented third-party pattern,
 * see DESIGN.md's Round 9), calling oPKGui's own existing Go HTTP service.
 *
 * NOT an iframe: DSM's own page sends a Content-Security-Policy with
 * frame-src restricted to 'self' (confirmed by reading the actual
 * response header), which blocks framing anything off-origin no matter
 * what TLS/cert setup the target has. connect-src is unrestricted
 * though, so plain XHR to another port works fine.
 *
 * NOT Ext.Ajax/Ext.data's built-in transport, either: that's a shared,
 * page-wide singleton DSM's own code configures (default headers,
 * possibly a CSRF token, etc.) -- since it's shared, DSM's own config
 * would silently apply to our cross-origin calls too and could trip a
 * CORS preflight our simple server-side config doesn't cover. Using a
 * raw, isolated XMLHttpRequest here avoids depending on whatever DSM's
 * global AJAX defaults happen to be.
 */
Ext.ns("SYNOCOMMUNITY.OPKGui");

(function() {
    var isHttps = window.location.protocol === "https:";
    var BASE = (isHttps ? "https:" : "http:") + "//" + window.location.hostname + ":" + (isHttps ? 18891 : 18890);

    var allowSystemDup = false;
    var grids = [];

    function httpRequest(method, url, cb) {
        var xhr = new XMLHttpRequest();
        xhr.open(method, url, true);
        if (method === "POST") {
            xhr.setRequestHeader("X-Requested-With", "XMLHttpRequest");
        }
        xhr.onreadystatechange = function() {
            if (xhr.readyState !== 4) return;
            var body = null;
            try { body = JSON.parse(xhr.responseText); } catch (e) {}
            cb(xhr.status >= 200 && xhr.status < 300, body, xhr.status);
        };
        xhr.onerror = function() { cb(false, null, 0); };
        xhr.send();
    }

    function reloadAll() {
        Ext.each(grids, function(g) { g.opkguiLoad(); });
    }

    // The one-time permission grant (opkg.conf/status access + the narrow
    // sudoers rule) still needs a DSM Task Scheduler entry -- oPKGui can
    // never get root itself (DESIGN.md Round 5). This creates that task
    // via DSM's own SYNO.Core.EventScheduler webapi instead of asking the
    // user to paste the script in by hand. Deliberately does NOT also call
    // "run": the user still clicks Run themselves in Task Scheduler, a
    // visible consent step before anything executes as root (see
    // DESIGN.md's Round 10 -- this was a scope choice, not a limitation).
    //
    // Uses Ext.Ajax (not the raw XHR helper above) on purpose: this call
    // goes to DSM's own same-origin /webapi/entry.cgi, and DSM's own code
    // globally hooks Ext.Ajax to attach the X-SYNO-TOKEN CSRF header --
    // exactly what a mutating webapi call here needs, and the reverse of
    // why Ext.Ajax was avoided for oPKGui's own cross-origin API above.
    var GRANT_SCRIPT = [
        "#!/bin/sh",
        "# oPKGui: grant its unprivileged service account (sc-opkgui) exactly the",
        "# elevated access it needs, since DSM 7 won't let the .spk itself request",
        "# any root privilege.",
        "",
        "for f in /opt/etc/opkg.conf /opt/lib/opkg/status; do",
        "    if [ -e \"${f}\" ]; then",
        "        chown root:opkgui \"${f}\"",
        "        chmod 640 \"${f}\"",
        "    fi",
        "done",
        "",
        "cat > /etc/sudoers.d/opkgui <<'EOF'",
        "sc-opkgui ALL=(root) NOPASSWD: /opt/bin/opkg install *",
        "sc-opkgui ALL=(root) NOPASSWD: /opt/bin/opkg remove *",
        "sc-opkgui ALL=(root) NOPASSWD: /opt/bin/opkg upgrade *",
        "Defaults!/opt/bin/opkg !requiretty",
        "EOF",
        "chown root:root /etc/sudoers.d/opkgui",
        "chmod 440 /etc/sudoers.d/opkgui"
    ].join("\n");

    function createPermissionGrantTask() {
        if (!confirm("Create a DSM Task Scheduler task (\"oPKGui permissions\", User: root, " +
            "Event: Boot-up) that grants oPKGui's service account read access to opkg's " +
            "config/status and a narrow sudo rule for opkg install/remove/upgrade?\n\n" +
            "This only CREATES the task -- it does not run it. You'll still need to open " +
            "Control Panel → Task Scheduler and click Run yourself.")) {
            return;
        }
        Ext.Ajax.request({
            url: "/webapi/entry.cgi",
            method: "POST",
            params: {
                api: "SYNO.Core.EventScheduler",
                method: "create",
                version: 1,
                task_name: "oPKGui permissions",
                owner: 0,
                event: "bootup",
                enable: true,
                notify_enable: false,
                notify_if_error: false,
                notify_mail: "",
                script: GRANT_SCRIPT
            },
            success: function(r) {
                var body = {};
                try { body = Ext.decode(r.responseText); } catch (e) {}
                if (body.success) {
                    alert("Created. Open Control Panel → Task Scheduler, select " +
                        "\"oPKGui permissions\", and click Run.");
                } else {
                    alert("Failed to create the task (error code " +
                        (body.error && body.error.code) + "). You can still set it up " +
                        "manually -- see TESTING.md.");
                }
            },
            failure: function(r) {
                alert("Request failed (" + r.status + "). You can still set it up " +
                    "manually -- see TESTING.md.");
            }
        });
    }

    function actionCall(verb, name, cb) {
        httpRequest("POST", BASE + "/api/action?action=" + verb + "&pkg=" + encodeURIComponent(name), cb);
    }

    function makeGrid(cfg) {
        var store = new Ext.data.JsonStore({
            root: "packages",
            idProperty: "name",
            fields: ["name", "version", "new_version", "desc", "system_dup"]
        });

        function load() {
            httpRequest("GET", BASE + "/api?action=" + cfg.action, function(ok, body) {
                if (!ok || !body) {
                    alert("Failed to load " + cfg.title + (body && body.error ? ": " + body.error : " (request failed)"));
                    return;
                }
                store.loadData(body);
            });
        }

        var columns = [{ header: "Package", dataIndex: "name", width: 220 }];

        if (cfg.action === "upgradable") {
            columns.push({ header: "Installed", dataIndex: "version", width: 100 });
            columns.push({ header: "Available", dataIndex: "new_version", width: 100 });
        } else if (cfg.action === "available") {
            columns.push({ header: "Version", dataIndex: "version", width: 100 });
            columns.push({ header: "Description", dataIndex: "desc", id: "opkgui-desc-col" });
        } else {
            columns.push({ header: "Version", dataIndex: "version", width: 100 });
        }

        var actionColIndex = columns.length;
        columns.push({
            header: "",
            dataIndex: "name",
            width: 70,
            renderer: function(value, meta, record) {
                if (cfg.action === "available" && record.get("system_dup") && !allowSystemDup) {
                    return '<span style="color:#999;">system</span>';
                }
                return '<a href="#" class="opkgui-action-link">' + cfg.buttonText + "</a>";
            }
        });

        var gridConfig = {
            title: cfg.title,
            store: store,
            columns: columns,
            autoExpandColumn: cfg.action === "available" ? "opkgui-desc-col" : undefined,
            sm: new Ext.grid.RowSelectionModel({ singleSelect: true }),
            listeners: {
                cellclick: function(grid, rowIndex, columnIndex) {
                    if (columnIndex !== actionColIndex) return;
                    var rec = store.getAt(rowIndex);
                    if (cfg.action === "available" && rec.get("system_dup") && !allowSystemDup) return;
                    var name = rec.get("name");
                    if (!confirm(cfg.buttonText + ' "' + name + '"?')) return;
                    actionCall(cfg.actionVerb, name, function(ok, body) {
                        if (!ok) {
                            alert((body && body.error || "request failed") + (body && body.output ? "\n" + body.output.join("\n") : ""));
                            return;
                        }
                        reloadAll();
                    });
                }
            }
        };

        if (cfg.action === "available") {
            gridConfig.tbar = [
                {
                    xtype: "checkbox",
                    boxLabel: "Allow installing packages that duplicate a system command",
                    handler: function(cb, checked) {
                        allowSystemDup = checked;
                        Ext.each(grids, function(g) {
                            if (g.opkguiAction === "available") g.getView().refresh();
                        });
                    }
                },
                "->",
                {
                    xtype: "textfield",
                    emptyText: "Filter by name...",
                    width: 180,
                    enableKeyEvents: true,
                    listeners: {
                        keyup: function(f) {
                            var v = f.getValue().toLowerCase();
                            store.filterBy(function(rec) {
                                return !v || rec.get("name").toLowerCase().indexOf(v) !== -1;
                            });
                        }
                    }
                }
            ];
        }

        var grid = new Ext.grid.GridPanel(gridConfig);
        grid.opkguiLoad = load;
        grid.opkguiAction = cfg.action;
        load();
        return grid;
    }

    Ext.define("SYNOCOMMUNITY.OPKGui.AppInstance", {
        extend: "SYNO.SDS.AppInstance",
        appWindowName: "SYNOCOMMUNITY.OPKGui.AppWindow",
        constructor: function() {
            this.callParent(arguments);
        }
    });

    Ext.define("SYNOCOMMUNITY.OPKGui.AppWindow", {
        extend: "SYNO.SDS.AppWindow",
        constructor: function(config) {
            var installedGrid = makeGrid({
                title: "Installed", action: "installed",
                buttonText: "Remove", actionVerb: "remove"
            });
            var upgradableGrid = makeGrid({
                title: "Updates available", action: "upgradable",
                buttonText: "Upgrade", actionVerb: "upgrade"
            });
            var availableGrid = makeGrid({
                title: "Available", action: "available",
                buttonText: "Install", actionVerb: "install"
            });
            grids = [installedGrid, upgradableGrid, availableGrid];

            config = Ext.apply({
                resizable: true,
                maximizable: true,
                minimizable: true,
                width: 820,
                height: 560,
                layout: "fit",
                tbar: [{
                    text: "Set Up Permission Grant Task",
                    handler: createPermissionGrantTask
                }],
                items: [{
                    xtype: "tabpanel",
                    activeTab: 0,
                    items: grids
                }]
            }, config);
            this.callParent([config]);
        }
    });
})();
