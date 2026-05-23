import QtQuick
import Quickshell.Io

Item {
    id: root

    property var pluginApi: null
    property var service

    Component.onCompleted: {
        if (pluginApi)
            service.start(pluginApi);

    }

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
