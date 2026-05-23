import QtQuick
import Quickshell.Io

Item {
    id: root

    property var pluginApi: null
    property var snapshot: ({
    })
    property var recommendation: ({
        "available": false,
        "message": "No data yet"
    })
    property var accounts: []
    property var syncTargets: []
    property var warnings: []
    property bool loading: false
    property bool actionRunning: false
    property string errorText: ""
    property string actionText: ""
    property string lastUpdated: ""
    property string pendingActionLabel: ""
    readonly property bool hasRecommendation: recommendation && recommendation.available === true
    readonly property string recommendationLabel: hasRecommendation ? (recommendation.account_name || recommendation.account_id) : ((recommendation && recommendation.message) || "No recommended account")
    readonly property string binaryPath: {
        if (pluginApi && pluginApi.pluginSettings && pluginApi.pluginSettings.binaryPath)
            return pluginApi.pluginSettings.binaryPath;

        if (pluginApi && pluginApi.manifest && pluginApi.manifest.metadata && pluginApi.manifest.metadata.defaultSettings && pluginApi.manifest.metadata.defaultSettings.binaryPath)
            return pluginApi.manifest.metadata.defaultSettings.binaryPath;

        return "oa";
    }
    readonly property int refreshIntervalSeconds: {
        if (pluginApi && pluginApi.pluginSettings && pluginApi.pluginSettings.refreshIntervalSeconds)
            return pluginApi.pluginSettings.refreshIntervalSeconds;

        if (pluginApi && pluginApi.manifest && pluginApi.manifest.metadata && pluginApi.manifest.metadata.defaultSettings && pluginApi.manifest.metadata.defaultSettings.refreshIntervalSeconds)
            return pluginApi.manifest.metadata.defaultSettings.refreshIntervalSeconds;

        return 300;
    }
    readonly property bool refreshOnLoad: {
        if (pluginApi && pluginApi.pluginSettings && pluginApi.pluginSettings.refreshOnLoad !== undefined)
            return pluginApi.pluginSettings.refreshOnLoad;

        if (pluginApi && pluginApi.manifest && pluginApi.manifest.metadata && pluginApi.manifest.metadata.defaultSettings && pluginApi.manifest.metadata.defaultSettings.refreshOnLoad !== undefined)
            return pluginApi.manifest.metadata.defaultSettings.refreshOnLoad;

        return true;
    }

    function start(api) {
        pluginApi = api;
        refresh(refreshOnLoad);
        refreshTimer.running = true;
    }

    function defaultSyncTargets() {
        return [{
            "id": "opencode",
            "label": "OpenCode",
            "command": ["sync", "opencode", "--json"]
        }, {
            "id": "codex",
            "label": "Codex",
            "command": ["sync", "codex", "--json"]
        }, {
            "id": "pi",
            "label": "Pi",
            "command": ["sync", "pi", "--json"]
        }, {
            "id": "all",
            "label": "All",
            "command": ["sync", "--all", "--json"]
        }];
    }

    function snapshotCommand(forceRefresh) {
        const args = [binaryPath, "noctalia", "snapshot", "--json"];
        if (forceRefresh)
            args.push("--refresh");

        return args;
    }

    function parseJSON(text, fallbackValue) {
        try {
            if (!text || text.trim() === "")
                return fallbackValue;

            return JSON.parse(text);
        } catch (error) {
            return fallbackValue;
        }
    }

    function applySnapshot(parsed) {
        snapshot = parsed || ({
        });
        recommendation = parsed && parsed.recommendation ? parsed.recommendation : ({
            "available": false,
            "message": "No recommendation"
        });
        accounts = parsed && parsed.accounts ? parsed.accounts : [];
        syncTargets = parsed && parsed.sync_targets ? parsed.sync_targets : defaultSyncTargets();
        warnings = parsed && parsed.warnings ? parsed.warnings : [];
        lastUpdated = parsed && parsed.generated_at ? parsed.generated_at : "";
        errorText = "";
    }

    function refresh(forceRefresh) {
        if (snapshotProcess.running)
            return ;

        loading = true;
        errorText = "";
        snapshotProcess.command = snapshotCommand(forceRefresh);
        snapshotProcess.running = true;
    }

    function syncTarget(targetId) {
        if (actionProcess.running)
            return ;

        const target = resolveSyncTarget(targetId);
        if (!target) {
            errorText = `Unknown sync target: ${targetId}`;
            return ;
        }
        pendingActionLabel = target.label || target.id || targetId;
        errorText = "";
        actionText = "";
        actionRunning = true;
        actionProcess.command = [binaryPath].concat(target.command || []);
        actionProcess.running = true;
    }

    function resolveSyncTarget(targetId) {
        const targets = syncTargets && syncTargets.length ? syncTargets : defaultSyncTargets();
        for (const target of targets) {
            if (target.id === targetId)
                return target;

        }
        return null;
    }

    function processErrorText(rawText, exitCode) {
        const parsed = parseJSON(rawText, null);
        if (parsed && parsed.error)
            return parsed.error;

        if (rawText && rawText.trim() !== "")
            return rawText.trim();

        return `oa command failed (exit ${exitCode})`;
    }

    visible: false

    Process {
        id: snapshotProcess

        running: false
        onExited: function(exitCode) {
            loading = false;
            if (exitCode === 0) {
                const parsed = root.parseJSON(snapshotStdout.text, null);
                if (parsed === null) {
                    errorText = "Failed to parse oa snapshot JSON";
                    return ;
                }
                applySnapshot(parsed);
                return ;
            }
            errorText = processErrorText(snapshotStderr.text, exitCode);
        }

        stdout: StdioCollector {
            id: snapshotStdout
        }

        stderr: StdioCollector {
            id: snapshotStderr
        }

    }

    Process {
        id: actionProcess

        running: false
        onExited: function(exitCode) {
            actionRunning = false;
            if (exitCode === 0) {
                const parsed = root.parseJSON(actionStdout.text, {
                });
                if (parsed.ok === false) {
                    errorText = parsed.error || `Failed to sync ${pendingActionLabel}`;
                    return ;
                }
                const accountName = parsed.account_name || parsed.account_id || "selected account";
                actionText = `Synced ${pendingActionLabel} using ${accountName}`;
                if (parsed.warnings)
                    warnings = parsed.warnings;

                refresh(false);
                return ;
            }
            errorText = processErrorText(actionStderr.text, exitCode);
        }

        stdout: StdioCollector {
            id: actionStdout
        }

        stderr: StdioCollector {
            id: actionStderr
        }

    }

    Timer {
        id: refreshTimer

        interval: refreshIntervalSeconds * 1000
        running: false
        repeat: true
        onTriggered: root.refresh(true)
    }

}
