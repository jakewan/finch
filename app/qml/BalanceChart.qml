import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import QtCharts
import Finch 1.0

ColumnLayout {
    id: root
    spacing: 8

    property var selectedAccountIds: []
    property var dynamicSeries: []
    property bool pendingFetch: false

    // D3 "tab10" palette — colorblind-considerate, widely used
    readonly property var colorPalette: [
        "#1f77b4", "#ff7f0e", "#2ca02c", "#d62728", "#9467bd",
        "#8c564b", "#e377c2", "#7f7f7f", "#bcbd22", "#17becf"
    ]

    // Chart is visible when there are selected accounts AND data exists
    readonly property bool chartVisible: selectedAccountIds.length > 0
                                         && !finchClient.timeSeriesEmpty

    function defaultFromDate() {
        return new Date().toISOString().slice(0, 10)
    }

    function defaultToDate() {
        let d = new Date()
        d.setMonth(d.getMonth() + 6)
        return d.toISOString().slice(0, 10)
    }

    function fetch() {
        if (selectedAccountIds.length === 0)
            return
        if (finchClient.timeSeriesLoading) {
            pendingFetch = true
            return
        }
        finchClient.fetchTimeSeries(fromField.text, toField.text,
                                    intervalBox.selectedInterval,
                                    selectedAccountIds)
    }

    function accountColor(accountId) {
        for (let i = 0; i < finchClient.accounts.length; i++) {
            if (finchClient.accounts[i].id === accountId)
                return colorPalette[i % colorPalette.length]
        }
        return colorPalette[0]
    }

    function clearSeries() {
        for (let i = 0; i < dynamicSeries.length; i++) {
            chartView.removeSeries(dynamicSeries[i])
            dynamicSeries[i].destroy()
        }
        dynamicSeries = []
    }

    function rebuildSeries() {
        clearSeries()

        let ids = finchClient.timeSeriesAccountIds
        let created = []
        for (let i = 0; i < ids.length; i++) {
            let accountId = ids[i]
            if (selectedAccountIds.indexOf(accountId) === -1)
                continue
            let name = accountId
            for (let j = 0; j < finchClient.accounts.length; j++) {
                if (finchClient.accounts[j].id === accountId) {
                    name = finchClient.accounts[j].name
                    break
                }
            }
            let series = chartView.createSeries(ChartView.SeriesTypeLine,
                                                 name, axisX, axisY)
            series.color = root.accountColor(accountId)
            finchClient.populateSeries(series, accountId)
            created.push(series)
        }
        dynamicSeries = created
    }

    Timer {
        id: fetchDebounce
        interval: 300
        onTriggered: root.fetch()
    }

    Connections {
        target: finchClient
        function onAccountsChanged() {
            if (finchClient.accounts.length > 0) {
                if (root.selectedAccountIds.length === 0) {
                    // First load — select all accounts
                    let ids = []
                    for (let i = 0; i < finchClient.accounts.length; i++)
                        ids.push(finchClient.accounts[i].id)
                    root.selectedAccountIds = ids
                } else {
                    // Preserve selections that still exist, add new accounts
                    let validIds = []
                    let knownIds = root.selectedAccountIds
                    for (let i = 0; i < finchClient.accounts.length; i++) {
                        let id = finchClient.accounts[i].id
                        if (knownIds.indexOf(id) !== -1)
                            validIds.push(id)
                    }
                    // Auto-select any newly added accounts
                    for (let i = 0; i < finchClient.accounts.length; i++) {
                        let id = finchClient.accounts[i].id
                        if (validIds.indexOf(id) === -1)
                            validIds.push(id)
                    }
                    root.selectedAccountIds = validIds
                }
                root.fetch()
            }
        }
        function onTimeSeriesDataChanged() {
            if (!finchClient.timeSeriesEmpty) {
                axisX.min = new Date(finchClient.timeSeriesMinDate)
                axisX.max = new Date(finchClient.timeSeriesMaxDate)
                axisY.min = finchClient.timeSeriesMinBalance
                axisY.max = finchClient.timeSeriesMaxBalance
            }
            root.rebuildSeries()
        }
        function onTimeSeriesLoadingChanged() {
            if (!finchClient.timeSeriesLoading && root.pendingFetch) {
                root.pendingFetch = false
                root.fetch()
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
            enabled: !finchClient.timeSeriesLoading
                     && root.selectedAccountIds.length > 0
            onClicked: root.fetch()
        }

        Item { Layout.fillWidth: true }
    }

    RowLayout {
        Layout.fillWidth: true
        Layout.fillHeight: true
        spacing: 0

        ColumnLayout {
            Layout.preferredWidth: 180
            Layout.maximumWidth: 180
            Layout.fillHeight: true
            spacing: 4

            Label {
                text: "Accounts"
                font.bold: true
                Layout.leftMargin: 8
            }

            ScrollView {
                Layout.fillWidth: true
                Layout.fillHeight: true

                ListView {
                    model: finchClient.accounts
                    delegate: CheckDelegate {
                        width: ListView.view.width
                        text: modelData.name
                        checked: root.selectedAccountIds.indexOf(modelData.id) !== -1
                        onToggled: {
                            let ids = root.selectedAccountIds.slice()
                            let idx = ids.indexOf(modelData.id)
                            if (checked && idx === -1)
                                ids.push(modelData.id)
                            else if (!checked && idx !== -1)
                                ids.splice(idx, 1)
                            root.selectedAccountIds = ids
                            if (ids.length === 0)
                                root.clearSeries()
                            else
                                fetchDebounce.restart()
                        }
                    }
                }
            }
        }

        Item {
            Layout.fillWidth: true
            Layout.fillHeight: true

            ChartView {
                id: chartView
                anchors.fill: parent
                visible: root.chartVisible
                antialiasing: true
                legend.visible: true
                theme: ChartView.ChartThemeLight

                DateTimeAxis {
                    id: axisX
                    format: "MMM yyyy"
                }

                ValueAxis {
                    id: axisY
                    labelFormat: "$%.0f"
                }

                LineSeries {
                    id: anchorSeries
                    axisX: axisX
                    axisY: axisY
                    visible: false
                }
            }

            Label {
                anchors.centerIn: parent
                visible: !finchClient.accountsLoading
                         && !finchClient.timeSeriesLoading
                         && !root.chartVisible
                text: {
                    if (finchClient.accounts.length === 0)
                        return "No accounts found"
                    if (root.selectedAccountIds.length === 0)
                        return "No accounts selected"
                    return "No data for selected range"
                }
                font.pointSize: 12
                color: "gray"
            }

            BusyIndicator {
                anchors.centerIn: parent
                running: finchClient.timeSeriesLoading
            }
        }
    }
}
