import QtQuick
import QtTest
// FinchFormTest is the test-only module (see app/CMakeLists.txt) holding the real
// shipping CreateAccountForm and the MockFinchClient double — scoped to just these two
// files so the test doesn't pull in the rest of the app's QML and its modules.
import FinchFormTest

TestCase {
    id: testCase
    name: "CreateAccountForm"
    when: windowShown

    Component {
        id: formFactory
        CreateAccountForm {}
    }

    Component {
        id: mockFactory
        MockFinchClient {}
    }

    SignalSpy {
        id: createdSpy
        signalName: "created"
    }

    property var currentForm: null
    property var currentMock: null

    // Returns { form, mock }; cleanup() destroys them after each test — even on an
    // assertion failure, which aborts the test function before any manual teardown runs.
    function build() {
        currentMock = mockFactory.createObject(testCase)
        currentForm = formFactory.createObject(testCase, { client: currentMock })
        verify(currentForm !== null, "form instantiated")
        return { form: currentForm, mock: currentMock }
    }

    // cleanup() is the Qt Quick Test per-test teardown hook; it always runs.
    function cleanup() {
        createdSpy.target = null
        if (currentForm) { currentForm.destroy(); currentForm = null }
        if (currentMock) { currentMock.destroy(); currentMock = null }
    }

    // C1 load-smoke: the real component instantiates and a known property reads back.
    function test_loadsAndStartsInvalid() {
        var ctx = build()
        compare(ctx.form.canSubmit, false)
    }

    function test_disabledWhenNameEmpty() {
        var ctx = build()
        ctx.form.typeIndex = 1 // Checking
        compare(ctx.form.nameText, "")
        compare(ctx.form.canSubmit, false)
    }

    function test_disabledWhenNoType() {
        var ctx = build()
        ctx.form.nameText = "Everyday"
        compare(ctx.form.selectedType, 0)
        compare(ctx.form.canSubmit, false)
    }

    function test_enabledWhenNameAndTypeValid() {
        var ctx = build()
        ctx.form.nameText = "Everyday"
        ctx.form.typeIndex = 1
        compare(ctx.form.canSubmit, true)
    }

    function test_submitSendsTrimmedNameAndSelectedType() {
        var ctx = build()
        ctx.form.nameText = "  Rainy Day  "
        ctx.form.typeIndex = 2 // Savings -> value 2
        ctx.form.submit()
        compare(ctx.mock.callCount, 1)
        compare(ctx.mock.lastName, "Rainy Day")
        compare(ctx.mock.lastType, 2)
    }

    function test_submitIgnoredWhenInvalid() {
        var ctx = build()
        ctx.form.nameText = "   " // whitespace only
        ctx.form.typeIndex = 1
        ctx.form.submit()
        compare(ctx.mock.callCount, 0)
    }

    function test_disabledWhileCreateInProgress() {
        var ctx = build()
        ctx.form.nameText = "Everyday"
        ctx.form.typeIndex = 1
        verify(ctx.form.canSubmit) // enabled before submit
        ctx.form.submit() // mock latches createAccountInProgress
        compare(ctx.mock.createAccountInProgress, true)
        compare(ctx.form.canSubmit, false) // form disables while a create is in flight
    }

    function test_doubleSubmitSendsOnlyOne() {
        var ctx = build()
        ctx.form.nameText = "Everyday"
        ctx.form.typeIndex = 1
        ctx.form.submit()
        ctx.form.submit() // second is gated by canSubmit (in-progress)
        compare(ctx.mock.callCount, 1)
    }

    function test_successClearsFormAndSignalsCreated() {
        var ctx = build()
        createdSpy.target = ctx.form
        createdSpy.clear()
        ctx.form.nameText = "Everyday"
        ctx.form.typeIndex = 1
        ctx.form.submit()
        ctx.mock.succeed("acct-123")
        compare(createdSpy.count, 1)
        compare(ctx.form.nameText, "")
        compare(ctx.form.selectedType, 0)
        createdSpy.target = null
    }

    function test_failureShowsErrorMessage() {
        var ctx = build()
        ctx.form.nameText = "Everyday"
        ctx.form.typeIndex = 1
        ctx.form.submit()
        ctx.mock.fail("could not reach the daemon")
        compare(ctx.form.errorText, "could not reach the daemon")
    }
}
