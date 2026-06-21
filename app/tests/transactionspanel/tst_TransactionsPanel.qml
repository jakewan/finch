import QtQuick
import QtTest
// FinchTransactionsTest is the test-only module (see app/CMakeLists.txt) holding the real
// shipping TransactionsPanel and the shared MockFinchClient double — scoped to just those
// two files so the test doesn't pull in the rest of the app's QML and its modules.
import FinchTransactionsTest

TestCase {
    id: testCase
    name: "TransactionsPanel"
    when: windowShown
    width: 640
    height: 480

    Component {
        id: panelFactory
        TransactionsPanel {}
    }

    Component {
        id: mockFactory
        MockFinchClient {}
    }

    property var currentPanel: null
    property var currentMock: null

    // Two accounts so an account-switch test has a second option to select.
    readonly property var sampleAccounts: [
        { id: "a1", name: "Checking" },
        { id: "a2", name: "Savings" }
    ]

    // One reconciled credit (positive) and one projected debit (negative), so a single
    // fixture pins both the sign-correct amount format and the status->label mapping.
    readonly property var sampleTransactions: [
        { id: "t1", accountId: "a1", date: "2026-01-15", amount: 5000,
          name: "Paycheck", description: "Jan salary", status: 3, recurringRuleId: "r1" },
        { id: "t2", accountId: "a1", date: "2026-01-20", amount: -1234,
          name: "Coffee", description: "", status: 1, recurringRuleId: "" }
    ]

    // Returns { panel, mock }; accounts are set on the mock BEFORE the panel is built so the
    // ComboBox model is already populated at completion — the realistic case, and the only
    // way to prove the panel does not auto-fetch despite a non-empty account list.
    function build(opts) {
        opts = opts || {}
        currentMock = mockFactory.createObject(testCase)
        currentMock.accounts = (opts.accounts !== undefined) ? opts.accounts : sampleAccounts
        var props = { client: currentMock }
        props.connected = (opts.connected !== undefined) ? opts.connected : true
        currentPanel = panelFactory.createObject(testCase, props)
        verify(currentPanel !== null, "panel instantiated")
        return { panel: currentPanel, mock: currentMock }
    }

    function cleanup() {
        if (currentPanel) { currentPanel.destroy(); currentPanel = null }
        if (currentMock) { currentMock.destroy(); currentMock = null }
    }

    // Helper: select an account and deliver a transaction list in one step.
    function selectAccountWith(ctx, index, txns) {
        ctx.panel.selectedAccountIndex = index
        ctx.mock.succeedTransactions(txns)
    }

    function test_loadsAndStartsUnselected() {
        var ctx = build()
        compare(ctx.panel.selectedAccountIndex, -1)
        compare(ctx.mock.transactionsCallCount, 0)
    }

    // A fresh panel with accounts present must NOT auto-fetch; the fetch fires only once a
    // real account is chosen (the ComboBox starts at currentIndex -1, not an index-0 default).
    function test_noFetchUntilAccountSelected() {
        var ctx = build()
        compare(ctx.mock.transactionsCallCount, 0)
        ctx.panel.selectedAccountIndex = 0
        compare(ctx.mock.transactionsCallCount, 1)
        compare(ctx.mock.lastAccountId, "a1")
    }

    function test_selectingAccountListsThatAccount() {
        var ctx = build()
        ctx.panel.selectedAccountIndex = 1
        compare(ctx.mock.transactionsCallCount, 1)
        compare(ctx.mock.lastAccountId, "a2")
    }

    function test_changingAccountRefetches() {
        var ctx = build()
        ctx.panel.selectedAccountIndex = 0
        ctx.panel.selectedAccountIndex = 1
        compare(ctx.mock.transactionsCallCount, 2)
        compare(ctx.mock.lastAccountId, "a2")
    }

    function test_transactionsRenderAsRows() {
        var ctx = build()
        selectAccountWith(ctx, 0, sampleTransactions)
        compare(ctx.panel.transactionList.count, 2)
    }

    function test_selectingRowFillsDetail() {
        var ctx = build()
        selectAccountWith(ctx, 0, sampleTransactions)
        ctx.panel.transactionList.currentIndex = 0
        verify(ctx.panel.selectedTransaction !== null)
        compare(ctx.panel.detailNameText, "Paycheck")
        compare(ctx.panel.detailDateText, "2026-01-15")
        compare(ctx.panel.detailDescriptionText, "Jan salary")
        compare(ctx.panel.detailRecurringRuleText, "r1")
    }

    function test_positiveAmountFormatted() {
        var ctx = build()
        selectAccountWith(ctx, 0, sampleTransactions)
        ctx.panel.transactionList.currentIndex = 0
        compare(ctx.panel.detailAmountText, "$50.00")
    }

    // The negative row pins the sign-correct format: "-$12.34", never "$-12.34".
    function test_negativeAmountFormattedWithSign() {
        var ctx = build()
        selectAccountWith(ctx, 0, sampleTransactions)
        ctx.panel.transactionList.currentIndex = 1
        compare(ctx.panel.detailAmountText, "-$12.34")
    }

    function test_statusRenderedAsLabelNotInt() {
        var ctx = build()
        selectAccountWith(ctx, 0, sampleTransactions)
        ctx.panel.transactionList.currentIndex = 0
        compare(ctx.panel.detailStatusText, "Reconciled")
        ctx.panel.transactionList.currentIndex = 1
        compare(ctx.panel.detailStatusText, "Projected")
    }

    function test_emptyResultShowsNoTransactions() {
        var ctx = build()
        selectAccountWith(ctx, 0, [])
        verify(ctx.panel.stateMessageVisible)
        compare(ctx.panel.stateMessageText, "No transactions for this account.")
    }

    function test_noAccountSelectedShowsPrompt() {
        var ctx = build()
        verify(ctx.panel.stateMessageVisible)
        compare(ctx.panel.stateMessageText, "Select an account to view its transactions.")
    }

    function test_disconnectedShowsConnectHint() {
        var ctx = build({ connected: false, accounts: [] })
        verify(ctx.panel.stateMessageVisible)
        compare(ctx.panel.stateMessageText, "Connect to the daemon to view transactions.")
    }

    // Switching the selected account must drop the prior detail selection, so the detail
    // pane never reads a row index that no longer exists in the incoming list.
    function test_changingAccountResetsSelection() {
        var ctx = build()
        selectAccountWith(ctx, 0, sampleTransactions)
        ctx.panel.transactionList.currentIndex = 0
        verify(ctx.panel.selectedTransaction !== null)
        ctx.panel.selectedAccountIndex = 1
        compare(ctx.panel.transactionList.currentIndex, -1)
        verify(ctx.panel.selectedTransaction === null)
        // Follow through with the new account's (shorter) data: the selection must stay
        // cleared once the swapped list actually arrives, not only at the synchronous reset.
        ctx.mock.succeedTransactions([sampleTransactions[1]])
        compare(ctx.panel.transactionList.count, 1)
        compare(ctx.panel.transactionList.currentIndex, -1)
        verify(ctx.panel.selectedTransaction === null)
    }

    // Switching account clears the prior account's rows immediately, during the new fetch's
    // in-flight window — the list never shows account A's transactions under account B.
    function test_switchingAccountClearsPriorRowsWhileLoading() {
        var ctx = build()
        selectAccountWith(ctx, 0, sampleTransactions)
        compare(ctx.panel.transactionList.count, 2)
        ctx.panel.selectedAccountIndex = 1          // new fetch in flight, no data yet
        verify(ctx.mock.transactionsLoading)
        compare(ctx.panel.transactionList.count, 0) // account 0's rows are gone, not stale
    }

    // Rapid account switch: selecting B while A's fetch is still in flight must end with B's
    // transactions shown, never A's. The client defers B's fetch behind A's, then on A's
    // completion discovers the selection moved on, discards A's result, and re-fetches B.
    function test_midFetchSwitchReconcilesToRequestedAccount() {
        var ctx = build()
        ctx.panel.selectedAccountIndex = 0          // A: fetch in flight
        verify(ctx.mock.transactionsLoading)
        ctx.panel.selectedAccountIndex = 1          // B requested before A completes
        compare(ctx.mock.lastAccountId, "a2")
        // A completes — but B is now selected, so A's data must be discarded, not applied,
        // and B re-fetched (the list passed here stands in for A's data).
        ctx.mock.succeedTransactions(sampleTransactions)
        verify(ctx.mock.transactionsLoading)        // reconciling: B's re-fetch in flight
        compare(ctx.panel.transactionList.count, 0) // A's data was NOT applied
        // B completes; its data finally lands.
        ctx.mock.succeedTransactions([sampleTransactions[1]])
        verify(!ctx.mock.transactionsLoading)
        compare(ctx.panel.transactionList.count, 1)
    }

    // New data for the same account also clears the prior selection (a stale index could
    // point past the end of a shorter incoming list).
    function test_newDataResetsSelection() {
        var ctx = build()
        selectAccountWith(ctx, 0, sampleTransactions)
        ctx.panel.transactionList.currentIndex = 1
        ctx.mock.succeedTransactions([])
        compare(ctx.panel.transactionList.currentIndex, -1)
        verify(ctx.panel.selectedTransaction === null)
    }
}
