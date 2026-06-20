import QtQuick
import QtTest
// Load the real shipping form by directory import, so the component under test is the
// exact file the app uses — not a stub. MockFinchClient sits in this directory and is
// available unqualified.
import "../qml"

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

    // Returns { form, mock }; each test destroys them via teardown().
    function build() {
        var mock = mockFactory.createObject(testCase)
        var form = formFactory.createObject(testCase, { client: mock })
        verify(form !== null, "form instantiated")
        return { form: form, mock: mock }
    }

    function teardown(ctx) {
        ctx.form.destroy()
        ctx.mock.destroy()
    }

    // C1 load-smoke: the real component instantiates and a known property reads back.
    function test_loadsAndStartsInvalid() {
        var ctx = build()
        compare(ctx.form.canSubmit, false)
        teardown(ctx)
    }

    function test_disabledWhenNameEmpty() {
        var ctx = build()
        ctx.form.typeIndex = 1 // Checking
        compare(ctx.form.nameText, "")
        compare(ctx.form.canSubmit, false)
        teardown(ctx)
    }

    function test_disabledWhenNoType() {
        var ctx = build()
        ctx.form.nameText = "Everyday"
        compare(ctx.form.selectedType, 0)
        compare(ctx.form.canSubmit, false)
        teardown(ctx)
    }

    function test_enabledWhenNameAndTypeValid() {
        var ctx = build()
        ctx.form.nameText = "Everyday"
        ctx.form.typeIndex = 1
        compare(ctx.form.canSubmit, true)
        teardown(ctx)
    }

    function test_submitSendsTrimmedNameAndSelectedType() {
        var ctx = build()
        ctx.form.nameText = "  Rainy Day  "
        ctx.form.typeIndex = 2 // Savings -> value 2
        ctx.form.submit()
        compare(ctx.mock.callCount, 1)
        compare(ctx.mock.lastName, "Rainy Day")
        compare(ctx.mock.lastType, 2)
        teardown(ctx)
    }

    function test_submitIgnoredWhenInvalid() {
        var ctx = build()
        ctx.form.nameText = "   " // whitespace only
        ctx.form.typeIndex = 1
        ctx.form.submit()
        compare(ctx.mock.callCount, 0)
        teardown(ctx)
    }

    function test_disabledWhileCreateInProgress() {
        var ctx = build()
        ctx.form.nameText = "Everyday"
        ctx.form.typeIndex = 1
        verify(ctx.form.canSubmit) // enabled before submit
        ctx.form.submit() // mock latches createAccountInProgress
        compare(ctx.mock.createAccountInProgress, true)
        compare(ctx.form.canSubmit, false) // form disables while a create is in flight
        teardown(ctx)
    }

    function test_doubleSubmitSendsOnlyOne() {
        var ctx = build()
        ctx.form.nameText = "Everyday"
        ctx.form.typeIndex = 1
        ctx.form.submit()
        ctx.form.submit() // second is gated by canSubmit (in-progress)
        compare(ctx.mock.callCount, 1)
        teardown(ctx)
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
        teardown(ctx)
    }

    function test_failureShowsErrorMessage() {
        var ctx = build()
        ctx.form.nameText = "Everyday"
        ctx.form.typeIndex = 1
        ctx.form.submit()
        ctx.mock.fail("could not reach the daemon")
        compare(ctx.form.errorText, "could not reach the daemon")
        teardown(ctx)
    }
}
