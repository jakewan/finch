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
    // Faithful to the real client's single-in-flight reconciliation (see listTransactions /
    // succeedTransactions): the account the running fetch is for, vs. the account most
    // recently requested. When they diverge, a stale completion is discarded and the
    // requested account re-fetched — so panel specs see the real states during a rapid
    // account switch rather than an unrealistic two-concurrent-fetch model.
    property string inFlightAccountId: ""

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

    // callCount counts invocations (every call). The re-entrancy guard mirrors the real
    // client: a second request while a fetch is in flight is recorded as the new requested
    // account but does NOT start a concurrent fetch — it is reconciled on completion.
    function listTransactions(accountId) {
        lastAccountId = accountId
        transactionsCallCount += 1
        if (transactionsLoading)
            return
        inFlightAccountId = accountId
        transactionsLoading = true
        transactions = []   // clear at fetch start, like the real client
    }

    // Models the in-flight fetch (for inFlightAccountId) completing. Assigning `transactions`
    // IS the notification the panel's model binding reacts to — exactly like the real
    // client's transactionsChanged; a signal-only shape would leave the bound ListView stale.
    // If a newer account was requested meanwhile, this completion is stale: discard `list`
    // and re-enter the in-flight state for the requested account (the spec drives the next
    // succeedTransactions to complete that reconciled fetch).
    function succeedTransactions(list) {
        transactionsLoading = false
        if (inFlightAccountId !== lastAccountId) {
            inFlightAccountId = lastAccountId
            transactionsLoading = true
            transactions = []   // re-fetch clears too, like the real client
            return
        }
        transactions = list
    }
}
