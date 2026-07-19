import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import QtCore
import Finch 1.0

ApplicationWindow {
    id: window
    visible: true
    width: 1024
    height: 768
    minimumWidth: 640
    minimumHeight: 480
    title: "Finch"

    Settings {
        property alias x: window.x
        property alias y: window.y
        property alias width: window.width
        property alias height: window.height
    }

    header: ToolBar {
        RowLayout {
            anchors.fill: parent
            anchors.leftMargin: 8
            anchors.rightMargin: 8

            Label {
                text: "Finch"
                font.pointSize: 14
            }

            Item { Layout.fillWidth: true }

            Button {
                text: finchClient.pingInProgress ? "Pinging…" : "Ping Daemon"
                enabled: !finchClient.pingInProgress
                onClicked: finchClient.ping()
            }
        }
    }

    Connections {
        target: finchClient
        function onConnectionStateChanged() {
            if (finchClient.connectionState === FinchClient.Connected) {
                finchClient.listAccounts()
            }
        }
    }

    NavigationShell {
        anchors.fill: parent
        destinationTitles: ["Overview", "Accounts", "Transactions", "Recurring Rules"]
        screens: [
            // Overview: the balance chart, with its disconnected empty-state and ping
            // spinner. Wrapped in a plain Item so the chart can anchors.fill — anchors are
            // illegal on a direct StackLayout child, but legal inside one.
            Item {
                Layout.fillWidth: true
                Layout.fillHeight: true

                BalanceChart {
                    anchors.fill: parent
                    anchors.margins: 16
                    visible: finchClient.connectionState === FinchClient.Connected
                             || !finchClient.timeSeriesEmpty
                }

                Label {
                    anchors.centerIn: parent
                    visible: finchClient.connectionState === FinchClient.Disconnected
                             && finchClient.timeSeriesEmpty
                    text: "Not connected to daemon"
                    font.pointSize: 12
                    color: "gray"
                }

                BusyIndicator {
                    anchors.centerIn: parent
                    running: finchClient.pingInProgress
                             && finchClient.timeSeriesEmpty
                }
            },

            // Accounts: the create form (gated on a live daemon, as the old toolbar button
            // was) plus a read-only list of existing accounts.
            AccountsPanel {
                Layout.fillWidth: true
                Layout.fillHeight: true
                client: finchClient
                createEnabled: finchClient.connectionState === FinchClient.Connected
            },

            TransactionsPanel {
                Layout.fillWidth: true
                Layout.fillHeight: true
                client: finchClient
                connected: finchClient.connectionState === FinchClient.Connected
            },

            RecurringRulesPanel {
                Layout.fillWidth: true
                Layout.fillHeight: true
                client: finchClient
                connected: finchClient.connectionState === FinchClient.Connected
            }
        ]
    }

    footer: ToolBar {
        RowLayout {
            anchors.fill: parent
            anchors.leftMargin: 8
            anchors.rightMargin: 8

            Rectangle {
                width: 8
                height: 8
                radius: 4
                color: {
                    switch (finchClient.connectionState) {
                    case FinchClient.Connecting:
                        return "gray"
                    case FinchClient.Connected:
                        return "green"
                    default:
                        return "red"
                    }
                }
            }

            Label {
                font.pointSize: 10
                text: {
                    switch (finchClient.connectionState) {
                    case FinchClient.Connecting:
                        return "Connecting to daemon…"
                    case FinchClient.Connected:
                        return "Connected to daemon v" + finchClient.daemonVersion
                    default:
                        return "Not connected to daemon"
                    }
                }
            }
        }
    }
}
