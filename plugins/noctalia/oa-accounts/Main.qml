import QtQuick
import Quickshell.Io

Item {
    id: root

    property var pluginApi: null
    property var service

    function startServiceIfReady() {
        if (pluginApi && service && service.pluginApi !== pluginApi)
            service.start(pluginApi);

    }

    onPluginApiChanged: startServiceIfReady()
    Component.onCompleted: startServiceIfReady()

    IpcHandler {
        function togglePanel() {
            if (!pluginApi || !pluginApi.withCurrentScreen)
                return ;

            pluginApi.withCurrentScreen((screen) => {
                return pluginApi.togglePanel(screen);
            });
        }

        function refresh() {
            service.refresh(true);
        }

        function sync(targetId: string) {
            service.syncTarget(targetId);
        }

        target: "plugin:oa-accounts"
    }

    service: OAAccountsService {
        id: serviceInstance
    }

}
