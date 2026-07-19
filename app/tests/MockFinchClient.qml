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

    // RecordTransactionForm (embedded in TransactionsPanel) drives these; default in-progress
    // false so the embedded form starts enabled. Each field is captured separately so a spec
    // can catch a name<->description transpose in the 6-arg positional call.
    property bool recordTransactionInProgress: false
    property string lastRecordAccountId: ""
    property string lastRecordDate: ""
    property var lastRecordAmount: 0
    property string lastRecordName: ""
    property string lastRecordDescription: ""
    property int lastRecordStatus: -1
    property int recordCallCount: 0

    // CreateRecurringRuleForm drives these; default in-progress false so the form starts
    // enabled. Each arg is captured separately so a spec can catch a transpose in the 8-arg
    // positional call.
    property bool createRecurringRuleInProgress: false
    property string lastCreateAccountId: ""
    property string lastCreateName: ""
    property var lastCreateAmount: 0
    property int lastCreateFrequency: -1
    property string lastCreateStartDate: ""
    property string lastCreateEndDate: ""
    property int lastCreateDayOfMonth: -1
    property var lastCreateSemiMonthlyDays: []
    property int createRecurringRuleCallCount: 0

    signal accountCreated(string id)
    signal accountCreateFailed(string message)
    signal transactionRecorded(string id)
    signal transactionRecordFailed(string message)
    signal recurringRuleCreated(string id)
    signal recurringRuleCreateFailed(string message)

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

    // Mirror the real client: in-progress latches true synchronously on the call and clears
    // when the test drives a terminal outcome via succeedRecord()/failRecord().
    function recordTransaction(accountId, date, amount, name, description, status) {
        lastRecordAccountId = accountId
        lastRecordDate = date
        lastRecordAmount = amount
        lastRecordName = name
        lastRecordDescription = description
        lastRecordStatus = status
        recordCallCount += 1
        recordTransactionInProgress = true
    }

    // Clear in-progress BEFORE emitting so the after-success specs see an enabled form again.
    function succeedRecord(id) {
        recordTransactionInProgress = false
        transactionRecorded(id)
    }

    function failRecord(message) {
        recordTransactionInProgress = false
        transactionRecordFailed(message)
    }

    // Mirror the real client: in-progress latches true synchronously on the call and clears
    // when the test drives a terminal outcome via succeed/failCreateRecurringRule().
    function createRecurringRule(accountId, name, amount, frequency, startDate, endDate, dayOfMonth, semiMonthlyDays) {
        lastCreateAccountId = accountId
        lastCreateName = name
        lastCreateAmount = amount
        lastCreateFrequency = frequency
        lastCreateStartDate = startDate
        lastCreateEndDate = endDate
        lastCreateDayOfMonth = dayOfMonth
        lastCreateSemiMonthlyDays = semiMonthlyDays
        createRecurringRuleCallCount += 1
        createRecurringRuleInProgress = true
    }

    function succeedCreateRecurringRule(id) {
        createRecurringRuleInProgress = false
        recurringRuleCreated(id)
    }

    function failCreateRecurringRule(message) {
        createRecurringRuleInProgress = false
        recurringRuleCreateFailed(message)
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
