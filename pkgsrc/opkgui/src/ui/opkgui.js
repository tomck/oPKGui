/*
 * oPKGui DSM desktop window.
 *
 * Modeled on SynoCommunity's own ExtJS getting-started guide (namespace +
 * base classes are the documented convention, not a guess -- see
 * DESIGN.md's Round 9). Wraps the existing, already-working HTML/JS
 * frontend in an iframe inside a real SYNO.SDS.AppWindow, rather than
 * reimplementing all its tab/grid/action logic as native ExtJS widgets --
 * this reuses the tested frontend as-is and only adds the window chrome.
 */
Ext.ns("SYNOCOMMUNITY.OPKGui");

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
        // window.location.hostname (not a hardcoded IP) so this works
        // regardless of how the admin is currently reaching DSM. DSM
        // forces HTTPS for its own UI, and a plain-http iframe inside an
        // https page gets mixed-content blocked -- so match protocol,
        // using oPKGui's HTTPS listener (port+1) when DSM is on HTTPS.
        var isHttps = window.location.protocol === "https:";
        var port = isHttps ? 18891 : 18890;
        var url = (isHttps ? "https:" : "http:") + "//" + window.location.hostname + ":" + port + "/";
        config = Ext.apply({
            resizable: true,
            maximizable: true,
            minimizable: true,
            width: 960,
            height: 640,
            layout: "fit",
            items: [{
                xtype: "panel",
                layout: "fit",
                html: '<iframe src="' + url + '" style="width:100%;height:100%;border:0;background:#fff;"></iframe>'
            }]
        }, config);
        this.callParent([config]);
    }
});
