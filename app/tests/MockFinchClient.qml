import QtQuick

// Test double for FinchClient: records createAccount calls and lets specs drive the
// success/failure signals the form reacts to, with no daemon or gRPC involved.
QtObject {
    property bool createAccountInProgress: false
    property string lastName: ""
    property int lastType: -1
    property int callCount: 0

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
}
