import QtQuick
import Quickshell
import qs.Commons

Item {
    id: root

    property var pluginApi: null
    property ShellScreen screen
    property string widgetId: ""
    property string section: ""
    property int sectionWidgetIndex: -1
    property int sectionWidgetsCount: 0

    implicitWidth: Style.capsuleHeight
    implicitHeight: Style.capsuleHeight

    Rectangle {
        anchors.fill: parent
        radius: Style.radiusL
        color: mouseArea.containsMouse ? Color.mHover : Style.capsuleColor

        Image {
            anchors.centerIn: parent
            source: Qt.resolvedUrl("icons/bot.svg")
            sourceSize.width: 18
            sourceSize.height: 18
            width: 18
            height: 18
            smooth: true
        }

    }

    MouseArea {
        id: mouseArea

        anchors.fill: parent
        hoverEnabled: true
        cursorShape: Qt.PointingHandCursor
        onClicked: {
            if (pluginApi && pluginApi.openPanel)
                pluginApi.openPanel(root.screen, root);

        }
    }

}
