import QtQuick
import QtQuick.Layouts
import qs.Commons
import qs.Widgets

Item {
    id: root

    property string label: ""
    property var limitData: null
    property color accentColor: Color.mPrimary

    function remainingPercent() {
        if (!limitData || limitData.percent_remaining === undefined)
            return 0;

        return Math.max(0, Math.min(100, Math.round(limitData.percent_remaining)));
    }

    function progressRatio() {
        return remainingPercent() / 100;
    }

    function summaryText() {
        if (!limitData)
            return "Unavailable";

        return `${remainingPercent()}% remaining`;
    }

    function resetText() {
        if (!limitData || !limitData.resets_at)
            return "reset unknown";

        const at = new Date(limitData.resets_at);
        if (isNaN(at.getTime()))
            return "reset unknown";

        return at.toLocaleString(undefined, {
            "weekday": "short",
            "hour": "2-digit",
            "minute": "2-digit"
        });
    }

    implicitHeight: content.implicitHeight

    ColumnLayout {
        id: content

        anchors.fill: parent
        spacing: Style.marginXS

        RowLayout {
            Layout.fillWidth: true

            NText {
                text: root.label
                pointSize: Style.fontSizeS
                font.weight: Font.Medium
                color: Color.mOnSurface
            }

            Item {
                Layout.fillWidth: true
            }

            NText {
                text: root.summaryText()
                pointSize: Style.fontSizeS
                color: Color.mOnSurfaceVariant
            }

        }

        Rectangle {
            Layout.fillWidth: true
            implicitHeight: Style.marginM
            radius: implicitHeight / 2
            color: Qt.alpha(Color.mOnSurface, 0.12)

            Rectangle {
                width: parent.width * root.progressRatio()
                height: parent.height
                radius: parent.radius
                color: root.limitData ? root.accentColor : Qt.alpha(Color.mOnSurfaceVariant, 0.4)

                Behavior on width {
                    NumberAnimation {
                        duration: 180
                        easing.type: Easing.OutCubic
                    }

                }

            }

        }

        NText {
            text: root.resetText()
            pointSize: Style.fontSizeXS
            color: Color.mOnSurfaceVariant
        }

    }

}
