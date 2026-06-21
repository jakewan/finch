import QtQuick

// Test double for FinchClient: records createAccount and listTransactions calls and lets
// specs drive the resulting signals/data the components react to, with no daemon or gRPC.
QtObject {
    property bool createAccountInProgress: false
    property string lastName: ""
    property int lastType: -1
    property int callCount: 0
    // AccountsPanel binds its account list to this; default empty so the binding is clean.
    property var accounts: []

    // TransactionsPanel binds its ListView model to this; default empty so the binding is
    // clean. succeedTransactions() ASSIGNS it (not just emits) so the binding refreshes —
    // see succeedTransactions below.
    property var transactions: []
    property string lastAccountId: ""
    property int transactionsCallCount: 0
    property bool transactionsLoading: false

    signal accountCreated(string id)
    signal accountCreateFailed(string message)

    // Mirror the real client's observable lifecycle: in-progress latches true on the
    // call and clears when the test drives a terminal outcome via succeed()/fail().
    function createAccount(name, type) {
        lastName = name
        lastType = type
        callCount += 1
        createAccountInProgress = true
    }

    function succeed(id) {
        createAccountInProgress = false
        accountCreated(id)
    }

    function fail(message) {
        createAccountInProgress = false
        accountCreateFailed(message)
    }

    function listTransactions(accountId) {
        lastAccountId = accountId
        transactionsCallCount += 1
        transactionsLoading = true
    }

    // Assigning `transactions` IS the notification the panel's model binding reacts to —
    // exactly like the real client's transactionsChanged. A signal-only shape would leave
    // the bound ListView stale and every render assertion failing against an empty view.
    function succeedTransactions(list) {
        transactionsLoading = false
        transactions = list
    }
}
