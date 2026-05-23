import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import qs.Commons
import qs.Widgets

Item {
    id: root

    property var pluginApi: null
    property var service: pluginApi && pluginApi.mainInstance ? pluginApi.mainInstance.service : null
    readonly property var geometryPlaceholder: panelContainer
    readonly property bool allowAttach: true
    property real contentPreferredWidth: 420 * Style.uiScaleRatio
    property real contentPreferredHeight: Math.min(680 * Style.uiScaleRatio, panelContent.implicitHeight + Style.marginL * 2)
    property double nowMs: Date.now()

    function formatUpdated(value) {
        if (!value)
            return "Updated never";

        const at = new Date(value);
        if (isNaN(at.getTime()))
            return "Updated never";

        const deltaMs = Math.max(0, root.nowMs - at.getTime());
        const deltaMinutes = Math.floor(deltaMs / 60000);
        if (deltaMinutes < 1)
            return "Updated just now";

        if (deltaMinutes < 60)
            return `Updated ${deltaMinutes} minute${deltaMinutes === 1 ? "" : "s"} ago`;

        const deltaHours = Math.floor(deltaMinutes / 60);
        if (deltaHours < 24)
            return `Updated ${deltaHours} hour${deltaHours === 1 ? "" : "s"} ago`;

        const deltaDays = Math.floor(deltaHours / 24);
        return `Updated ${deltaDays} day${deltaDays === 1 ? "" : "s"} ago`;
    }

    function formatAbsoluteDate(value) {
        if (!value)
            return "unknown date";

        const at = new Date(value);
        return isNaN(at.getTime()) ? "unknown date" : at.toLocaleString();
    }

    function hasRenewableSubscription(account) {
        if (!account || !account.subscription || !account.subscription.active_until)
            return false;

        const renewalAt = new Date(account.subscription.active_until);
        const validSubscriptionCutoff = new Date("1970-01-02T00:00:00Z");
        return !isNaN(renewalAt.getTime()) && renewalAt >= validSubscriptionCutoff;
    }

    function visibleAccounts() {
        const accounts = service && service.accounts ? service.accounts : [];
        return accounts.filter((account) => {
            return root.hasRenewableSubscription(account);
        });
    }

    anchors.fill: parent

    Timer {
        interval: 60000
        running: true
        repeat: true
        onTriggered: root.nowMs = Date.now()
    }

    Connections {
        function onLastUpdatedChanged() {
            root.nowMs = Date.now();
        }

        target: service
    }

    Rectangle {
        id: panelContainer

        anchors.fill: parent
        color: "transparent"

        ScrollView {
            anchors.fill: parent
            anchors.margins: Style.marginL
            clip: true

            ColumnLayout {
                id: panelContent

                width: panelContainer.width - Style.marginL * 2
                spacing: Style.marginM

                Rectangle {
                    Layout.fillWidth: true
                    color: "transparent"
                    implicitHeight: headerLayout.implicitHeight

                    RowLayout {
                        id: headerLayout

                        anchors.fill: parent
                        spacing: Style.marginS

                        ColumnLayout {
                            Layout.fillWidth: true
                            spacing: 2

                            NText {
                                text: "OpenAI Accounts"
                                pointSize: Style.fontSizeL
                                font.weight: Font.DemiBold
                                color: Color.mOnSurface
                            }

                            NText {
                                text: service && service.loading ? "Refreshing snapshot…" : root.formatUpdated(service ? service.lastUpdated : "")
                                pointSize: Style.fontSizeS
                                color: Color.mOnSurfaceVariant
                            }

                        }

                        NButton {
                            icon: "refresh"
                            text: service && service.loading ? "Refreshing" : "Refresh"
                            enabled: !!service && !service.loading
                            onClicked: service.refresh(true)
                        }

                        NIconButton {
                            icon: "x"
                            onClicked: {
                                if (pluginApi && pluginApi.closePanel)
                                    pluginApi.closePanel(pluginApi.panelOpenScreen);

                            }
                        }

                    }

                }

                Rectangle {
                    Layout.fillWidth: true
                    radius: Style.radiusL
                    color: Style.capsuleColor
                    border.color: Color.mOutline
                    border.width: 1
                    implicitHeight: recommendationLayout.implicitHeight + Style.marginM * 2

                    ColumnLayout {
                        id: recommendationLayout

                        anchors.fill: parent
                        anchors.margins: Style.marginM
                        spacing: Style.marginS

                        NText {
                            text: "Recommendation"
                            pointSize: Style.fontSizeM
                            font.weight: Font.Medium
                            color: Color.mOnSurface
                        }

                        NText {
                            text: service ? service.recommendationLabel || "No snapshot yet" : "No snapshot yet"
                            pointSize: Style.fontSizeL
                            color: service && service.hasRecommendation ? Color.mPrimary : Color.mOnSurface
                        }

                        NText {
                            text: service && service.recommendation ? service.recommendation.message || "" : ""
                            visible: text !== ""
                            pointSize: Style.fontSizeS
                            color: Color.mOnSurfaceVariant
                        }

                    }

                }

                Rectangle {
                    Layout.fillWidth: true
                    visible: !!(service && service.errorText)
                    radius: Style.radiusL
                    color: "transparent"
                    border.color: Color.mTertiary
                    border.width: 1
                    implicitHeight: errorTextItem.implicitHeight + Style.marginM * 2

                    NText {
                        id: errorTextItem

                        anchors.fill: parent
                        anchors.margins: Style.marginM
                        text: service ? service.errorText || "" : ""
                        wrapMode: Text.Wrap
                        pointSize: Style.fontSizeS
                        color: Color.mTertiary
                    }

                }

                Rectangle {
                    Layout.fillWidth: true
                    visible: !!(service && service.actionText)
                    radius: Style.radiusL
                    color: "transparent"
                    border.color: Color.mPrimary
                    border.width: 1
                    implicitHeight: actionTextItem.implicitHeight + Style.marginM * 2

                    NText {
                        id: actionTextItem

                        anchors.fill: parent
                        anchors.margins: Style.marginM
                        text: service ? service.actionText || "" : ""
                        wrapMode: Text.Wrap
                        pointSize: Style.fontSizeS
                        color: Color.mPrimary
                    }

                }

                ColumnLayout {
                    Layout.fillWidth: true
                    spacing: Style.marginS

                    NText {
                        text: "Sync targets"
                        pointSize: Style.fontSizeM
                        font.weight: Font.Medium
                        color: Color.mOnSurface
                    }

                    Flow {
                        Layout.fillWidth: true
                        spacing: Style.marginS

                        Repeater {
                            model: service ? service.syncTargets || [] : []

                            delegate: NButton {
                                required property var modelData

                                text: modelData.label
                                enabled: !!service && !service.actionRunning
                                onClicked: service.syncTarget(modelData.id)
                            }

                        }

                    }

                }

                ColumnLayout {
                    Layout.fillWidth: true
                    spacing: Style.marginS
                    visible: (service && service.warnings ? service.warnings : []).length > 0

                    NText {
                        text: "Warnings"
                        pointSize: Style.fontSizeM
                        font.weight: Font.Medium
                        color: Color.mOnSurface
                    }

                    Repeater {
                        model: service ? service.warnings || [] : []

                        delegate: Rectangle {
                            required property var modelData

                            Layout.fillWidth: true
                            radius: Style.radiusL
                            color: "transparent"
                            border.color: Color.mOutline
                            border.width: 1
                            implicitHeight: warningLayout.implicitHeight + Style.marginM * 2

                            ColumnLayout {
                                id: warningLayout

                                anchors.fill: parent
                                anchors.margins: Style.marginM
                                spacing: 2

                                NText {
                                    text: modelData.account_id || "account"
                                    pointSize: Style.fontSizeS
                                    font.weight: Font.Medium
                                    color: Color.mOnSurface
                                }

                                NText {
                                    text: modelData.message || ""
                                    wrapMode: Text.Wrap
                                    pointSize: Style.fontSizeS
                                    color: Color.mOnSurfaceVariant
                                }

                            }

                        }

                    }

                }

                ColumnLayout {
                    Layout.fillWidth: true
                    spacing: Style.marginS

                    NText {
                        text: "Accounts"
                        pointSize: Style.fontSizeM
                        font.weight: Font.Medium
                        color: Color.mOnSurface
                    }

                    Repeater {
                        model: root.visibleAccounts()

                        delegate: Rectangle {
                            required property var modelData

                            Layout.fillWidth: true
                            radius: Style.radiusL
                            color: Style.capsuleColor
                            border.color: modelData.recommendation && modelData.recommendation.selected ? Color.mPrimary : Color.mOutline
                            border.width: 1
                            implicitHeight: accountLayout.implicitHeight + Style.marginM * 2

                            ColumnLayout {
                                id: accountLayout

                                anchors.fill: parent
                                anchors.margins: Style.marginM
                                spacing: Style.marginS

                                RowLayout {
                                    Layout.fillWidth: true

                                    ColumnLayout {
                                        Layout.fillWidth: true
                                        spacing: 2

                                        NText {
                                            text: modelData.name || modelData.id
                                            pointSize: Style.fontSizeM
                                            font.weight: Font.Medium
                                            color: Color.mOnSurface
                                        }

                                        NText {
                                            text: `${modelData.id} • ${modelData.plan_type || modelData.model || "unknown plan"}`
                                            pointSize: Style.fontSizeS
                                            color: Color.mOnSurfaceVariant
                                        }

                                    }

                                    NText {
                                        text: modelData.recommendation && modelData.recommendation.selected ? "Recommended" : ((modelData.recommendation && modelData.recommendation.eligible) ? "Eligible" : "Unavailable")
                                        pointSize: Style.fontSizeS
                                        color: modelData.recommendation && modelData.recommendation.selected ? Color.mPrimary : Color.mOnSurfaceVariant
                                    }

                                }

                                NText {
                                    text: `Auth: ${modelData.auth_method || "none"}${modelData.auth_configured ? "" : " (not configured)"}`
                                    pointSize: Style.fontSizeS
                                    color: Color.mOnSurfaceVariant
                                }

                                LimitBar {
                                    Layout.fillWidth: true
                                    label: "Daily"
                                    limitData: modelData.daily
                                    accentColor: Color.mPrimary
                                }

                                LimitBar {
                                    Layout.fillWidth: true
                                    label: "Weekly"
                                    limitData: modelData.weekly
                                    accentColor: Color.mSecondary
                                }

                                NText {
                                    visible: !!modelData.subscription
                                    Layout.fillWidth: true
                                    text: modelData.subscription ? `Subscription until ${root.formatAbsoluteDate(modelData.subscription.active_until)}` : ""
                                    wrapMode: Text.Wrap
                                    pointSize: Style.fontSizeS
                                    color: Color.mOnSurfaceVariant
                                }

                            }

                        }

                    }

                    NText {
                        visible: root.visibleAccounts().length === 0
                        text: "No subscribed accounts to display"
                        pointSize: Style.fontSizeS
                        color: Color.mOnSurfaceVariant
                    }

                }

            }

        }

    }

}
