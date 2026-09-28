/*
 * oPKGui DSM desktop window.
 *
 * Real ExtJS 3.x widgets (SYNOCOMMUNITY.OPKGui.* namespace, extending
 * SYNO.SDS.AppInstance/AppWindow -- the documented third-party pattern,
 * see DESIGN.md's Round 9), calling oPKGui's own existing Go HTTP service
 * via Ext.Ajax/Ext.data.JsonStore.
 *
 * NOT an iframe: DSM's own page sends a Content-Security-Policy with
 * frame-src restricted to 'self' (confirmed by reading the actual
 * response header), which blocks framing anything off-origin no matter
 * what TLS/cert setup the target has. connect-src is unrestricted
 * though, so plain XHR to another port works fine -- hence real grid
 * widgets instead of wrapping the existing HTML page in an iframe.
 */
Ext.ns("SYNOCOMMUNITY.OPKGui");

(function() {
    var isHttps = window.location.protocol === "https:";
    var BASE = (isHttps ? "https:" : "http:") + "//" + window.location.hostname + ":" + (isHttps ? 18891 : 18890);

    var allowSystemDup = false;
    var grids = [];

    function reloadAll() {
        Ext.each(grids, function(g) { g.opkguiStore.reload(); });
    }

    function actionCall(verb, name, cb) {
        Ext.Ajax.request({
            url: BASE + "/api/action?action=" + verb + "&pkg=" + encodeURIComponent(name),
            method: "POST",
            headers: { "X-Requested-With": "XMLHttpRequest" },
            success: function(resp) {
                var body = {};
                try { body = Ext.decode(resp.responseText); } catch (e) {}
                cb(true, body);
            },
            failure: function(resp) {
                var body = {};
                try { body = Ext.decode(resp.responseText); } catch (e) {}
                cb(false, body);
            }
        });
    }

    function makeGrid(cfg) {
        var store = new Ext.data.JsonStore({
            url: BASE + "/api?action=" + cfg.action,
            root: "packages",
            idProperty: "name",
            fields: ["name", "version", "new_version", "desc", "system_dup"],
            autoLoad: true
        });

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
                            alert((body.error || "request failed") + (body.output ? "\n" + body.output.join("\n") : ""));
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
        grid.opkguiStore = store;
        grid.opkguiAction = cfg.action;
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
