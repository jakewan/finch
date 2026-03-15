import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import QtCharts
import Finch 1.0

ColumnLayout {
    id: root
    spacing: 8

    property string selectedAccountId: {
        if (finchClient.accounts.length > 0)
            return finchClient.accounts[0].id
        return ""
    }

    function defaultFromDate() {
        return new Date().toISOString().slice(0, 10)
    }

    function defaultToDate() {
        let d = new Date()
        d.setMonth(d.getMonth() + 6)
        return d.toISOString().slice(0, 10)
    }

    function fetch() {
        if (selectedAccountId === "")
            return
        finchClient.fetchTimeSeries(fromField.text, toField.text,
                                    intervalBox.selectedInterval,
                                    [selectedAccountId])
    }

    Connections {
        target: finchClient
        function onAccountsChanged() {
            if (finchClient.accounts.length > 0)
                root.fetch()
        }
        function onTimeSeriesDataChanged() {
            if (!finchClient.timeSeriesEmpty) {
                finchClient.populateSeries(mainSeries, root.selectedAccountId)
                axisX.min = new Date(finchClient.timeSeriesMinDate)
                axisX.max = new Date(finchClient.timeSeriesMaxDate)
                axisY.min = finchClient.timeSeriesMinBalance
                axisY.max = finchClient.timeSeriesMaxBalance
            }
        }
    }

    RowLayout {
        Layout.fillWidth: true
        spacing: 8

        Label { text: "From:" }
        TextField {
            id: fromField
            text: root.defaultFromDate()
            placeholderText: "YYYY-MM-DD"
            Layout.preferredWidth: 120
        }

        Label { text: "To:" }
        TextField {
            id: toField
            text: root.defaultToDate()
            placeholderText: "YYYY-MM-DD"
            Layout.preferredWidth: 120
        }

        Label { text: "Interval:" }
        ComboBox {
            id: intervalBox
            model: ["Daily", "Weekly", "Monthly"]
            property var intervalValues: [1, 2, 3]
            property int selectedInterval: intervalValues[currentIndex]
            currentIndex: 1
        }

        Button {
            text: finchClient.timeSeriesLoading ? "Loading…" : "Fetch"
            enabled: !finchClient.timeSeriesLoading && root.selectedAccountId !== ""
            onClicked: root.fetch()
        }

        Item { Layout.fillWidth: true }
    }

    Item {
        Layout.fillWidth: true
        Layout.fillHeight: true

        ChartView {
            id: chartView
            anchors.fill: parent
            visible: !finchClient.timeSeriesEmpty
            antialiasing: true
            legend.visible: false

            DateTimeAxis {
                id: axisX
                format: "MMM yyyy"
            }

            ValueAxis {
                id: axisY
                labelFormat: "$%.0f"
            }

            LineSeries {
                id: mainSeries
                axisX: axisX
                axisY: axisY
            }
        }

        Label {
            anchors.centerIn: parent
            visible: !finchClient.accountsLoading && !finchClient.timeSeriesLoading
                     && finchClient.timeSeriesEmpty
            text: finchClient.accounts.length === 0
                  ? "No accounts found"
                  : "No data for selected range"
            font.pointSize: 12
            color: "gray"
        }

        BusyIndicator {
            anchors.centerIn: parent
            running: finchClient.timeSeriesLoading
        }
    }
}
